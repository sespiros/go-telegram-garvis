package garvis

import (
	"fmt"
	"log"
	"strings"

	"cloud.google.com/go/datastore"
	"golang.org/x/net/context"
	glog "google.golang.org/appengine/log"

	"github.com/go-telegram-bot-api/telegram-bot-api"
)

type TextFilter struct {
	bot  *tgbotapi.BotAPI
	ctx  context.Context
	done chan bool
}

type UserRule struct {
	ChatID   int64
	UserID   int
	RuleText string
	Count    int
	Limit    int
}

type User struct {
	UserID   int
	UserName string
}

func (rule TextFilter) Check(update tgbotapi.Update) error {
	projectID := config.AppID
	ctx := rule.ctx

	// It is not possible to match usernames to user ids from the API
	// for the case of mentions so we need to store username-ids in the database
	// for lookup
	updateUsers(ctx, update)

	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatal(err)
	}

	query := datastore.NewQuery("UserRule").KeysOnly().
		Filter("ChatID = ", update.Message.Chat.ID).
		Filter("UserID = ", update.Message.From.ID)

	keys, err := client.GetAll(ctx, query, nil)
	if err != nil {
		glog.Errorf(ctx, "client.GetAll: %v", err)
	}

	for _, ruleKey := range keys {
		var userRule UserRule
		err = client.Get(ctx, ruleKey, &userRule)
		if err != nil {
			glog.Errorf(ctx, "client.Get: %v", err)
		}

		if i := strings.Index(update.Message.Text, userRule.RuleText); i != -1 {
			userRule.Count = userRule.Count + 1
			if userRule.Count >= userRule.Limit {
				userRule.Count = 0
				_, err = client.Put(ctx, ruleKey, &userRule)
				if err != nil {
					glog.Errorf(ctx, "client.Put(reset): %v", err)
				}
				rule.Trigger(update)
			}
			_, err = client.Put(ctx, ruleKey, &userRule)
			if err != nil {
				glog.Errorf(ctx, "client.Put: %v", err)
			}
		}
	}

	rule.done <- true
	return nil
}

func (rule TextFilter) GetCommands(commands map[string]Rule) {
	commands["adduserrule"] = rule
}

func (rule TextFilter) RunCommand(cmd string, cmdarg CommandArguments) {
	ctx := cmdarg.ctx
	update := cmdarg.update
	switch cmd {
	case "adduserrule":
		addUserRule(ctx, update)
	}
}

func (rule TextFilter) Trigger(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "TRIGGERED!!1!")
	rule.bot.Send(msg)
}

func addUserRule(ctx context.Context, update tgbotapi.Update) (err error) {
	command := strings.SplitN(update.Message.Text, " ", 4)
	count := []byte(command[1])[0] - '0'
	ruletext := command[3]

	var user int
	ents := update.Message.Entities
	for _, ent := range *ents {
		switch ent.Type {
		case "text_mention":
			user = ent.User.ID
		case "mention":
			if user, err = getUserID(ctx, command[2][1:]); err != nil {
				log.Fatal(err)
			}
		}
	}

	client, err := datastore.NewClient(ctx, config.AppID)
	if err != nil {
		log.Fatal(err)
	}

	kind := "UserRule"
	key := fmt.Sprintf("%v-%v-%v", update.Message.Chat.ID, user, ruletext)
	ruleKey := datastore.NameKey(kind, key, nil)
	userRule := UserRule{
		ChatID:   update.Message.Chat.ID,
		UserID:   user,
		RuleText: ruletext,
		Count:    0,
		Limit:    int(count),
	}

	_, err = client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var empty UserRule
		if err := tx.Get(ruleKey, &empty); err != datastore.ErrNoSuchEntity {
			return err
		}
		_, err := tx.Put(ruleKey, &userRule)
		return err
	})

	if err != nil {
		log.Fatal(err)
	}

	return nil
}

func getUserID(ctx context.Context, mention string) (int, error) {
	client, err := datastore.NewClient(ctx, config.AppID)
	if err != nil {
		log.Fatal(err)
	}

	var user User
	key := datastore.NameKey("User", mention, nil)
	err = client.Get(ctx, key, &user)
	if err != nil {
		glog.Errorf(ctx, "Error fetching user: %v", err)
		return 0, err
	}

	return user.UserID, nil
}

func updateUsers(ctx context.Context, update tgbotapi.Update) error {
	client, err := datastore.NewClient(ctx, config.AppID)
	if err != nil {
		log.Fatal(err)
	}

	kind := "User"
	userKey := datastore.NameKey(kind, update.Message.From.UserName, nil)
	user := User{
		UserID:   update.Message.From.ID,
		UserName: update.Message.From.UserName,
	}
	_, err = client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var empty User
		if err := tx.Get(userKey, &empty); err != datastore.ErrNoSuchEntity {
			return err
		}
		_, err := tx.Put(userKey, &user)
		return err
	})

	if err != nil {
		log.Fatal(err)
	}

	return nil
}

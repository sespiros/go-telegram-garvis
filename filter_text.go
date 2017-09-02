package garvis

import (
	"fmt"
	"log"
	"regexp"
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

type TextFilterRule struct {
	ChatID    int64
	UserID    int
	RxText    string
	TextReply string
	Count     int
	Limit     int
}

type User struct {
	UserID   int
	UserName string
}

func (filter TextFilter) Run(update tgbotapi.Update) error {
	projectID := config.AppID
	ctx := filter.ctx

	// It is not possible to match usernames to user ids from the API
	// for the case of mentions so we need to store username-ids in the database
	// for lookup
	updateUsers(ctx, update)

	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatal(err)
	}

	query := datastore.NewQuery("UserRule").KeysOnly().
		Filter("ChatID = ", update.Message.Chat.ID)
	queryUser := query.Filter("UserID = ", update.Message.From.ID)
	queryStatic := query.Filter("UserID = ", 0)

	userkeys, err := client.GetAll(ctx, queryUser, nil)
	statickeys, err := client.GetAll(ctx, queryStatic, nil)
	if err != nil {
		glog.Errorf(ctx, "client.GetAll: %v", err)
	}

	for _, ruleKey := range userkeys {
		var rule TextFilterRule
		err = client.Get(ctx, ruleKey, &rule)
		if err != nil {
			glog.Errorf(ctx, "client.Get: %v", err)
		}

		rxRule := regexp.MustCompile(rule.RxText)

		if rxRule.MatchString(update.Message.Text) {
			rule.Count = rule.Count + 1
			if rule.Count >= rule.Limit {
				rule.Count = 0
				_, err = client.Put(ctx, ruleKey, &rule)
				if err != nil {
					glog.Errorf(ctx, "client.Put(reset): %v", err)
				}
				filter.Trigger(update, rule)
			}
			_, err = client.Put(ctx, ruleKey, &rule)
			if err != nil {
				glog.Errorf(ctx, "client.Put: %v", err)
			}
		}
	}

	for _, ruleKey := range statickeys {
		var rule TextFilterRule
		err = client.Get(ctx, ruleKey, &rule)
		if err != nil {
			glog.Errorf(ctx, "client.Get: %v", err)
		}

		rxRule := regexp.MustCompile(rule.RxText)

		if rxRule.MatchString(update.Message.Text) {
			filter.Trigger(update, rule)
		}
	}

	filter.done <- true
	return nil
}

func (filter TextFilter) GetCommands(commands map[string]Filter) {
	commands["adduserrule"] = filter
	commands["addstaticrule"] = filter
}

func (filter TextFilter) RunCommand(cmd string, cmdarg CommandArguments) {
	ctx := cmdarg.ctx
	update := cmdarg.update
	var err error
	glog.Debugf(ctx, "%v", update.Message.Text)
	switch cmd {
	case "adduserrule":
		err = addUserRule(ctx, update)
	case "addstaticrule":
		err = addStaticRule(ctx, update)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func (filter TextFilter) Trigger(update tgbotapi.Update, rule TextFilterRule) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, rule.TextReply)
	filter.bot.Send(msg)
}

func addUserRule(ctx context.Context, update tgbotapi.Update) (err error) {
	command := strings.SplitN(update.Message.Text, " ", 2)
	if len(command) < 2 {
		return nil
	}
	argstr := command[1]
	args := strings.SplitN(argstr, "~", 4)
	if len(args) < 4 {
		return nil
	}
	text := args[0]
	textreply := args[1]
	count := []byte(args[3])[0] - '0'

	var user int
	ents := update.Message.Entities
	for _, ent := range *ents {
		switch ent.Type {
		case "text_mention":
			user = ent.User.ID
		case "mention":
			if user, err = getUserID(ctx, args[2][1:]); err != nil {
				return err
			}
		}
	}

	client, err := datastore.NewClient(ctx, config.AppID)
	if err != nil {
		return err
	}

	kind := "UserRule"
	key := fmt.Sprintf("%v-%v-%v", update.Message.Chat.ID, user, text)
	ruleKey := datastore.NameKey(kind, key, nil)
	rule := TextFilterRule{
		ChatID:    update.Message.Chat.ID,
		UserID:    user,
		RxText:    text,
		TextReply: textreply,
		Count:     0,
		Limit:     int(count),
	}

	_, err = client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var empty TextFilterRule
		if err := tx.Get(ruleKey, &empty); err != datastore.ErrNoSuchEntity {
			return err
		}
		_, err := tx.Put(ruleKey, &rule)
		return err
	})

	return err
}

func addStaticRule(ctx context.Context, update tgbotapi.Update) (err error) {
	command := strings.SplitN(update.Message.Text, " ", 2)
	if len(command) < 2 {
		return nil
	}
	argstr := command[1]
	args := strings.SplitN(argstr, "~", 2)
	if len(args) < 2 {
		return nil
	}
	text := args[0]
	textreply := args[1]

	client, err := datastore.NewClient(ctx, config.AppID)
	if err != nil {
		return err
	}

	kind := "UserRule"
	key := fmt.Sprintf("%v-%v", update.Message.Chat.ID, text)
	ruleKey := datastore.NameKey(kind, key, nil)
	userRule := TextFilterRule{
		ChatID:    update.Message.Chat.ID,
		RxText:    text,
		TextReply: textreply,
	}

	_, err = client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var empty TextFilterRule
		if err := tx.Get(ruleKey, &empty); err != datastore.ErrNoSuchEntity {
			return err
		}
		_, err := tx.Put(ruleKey, &userRule)
		return err
	})

	return err
}

func getUserID(ctx context.Context, mention string) (int, error) {
	client, err := datastore.NewClient(ctx, config.AppID)
	if err != nil {
		return -1, err
	}

	var user User
	key := datastore.NameKey("User", mention, nil)
	err = client.Get(ctx, key, &user)
	if err != nil {
		glog.Errorf(ctx, "Error fetching user: %v", err)
		return -1, err
	}

	return user.UserID, nil
}

func updateUsers(ctx context.Context, update tgbotapi.Update) error {
	client, err := datastore.NewClient(ctx, config.AppID)
	if err != nil {
		return err
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

	return err
}

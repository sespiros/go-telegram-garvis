package garvis

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/context"
	"google.golang.org/appengine/datastore"
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
	RxText    string
	TextReply string
	Count     int
	Limit     int
	UserID    int
}

type User struct {
	UserID   int
	UserName string
}

func (filter TextFilter) Run(update tgbotapi.Update) error {
	ctx := filter.ctx

	// It is not possible to match usernames to user ids from the API
	// for the case of mentions so we need to store username-ids in the database
	// for lookup
	updateUsers(ctx, update)

	query := datastore.NewQuery("Rule").Filter("ChatID = ", update.Message.Chat.ID)
	queryUserRules := query.Filter("UserID = ", update.Message.From.ID)
	queryStaticRules := query.Filter("UserID = ", 0)

	var userRules []TextFilterRule
	var staticRules []TextFilterRule
	userKeys, err := queryUserRules.GetAll(ctx, &userRules)
	staticKeys, err := queryStaticRules.GetAll(ctx, &staticRules)
	if err != nil {
		glog.Errorf(ctx, "client.GetAll: %v", err)
	}
	rules := append(userRules, staticRules...)
	keys := append(userKeys, staticKeys...)

	for i, rule := range rules {
		k := keys[i]

		rxRule := regexp.MustCompile(rule.RxText)

		if rxRule.MatchString(update.Message.Text) {
			rule.Count = rule.Count + 1
			if rule.Count >= rule.Limit {
				rule.Count = 0
				_, err = datastore.Put(ctx, k, &rule)
				if err != nil {
					glog.Errorf(ctx, "client.Put(reset): %v", err)
				}
				filter.Trigger(update, rule)
			}
			_, err = datastore.Put(ctx, k, &rule)
			if err != nil {
				glog.Errorf(ctx, "client.Put: %v", err)
			}
		}
	}

	filter.done <- true
	return nil
}

func (filter TextFilter) GetCommands(commands map[string]Filter) {
	commands["addrule"] = filter
}

func (filter TextFilter) RunCommand(cmd string, cmdarg CommandArguments) {
	ctx := cmdarg.ctx
	update := cmdarg.update
	var err error
	switch cmd {
	case "addrule":
		err = addRule(ctx, update)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func (filter TextFilter) Trigger(update tgbotapi.Update, rule TextFilterRule) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, rule.TextReply)
	filter.bot.Send(msg)
}

func addRule(ctx context.Context, update tgbotapi.Update) (err error) {
	command := strings.SplitN(update.Message.Text, " ", 2)
	if len(command) < 2 {
		return nil
	}
	argstr := command[1]
	args := strings.SplitN(argstr, "~", 3)
	var userID int
	switch len(args) {
	case 2:
		userID = 0
	case 3:
		ents := update.Message.Entities
		for _, ent := range *ents {
			switch ent.Type {
			case "text_mention":
				userID = ent.User.ID
			case "mention":
				if userID, err = getUserID(ctx, args[2][1:]); err != nil {
					return err
				}
			}
		}
	default:
		return nil
	}
	arg1 := strings.SplitN(args[0], "#", 2)
	text := fmt.Sprintf("(?i)%v(?-i)", arg1[0])
	var limit int
	if len(arg1) < 2 {
		limit = 1
	} else {
		limit, _ = strconv.Atoi(arg1[1])
	}
	textreply := args[1]

	key := fmt.Sprintf("%v-%v-%v", update.Message.Chat.ID, text, userID)
	ruleKey := datastore.NewKey(ctx, "Rule", key, 0, nil)
	rule := TextFilterRule{
		ChatID:    update.Message.Chat.ID,
		RxText:    text,
		TextReply: textreply,
		Count:     0,
		Limit:     int(limit),
		UserID:    userID,
	}

	err = datastore.RunInTransaction(ctx, func(ctx context.Context) error {
		var empty TextFilterRule
		if err := datastore.Get(ctx, ruleKey, &empty); err != datastore.ErrNoSuchEntity {
			return err
		}
		_, err := datastore.Put(ctx, ruleKey, &rule)
		return err
	}, nil)

	return err
}

func getUserID(ctx context.Context, mention string) (int, error) {
	var user User
	key := datastore.NewKey(ctx, "User", mention, 0, nil)
	err := datastore.Get(ctx, key, &user)
	if err != nil {
		glog.Errorf(ctx, "Error fetching user: %v", err)
		return -1, err
	}

	return user.UserID, nil
}

func updateUsers(ctx context.Context, update tgbotapi.Update) (err error) {
	userKey := datastore.NewKey(ctx, "User", update.Message.From.UserName, 0, nil)
	user := User{
		UserID:   update.Message.From.ID,
		UserName: update.Message.From.UserName,
	}
	err = datastore.RunInTransaction(ctx, func(ctx context.Context) error {
		var empty User
		if err := datastore.Get(ctx, userKey, &empty); err != datastore.ErrNoSuchEntity {
			return err
		}
		_, err := datastore.Put(ctx, userKey, &user)
		return err
	}, nil)

	return err
}

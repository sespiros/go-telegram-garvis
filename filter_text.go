package garvis

import (
	"bytes"
	"fmt"
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
	Hidden    bool
	CreatorID int
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
				filter.Trigger(rule)
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
	commands["deleterule"] = filter
	commands["listrules"] = filter
	commands["addhiddenrule"] = filter
}

func (filter TextFilter) RunCommand(cmd string, cmdarg CommandArguments) {
	update := cmdarg.update
	var err error
	switch cmd {
	case "addrule":
		err = filter.addRule(update, false)
	case "deleterule":
		err = filter.deleteRule(update)
	case "listrules":
		err = filter.listRules(update)
	case "addhiddenrule":
		err = filter.addRule(update, true)
	}
	if err != nil {
		glog.Errorf(filter.ctx, err.Error())
	}
}

func (filter TextFilter) Trigger(rule TextFilterRule) {
	msg := tgbotapi.NewMessage(rule.ChatID, rule.TextReply)
	filter.bot.Send(msg)
}

func (filter TextFilter) addRule(update tgbotapi.Update, hidden bool) (err error) {
	ctx := filter.ctx

	command := strings.SplitN(update.Message.Text, " ", 2)
	if len(command) < 2 {
		usage := "Usage: /addrule [regex matcher case insensitive by default][#count (optional default: 1)]~[reply]~[user (optional)]"
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, usage)
		filter.bot.Send(msg)
		return nil
	}
	argstr := command[1]
	args := strings.SplitN(argstr, "~", 3)
	var userID int
	if len(args) < 2 {
		return nil
	}

	if args[1] == "" {
		return nil
	}

	if len(args) >= 3 {
		if args[2] == "" {
			userID = 0
		} else {
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
		}
	}

	arg1 := strings.SplitN(args[0], "#", 2)
	text := fmt.Sprintf("(?i)%v", arg1[0])
	_, err = regexp.Compile(text)
	if err != nil {
		usage := "Invalid Regex"
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, usage)
		filter.bot.Send(msg)
		return nil
	}

	var limit int
	if len(arg1) < 2 {
		limit = 1
	} else {
		limit, err = strconv.Atoi(arg1[1])
		if err != nil {
			return nil
		}
	}

	textreply := args[1]

	keyl, _, _ := datastore.AllocateIDs(ctx, "Rule", nil, 1)
	ruleKey := datastore.NewKey(ctx, "Rule", "", keyl, nil)
	rule := TextFilterRule{
		ChatID:    update.Message.Chat.ID,
		RxText:    text,
		TextReply: textreply,
		Count:     0,
		Limit:     int(limit),
		UserID:    userID,
		CreatorID: update.Message.From.ID,
		Hidden:    hidden,
	}

	_, err = datastore.Put(ctx, ruleKey, &rule)

	return err
}

func (filter TextFilter) deleteRule(update tgbotapi.Update) (err error) {
	ctx := filter.ctx

	command := strings.SplitN(update.Message.Text, " ", 2)
	if len(command) < 2 {
		usage := "Usage: /deleterule [ruleID]"
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, usage)
		filter.bot.Send(msg)
		return nil
	}
	key, _ := strconv.ParseInt(command[1], 10, 64)
	ruleKey := datastore.NewKey(ctx, "Rule", "", key, nil)

	userID := update.Message.From.ID

	err = datastore.RunInTransaction(ctx, func(ctx context.Context) error {
		var rule TextFilterRule
		err := datastore.Get(ctx, ruleKey, &rule)
		if err != nil {
			return err
		}

		if rule.ChatID == update.Message.Chat.ID || rule.CreatorID == userID {
			err = datastore.Delete(ctx, ruleKey)
		}

		return err
	}, nil)

	return err
}

func (filter TextFilter) listRules(update tgbotapi.Update) (err error) {
	ctx := filter.ctx

	var buffer bytes.Buffer
	var query *datastore.Query
	var header string

	if update.Message.Chat.Type == "private" {
		userID := update.Message.From.ID
		query = datastore.NewQuery("Rule").Filter("CreatorID = ", userID)
		header = fmt.Sprintf("|%s|%s%s%s%s|%s|\n", "ID", "Regex", "(#Count)", "~Reply", "(~For User)", "Chat name")
	} else {
		query = datastore.NewQuery("Rule").Filter("ChatID = ", update.Message.Chat.ID)
		header = fmt.Sprintf("|%s|%s%s%s%s|\n", "ID", "Regex", "(#Count)", "~Reply", "(~For User)")
	}
	buffer.WriteString("All the Regexes are prepended with (?i) to be case insensitive\n")
	buffer.WriteString(header)
	buffer.String()

	for t := query.Run(ctx); ; {
		var rule TextFilterRule
		var userName string
		var userID int

		k, err := t.Next(&rule)
		if err == datastore.Done {
			break
		}

		if rule.UserID != 0 {
			userName, userID, _ = getUserName(ctx, rule.UserID)
		}

		if update.Message.Chat.Type == "private" || !rule.Hidden {
			// strip (?i) rule that makes the regexes case-sensitive by default before showing
			buffer.WriteString(fmt.Sprintf("|%v|%v", k.IntID(), rule.RxText[4:]))

			if rule.Limit != 1 {
				buffer.WriteString(fmt.Sprintf("#%v", rule.Limit))
			}

			buffer.WriteString(fmt.Sprintf("~%v", rule.TextReply))

			if rule.UserID != 0 {
				if userName != "" {
					buffer.WriteString(fmt.Sprintf("~@%v", userName))
				} else {
					buffer.WriteString(fmt.Sprintf("~{%v}", userID))
				}
			}

			if update.Message.Chat.Type == "private" {
				chat, _ := filter.bot.GetChat(tgbotapi.ChatConfig{ChatID: rule.ChatID})
				buffer.WriteString(fmt.Sprintf("|%v", chat.Title))
			}

			buffer.WriteString(fmt.Sprintf("|\n"))
		}
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, buffer.String())
	filter.bot.Send(msg)

	return nil
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

/* Return User's ID if user doesn't have a username and therefore doesn't exist in the database */
func getUserName(ctx context.Context, userID int) (string, int, error) {
	var users []User
	query := datastore.NewQuery("User").Filter("UserID = ", userID)

	query.GetAll(ctx, &users)

	if len(users) == 0 {
		return "", userID, nil
	}

	return users[0].UserName, 0, nil
}

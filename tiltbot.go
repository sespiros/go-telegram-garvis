package tiltbot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	glog "google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

var config Configuration

func loadConfig(path string) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal("Config file missing.", err)
	}

	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Fatal("Config parse error: ", err)
	}
}

func init() {
	loadConfig("./config.json")
	http.HandleFunc("/"+config.TgBotKey+"/start", startBot)
	http.HandleFunc("/"+config.TgBotKey+"/stop", stopBot)
	http.HandleFunc("/"+config.TgBotKey, handler)
}

func startBot(w http.ResponseWriter, r *http.Request) {
	url := "https://" + r.URL.Host + "/" + config.TgBotKey
	setWebhook(url, r)
	w.Write([]byte("The bot has been initialized."))
}

func stopBot(w http.ResponseWriter, r *http.Request) {
	setWebhook("", r)
	w.Write([]byte("The bot has been disabled."))
}

func getBot(c context.Context, r *http.Request) (*tgbotapi.BotAPI, error) {
	client := urlfetch.Client(c)
	bot, err := tgbotapi.NewBotAPIWithClient(config.TgBotKey, client)
	if err != nil {
		return nil, err
	}
	return bot, nil
}

func setWebhook(link string, r *http.Request) {
	c := appengine.NewContext(r)
	bot, err := getBot(c, r)
	if err != nil {
		log.Panic(err)
	}
	_, err = bot.SetWebhook(tgbotapi.NewWebhook(link))
	if err != nil {
		log.Fatal(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	bot, err := getBot(ctx, r)
	if err != nil {
		log.Fatal(err)
	}

	defer r.Body.Close()

	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}

	var update tgbotapi.Update
	json.Unmarshal(bytes, &update)

	if update.Message == nil {
		return
	}

	err = updateUsers(update, ctx)
	if err != nil {
		log.Fatal(err)
	}

	err = tiltBot(bot, update, ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func updateUsers(update tgbotapi.Update, ctx context.Context) error {
	projectID := "go-tiltbot"

	client, err := datastore.NewClient(ctx, projectID)
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
		// We first check that there is no entity stored with the given key.
		var empty User
		if err := tx.Get(userKey, &empty); err != datastore.ErrNoSuchEntity {
			return err
		}
		// If there was no matching entity, store it now.
		_, err := tx.Put(userKey, &user)
		return err
	})

	return nil
}

func tiltBot(bot *tgbotapi.BotAPI, update tgbotapi.Update, ctx context.Context) (err error) {
	if isCommandForMe(bot, update.Message) {
		switch update.Message.Command() {
		case "adduserrule":
			err = addUserRule(update, ctx)
		}
	} else {
		err = checkRules(bot, update, ctx)
	}

	if err != nil {
		return err
	}

	return nil
}

func isCommandForMe(bot *tgbotapi.BotAPI, m *tgbotapi.Message) bool {
	if m.IsCommand() {
		command := strings.SplitN(m.Text, " ", 2)[0][1:]

		if i := strings.Index(command, "@"); i != -1 {
			botName := command[i+1:]
			if bot.Self.UserName != botName {
				return false
			}
		}

		return true
	}

	return false
}

func addUserRule(update tgbotapi.Update, ctx context.Context) error {
	projectID := "go-tiltbot"

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
			user = getUserID(command[2][1:], ctx)
		}
	}

	client, err := datastore.NewClient(ctx, projectID)
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
		// We first check that there is no entity stored with the given key.
		var empty UserRule
		if err := tx.Get(ruleKey, &empty); err != datastore.ErrNoSuchEntity {
			return err
		}
		// If there was no matching entity, store it now.
		_, err := tx.Put(ruleKey, &userRule)
		return err
	})

	return nil
}

func getUserID(mention string, ctx context.Context) int {
	projectID := "go-tiltbot"

	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatal(err)
	}

	var user User
	key := datastore.NameKey("User", mention, nil)
	err = client.Get(ctx, key, &user)
	if err != nil {
		glog.Errorf(ctx, "Error fetching user: %v", err)
	}

	return user.UserID
}

func checkRules(bot *tgbotapi.BotAPI, update tgbotapi.Update, ctx context.Context) error {
	if checkCaps(update) {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "SHUT THE FUCK UP MATE")
		bot.Send(msg)
		return nil
	}
	if checkLength(update) {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "TL;DR")
		bot.Send(msg)
		return nil
	}
	if checkUserRules(update, ctx) {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "TRIGGERED!!1!")
		bot.Send(msg)
		return nil
	}
	return nil
}

func checkCaps(update tgbotapi.Update) bool {
	m := update.Message.Text

	if m == strings.ToUpper(m) && len(m) > 5 {
		return true
	}

	return false
}

func checkLength(update tgbotapi.Update) bool {

	if len(update.Message.Text) > 300 {
		return true
	}

	return false
}

func checkUserRules(update tgbotapi.Update, ctx context.Context) bool {
	projectID := "go-tiltbot"

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
		var rule UserRule
		err = client.Get(ctx, ruleKey, &rule)
		if err != nil {
			glog.Errorf(ctx, "client.Get: %v", err)
		}

		if i := strings.Index(update.Message.Text, rule.RuleText); i != -1 {
			rule.Count = rule.Count + 1
			if rule.Count >= rule.Limit {
				rule.Count = 0
				_, err = client.Put(ctx, ruleKey, &rule)
				if err != nil {
					glog.Errorf(ctx, "client.Put(reset): %v", err)
				}
				return true
			}
			_, err = client.Put(ctx, ruleKey, &rule)
			if err != nil {
				glog.Errorf(ctx, "client.Put: %v", err)
			}
		}
	}

	return false
}

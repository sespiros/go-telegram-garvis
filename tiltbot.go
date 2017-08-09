package tiltbot

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
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

	err = tiltBot(bot, update, ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func tiltBot(bot *tgbotapi.BotAPI, update tgbotapi.Update, ctx context.Context) (err error) {
	if isCommandForMe(bot, update.Message) {
		switch update.Message.Command() {
		case "addrule":
			err = addUserRule(update, ctx)
		}
	} else {
		err = checkRules(bot, update)
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

	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatal(err)
	}

	kind := "UserRule"
	ruleKey := datastore.IncompleteKey(kind, nil)
	rule := UserRule{
		ChatID:   update.Message.Chat.ID,
		UserID:   update.Message.From.ID,
		RuleText: "test",
		Count:    0,
		Limit:    1,
	}

	if _, err := client.Put(ctx, ruleKey, &rule); err != nil {
		log.Fatalf("Failed to save rule: %v", err)
	}

	return nil
}

func checkRules(bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
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
	if checkUserRules(update) {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Triggered")
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

func checkUserRules(update tgbotapi.Update) bool {
	return false
}

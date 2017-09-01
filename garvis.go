package garvis

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"golang.org/x/net/context"

	"github.com/go-telegram-bot-api/telegram-bot-api"
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
		log.Fatal(err)
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

	garvis(bot, ctx, update)
}

func garvis(bot *tgbotapi.BotAPI, ctx context.Context, update tgbotapi.Update) {
	var rules []Rule
	done := make(chan bool)
	rules = append(rules, Caps{bot, ctx, done})
	rules = append(rules, Tldr{bot, ctx, done})
	rules = append(rules, TextFilter{bot, ctx, done})

	commandRules := make(map[string]Rule)

	for _, r := range rules {
		r.GetCommands(commandRules)
	}

	if isCommandForMe(bot, update.Message) {
		cmd := update.Message.Command()
		if e, ok := commandRules[cmd]; ok {
			cmdarg := CommandArguments{ctx, update}
			e.RunCommand(cmd, cmdarg)
		}
	} else {
		for _, r := range rules {
			go r.Check(update)
		}
		for i := 0; i < len(rules); i++ {
			<-done
		}
	}
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

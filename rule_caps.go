package garvis

import (
	"regexp"
	"strings"

	"golang.org/x/net/context"

	"github.com/go-telegram-bot-api/telegram-bot-api"
)

type Caps struct {
	bot  *tgbotapi.BotAPI
	ctx  context.Context
	done chan bool
}

func (rule Caps) Check(update tgbotapi.Update) error {
	m := update.Message.Text

	if checkCaps(m) {
		rule.Trigger(update)
	}

	rule.done <- true
	return nil
}

func (rule Caps) GetCommands(map[string]Rule) {

}

func (rule Caps) RunCommand(string, CommandArguments) {

}

func (rule Caps) Trigger(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "SHUT THE FUCK UP MATE")
	rule.bot.Send(msg)
}

func checkCaps(m string) bool {
	var emojiRx = regexp.MustCompile(`[\x{1F600}-\x{1F6FF}|[\x{2600}-\x{26FF}]`)
	m = emojiRx.ReplaceAllString(m, ``)
	var symbolRx = regexp.MustCompile(`[^\pL\s]`)
	m = symbolRx.ReplaceAllString(m, ``)
	if m == strings.ToUpper(m) && len(m) > 5 {
		return true
	}

	return false
}

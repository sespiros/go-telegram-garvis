package garvis

import (
	"regexp"
	"strings"

	"golang.org/x/net/context"

	"github.com/go-telegram-bot-api/telegram-bot-api"
)

type CapsFilter struct {
	bot  *tgbotapi.BotAPI
	ctx  context.Context
	done chan bool
}

func (filter CapsFilter) Run(update tgbotapi.Update) error {
	m := update.Message.Text

	if checkCapsFilter(m) {
		filter.Trigger(update)
	}

	filter.done <- true
	return nil
}

func (filter CapsFilter) GetCommands(map[string]Filter) {

}

func (filter CapsFilter) RunCommand(string, CommandArguments) {

}

func (filter CapsFilter) Trigger(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "SHUT THE FUCK UP MATE")
	filter.bot.Send(msg)
}

func checkCapsFilter(m string) bool {
	var emojiRx = regexp.MustCompile(`[\x{1F600}-\x{1F6FF}|[\x{2600}-\x{26FF}]`)
	m = emojiRx.ReplaceAllString(m, ``)
	var symbolRx = regexp.MustCompile(`[^\pL\s]`)
	m = symbolRx.ReplaceAllString(m, ``)
	if m == strings.ToUpper(m) && len(m) > 5 {
		return true
	}

	return false
}

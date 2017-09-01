package garvis

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"golang.org/x/net/context"
)

type Tldr struct {
	bot  *tgbotapi.BotAPI
	ctx  context.Context
	done chan bool
}

func (rule Tldr) Check(update tgbotapi.Update) error {
	if len(update.Message.Text) > 300 {
		rule.Trigger(update)
	}

	rule.done <- true
	return nil
}

func (rule Tldr) GetCommands(map[string]Rule) {

}

func (rule Tldr) RunCommand(string, CommandArguments) {

}

func (rule Tldr) Trigger(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "TL;DR")
	rule.bot.Send(msg)
}

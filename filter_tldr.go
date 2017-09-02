package garvis

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"golang.org/x/net/context"
)

type TldrFilter struct {
	bot  *tgbotapi.BotAPI
	ctx  context.Context
	done chan bool
}

func (filter TldrFilter) Run(update tgbotapi.Update) error {
	if len(update.Message.Text) > 300 {
		filter.Trigger(update)
	}

	filter.done <- true
	return nil
}

func (filter TldrFilter) GetCommands(map[string]Filter) {

}

func (filter TldrFilter) RunCommand(string, CommandArguments) {

}

func (filter TldrFilter) Trigger(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "TL;DR")
	filter.bot.Send(msg)
}

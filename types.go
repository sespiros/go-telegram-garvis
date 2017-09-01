package garvis

import (
	"golang.org/x/net/context"

	"github.com/go-telegram-bot-api/telegram-bot-api"
)

type Configuration struct {
	TgBotKey string
	AppID    string
}

type Rule interface {
	Check(tgbotapi.Update) error
	GetCommands(map[string]Rule)
	RunCommand(string, CommandArguments)
	Trigger(tgbotapi.Update)
}

type CommandArguments struct {
	ctx    context.Context
	update tgbotapi.Update
}

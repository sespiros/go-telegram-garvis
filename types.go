package garvis

import (
	"golang.org/x/net/context"

	"github.com/go-telegram-bot-api/telegram-bot-api"
)

type Configuration struct {
	TgBotKey string
}

type Filter interface {
	Run(tgbotapi.Update) error
	GetCommands(map[string]Filter)
	RunCommand(string, CommandArguments)
}

type CommandArguments struct {
	ctx    context.Context
	update tgbotapi.Update
}

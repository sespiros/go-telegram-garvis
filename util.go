package tiltbot

type Configuration struct {
	TgBotKey string
}

type UserRule struct {
	ChatID   int64
	UserID   int
	RuleText string
	Count    int
	Limit    int
}

type User struct {
	UserID   int
	UserName string
}

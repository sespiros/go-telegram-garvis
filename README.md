# go-telegram-garvis

<img src="https://i.imgur.com/QOjdg7M.jpg" width=256>

Garvis is a rule-based AI bot for Telegram written in Go. It is built using [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api). It is written for Google App Engine but is easily ported to be hosted anywhere.

I created Garvis in the process of learning Go and trying to create a bot to mimic some of my reactions in Telegram group chats. For the time being it is following a fairly simple process by passing each message sent in a group through every filter available.

It can be used:
- as an autoresponder in group chats
- to create a personal butler for automating stuff with Telegram
- for gradually replacing yourself in group chats. (WIP: adding rules for all the modes of sarcasm, might use some sentiment analysis, might use some ML to make it learn from my responses hmmm... will see [:)](http://s.quickmeme.com/img/85/85b932b4dc1387653b77a77e6c3a7f0f18aff9dd27cb023f6eac2deec947f29c.jpg) )

## Features
- Generic filter system for easy creation of new filters and plugins
- Communication using Webhooks, making it easily scalable for environments like App Engine that spawn multiple instances for each request

## Rules added so far
- **Caps rule**: When a message is sent using all uppercase characters (stop shouting!)
- **TL;DR rule**: When a message exceeds a specified length
- **TextFilter**: This is a more complicated programmable rule that allows the creation of text filters for a specific user that trigger when the user says something for a number of times. This rule is using GAE key-based datastore to store the filters and their state.

## Installation
1. Fork the project
2. Create a new bot in Telegram using [BotFather](https://core.telegram.org/bots#6-botfather)
3. Disable group privacy mode so that it can read all the messages sent in a group
4. Create a new project in Google AppEngine
5. Create a config.json
```
{
    "TgBotKey": "YOUR KEY HERE",
    "AppID": "YOUR GAE APP ID HERE"
}
```
6. Create your own filters
7. Deploy it
8. Activate it by visiting https://{app-id}.appspot.com/{telegram-bot-key}/start
9. Add it to a group (maybe)
10. Leave the group (potentially)

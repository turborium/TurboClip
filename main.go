package main

import (
	"errors"
	"log"
	"os"

	toml "github.com/BurntSushi/toml"
	botapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"turboclip/db"
	"turboclip/lastlog"
	"turboclip/text"
)

const logFileName = "log.txt"
const configFileName = "config.toml"
const messagesFileName = "messages.toml"
const databaseFileName = "bot.db"

var Config struct {
	BotToken string
	OwnerID  int64
}

func loadConfig() {
	if _, err := os.Stat(configFileName); errors.Is(err, os.ErrNotExist) {
		log.Panicf("ERROR[config] Config file \"%s\" not found: %v", configFileName, err)
	}

	meta, err := toml.DecodeFile(configFileName, &Config)
	if err != nil {
		log.Panicf("ERROR[config] Bad config file \"%s\": %v", configFileName, err)
	}

	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		for _, key := range undecoded {
			log.Panicf("ERROR[config] Typo in config file \"%s\": Key \"%s\" does not exist", configFileName, key)
		}
	}
}

func main() {
	// log
	lastlog.BeginLogging(logFileName, 25)
	defer lastlog.EndLogging()
	log.Print("----------------------------------------")
	log.Print("[bot] init")

	// config
	log.Print("[bot] read config")
	loadConfig()

	// text
	log.Print("[bot] load messages")
	err := text.LoadFromFile(messagesFileName)
	if err != nil {
		log.Panicf("ERROR[bot] messages loading error: %v", err)
	}

	// db
	log.Print("[bot] open database")
	err = db.Open(databaseFileName)
	if err != nil {
		log.Panicf("ERROR[bot] open database error: %v", err)
	}
	defer db.Close()

	// init bot api
	log.Print("[bot] init bot api")
	api, err := botapi.NewBotAPI(Config.BotToken)
	if err != nil {
		log.Panicf("ERROR[bot] can't init bot api: %v", err)
	}
	api.Debug = false

	// check super user
	superUser, err := api.GetChatMember(
		botapi.GetChatMemberConfig{
			ChatConfigWithUser: botapi.ChatConfigWithUser{
				UserID: Config.OwnerID,
				ChatID: Config.OwnerID,
			},
		},
	)
	if err != nil {
		log.Panicf("ERROR[bot] super user not found ID:%d: %v", Config.OwnerID, err)
	}
	log.Printf("[bot] super user: [\"%v\", %v]", superUser.User.UserName, Config.OwnerID)

	// run
	bot := Bot{
		BotToken: Config.BotToken,
		OwnerID:  Config.OwnerID,
	}
	bot.Run()
}

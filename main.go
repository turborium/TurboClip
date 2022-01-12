package main

import (
	"errors"
	"log"
	"os"

	toml "github.com/BurntSushi/toml"

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
		log.Fatalf("ERROR[config] Config file \"%s\" not found: %v", configFileName, err)
	}

	meta, err := toml.DecodeFile(configFileName, &Config)
	if err != nil {
		log.Fatalf("ERROR[config] Bad config file \"%s\": %v", configFileName, err)
	}

	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		for _, key := range undecoded {
			log.Fatalf("ERROR[config] Typo in config file \"%s\": Key \"%s\" does not exist", configFileName, key)
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
		log.Fatalf("ERROR[bot] messages loading error: %v", err)
	}

	// db
	log.Print("[bot] open database")
	err = db.Open(databaseFileName)
	if err != nil {
		log.Fatalf("ERROR[bot] open database error: %v", err)
	}
	defer db.Close()

	// run
	bot := Bot{
		BotToken: Config.BotToken,
		OwnerID:  Config.OwnerID,
	}
	bot.Run()
}

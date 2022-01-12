package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"turboclip/db"
	"turboclip/lastlog"
	"turboclip/text"

	tb "gopkg.in/tucnak/telebot.v2"
)

type Bot struct {
	BotToken  string
	OwnerID   int64
	startTime time.Time
	telebot   *tb.Bot
}

const maxClipPerHour = 10

func (bot *Bot) pwnedCheck(message *tb.Message) {
	if message.Sender.ID != bot.OwnerID {
		m, err := json.MarshalIndent(message, "", "  ")
		if err != nil {
			log.Fatal("ERROR[bot] pwned!", err)
		}
		log.Fatalf("ERROR[bot] pwned!\n%v", string(m))
	}
}

func (bot *Bot) isSkipOldMessage(message *tb.Message) bool {
	if bot.startTime.After(time.Unix(message.Unixtime, 0)) {
		log.Printf("[bot] skip old message from [\"%v\", %v]: \"%v\"", message.Sender.Username, message.Sender.ID, message.Text)
		return true
	} else {
		return false
	}
}

func (bot *Bot) tryAddUser(message *tb.Message) {
	var isNew bool
	user, err := db.AddOrFindUser(message.Sender.ID, &isNew)
	if err != nil {
		bot.internalError(err)
	}
	if isNew {
		log.Printf("[bot] added new user [\"%v\", %v]", message.Sender.Username, message.Sender.ID)
	}
	if user.Name != message.Sender.Username {
		err = db.ApplyName(user.ID, message.Sender.Username)
		if err != nil {
			bot.internalError(err)
		}
	}
}

func (bot *Bot) onlyForOwner(handler func(message *tb.Message)) func(*tb.Message) {
	return func(message *tb.Message) {
		if message.Sender.ID != bot.OwnerID {
			bot.send(message.Sender, text.Format("CommandNotFound"))
			return
		}
		handler(message)
	}
}

func (bot *Bot) handle(endpoint interface{}, rawHandler interface{}) {
	if handler, ok := rawHandler.(func(*tb.Message)); ok {
		bot.telebot.Handle(endpoint, func(message *tb.Message) {
			bot.tryAddUser(message)
			if bot.isSkipOldMessage(message) {
				return
			}
			handler(message)
		})
	} else {
		bot.telebot.Handle(endpoint, rawHandler)
	}
}

func (bot *Bot) send(to tb.Recipient, what interface{}, options ...interface{}) (*tb.Message, error) {
	const attempts = 10
	tryCount := 0
	sleep := time.Millisecond * 10
try:
	message, err := bot.telebot.Send(to, what, options...)

	if err != nil {
		// again?
		if tryCount < attempts {
			tryCount++
		} else {
			log.Printf("WARNING[bot] can't send \"%v\" to \"%v\"", what, to.Recipient())
			return nil, err
		}
		// try fix bad markdown
		// ага, строки сравниваем, я хз почему куча либ для поступа к api не сохраняет в error код ошибки
		if strings.Contains(err.Error(), "(400)") {
			for i := range options {
				if opt, ok := options[i].(*tb.SendOptions); ok && opt.ParseMode != tb.ModeDefault {
					log.Printf("WARNING[bot] try fix bad markdown \"%v\"", what)
					opt.ParseMode = tb.ModeDefault
					break
				}
			}
		}
		// wait
		time.Sleep(sleep)
		sleep = sleep * 3
		// try again
		goto try
	}

	return message, nil
}

func (bot *Bot) internalError(err error) {
	log.Fatalf("ERROR[bot] internal error: %v", err)
}

// /log - owner only
func (bot *Bot) onLog(message *tb.Message) {
	bot.pwnedCheck(message)
	bot.send(message.Sender, "```\n"+lastlog.LastText()+"\n```", &tb.SendOptions{ParseMode: tb.ModeMarkdown})
}

// /logfile - owner only
func (bot *Bot) onLogfile(message *tb.Message) {
	log.Printf("[bot] /logfile ")
	bot.pwnedCheck(message)

	file := tb.Document{File: tb.FromDisk(logFileName), FileName: "log.txt"}

	bot.send(message.Sender, &file)
}

// /logfile - owner only
func (bot *Bot) onDbfile(message *tb.Message) {
	log.Printf("[bot] /dbfile ")
	bot.pwnedCheck(message)

	file := tb.Document{File: tb.FromDisk(databaseFileName), FileName: databaseFileName}

	bot.send(message.Sender, &file)
}

// /list - owner only
func (bot *Bot) onList(message *tb.Message) {
	log.Printf("[bot] /list for user [\"%v\", %v]", message.Sender.Username, message.Sender.ID)
	bot.pwnedCheck(message)

	mounths, err := db.GetMounts(message.Time().Location())
	if err != nil {
		bot.internalError(err)
	}

	if len(mounths) == 0 {
		bot.send(message.Sender, text.Format("Nothing"), &tb.SendOptions{ParseMode: tb.ModeMarkdown})
		return
	}

	selector := &tb.ReplyMarkup{}
	buttons := []tb.InlineButton{}
	rows := [][]tb.InlineButton{}
	for _, mounth := range mounths {
		if len(buttons) >= 6 {
			rows = append(rows, buttons)
			buttons = []tb.InlineButton{}
		}
		buttons = append(buttons,
			tb.InlineButton{
				Text: strconv.Itoa(mounth.Mount) + "." + strconv.Itoa(mounth.Year%100),
				Data: strconv.Itoa(mounth.Mount) + "." + strconv.Itoa(mounth.Year),
			})
	}
	for len(buttons) < 6 {
		buttons = append(buttons, tb.InlineButton{Text: "...", Data: "NOP"})
	}
	rows = append(rows, buttons)
	selector.InlineKeyboard = rows

	bot.send(message.Sender, text.Format("ChooseMounth"), &tb.SendOptions{ParseMode: tb.ModeMarkdown, ReplyMarkup: selector})
}

// callback - owner only
func (bot *Bot) onCallback(callback *tb.Callback) {
	log.Printf("[bot] callback for user [\"%v\", %v]: \"%v\"", callback.Sender.Username, callback.Sender.ID, callback.Data)

	bot.telebot.Respond(callback)

	// check
	if callback.Sender.ID != bot.OwnerID {
		bot.send(callback.Sender, text.Format("CommandNotFound"))
		return
	}

	if callback.Data == "NOP" {
		return
	}

	// parse
	dataPair := strings.Split(callback.Data, ".")
	if len(dataPair) != 2 {
		bot.internalError(fmt.Errorf("ERROR[bot] callback error"))
	}
	month, err := strconv.Atoi(dataPair[0])
	if err != nil {
		bot.internalError(err)
	}
	year, err := strconv.Atoi(dataPair[1])
	if err != nil {
		bot.internalError(err)
	}

	// work
	loc := callback.Message.Time().Location()
	highlights, err := db.GetHighlights(year, month, loc)
	if err != nil {
		bot.internalError(err)
	}

	if len(highlights) == 0 {
		bot.send(callback.Sender, text.Format("Nothing"), &tb.SendOptions{ParseMode: tb.ModeMarkdown})
		return
	}

	text := fmt.Sprintf("*====== %02d.%d ======*\n", month, year%100)
	day := -1
	for _, item := range highlights {
		if day != item.Time.In(loc).Day() {
			day = item.Time.In(loc).Day()
			text = text + item.Time.In(loc).Format("*\n[ 02.01.06 ]*\n")
		}

		user, err := db.AddOrFindUser(item.UserID, nil)
		if err != nil {
			bot.internalError(err)
		}

		text = text + item.Time.In(loc).Format("*15:04 - \"") + user.Name + "\"*\n" + item.Text + "\n"
	}

	bot.send(callback.Sender, text, &tb.SendOptions{ParseMode: tb.ModeMarkdown})
}

// /start
func (bot *Bot) onStart(message *tb.Message) {
	log.Printf("[bot] /start for user [\"%v\", %v]", message.Sender.Username, message.Sender.ID)

	bot.send(message.Sender, text.Format("Start", message.Sender.FirstName), &tb.SendOptions{ParseMode: tb.ModeMarkdown})
}

// text
func (bot *Bot) onText(message *tb.Message) {
	log.Printf("[bot] text for user [\"%v\", %v]", message.Sender.Username, message.Sender.ID)

	if strings.HasPrefix(message.Text, "/") {
		bot.telebot.Send(message.Sender, text.Format("CommandNotFound"))
		return
	}

	// spam test
	count, err := db.CountForDuration(message.Sender.ID, time.Hour)
	if err != nil {
		bot.internalError(err)
	}
	if count >= maxClipPerHour {
		log.Printf("[bot] spam from [\"%v\", %v]: \"%v\"", message.Sender.Username, message.Sender.ID, message.Text)
		bot.send(message.Sender, text.Format("Spam"))
		return
	}

	log.Printf("[bot] new highlight from [\"%v\", %v]: \"%v\"", message.Sender.Username, message.Sender.ID, message.Text)

	_, err = db.NewHighlight(message.Sender.ID, message.Text)
	if err != nil {
		bot.internalError(err)
	}

	bot.send(message.Sender, text.Format("Thanks", message.Sender.FirstName))

	// notify
	bot.send(&tb.User{ID: bot.OwnerID}, text.Format("NewFrom", message.Sender.Username, message.Sender.ID, message.Text))
}

// /stat
func (bot *Bot) onStat(message *tb.Message) {
	log.Printf("[bot] /stat for user [\"%v\", %v]", message.Sender.Username, message.Sender.ID)

	stat, err := db.GetStat(message.Sender.ID)
	if err != nil {
		bot.internalError(err)
	}

	d := time.Now().UTC().Sub(bot.startTime)
	bot.send(message.Sender, text.Format("Stat", stat.Count, stat.UserCount, d.String()), &tb.SendOptions{ParseMode: tb.ModeMarkdown})
}

func (bot *Bot) Run() {
	/*
		rand.Seed(time.Now().Unix())
		s := []string{"Реакция на ролик про аниме для линуксоидов", "Просмотр кода Winderton", "Программирование визуального эффекта"}
		for i := 0; i < 200; i++ {
			t := time.Date(2020+rand.Intn(2), time.Month(1+rand.Intn(12)), 1+rand.Intn(28), rand.Intn(24), rand.Intn(60), 0, 0, time.Local)
			db.NewHighlight_test(bot.OwnerID, s[rand.Intn(len(s))], t)
		}
	*/

	log.Print("[bot] init")

	bot.startTime = time.Now().UTC()

	var err error
	bot.telebot, err = tb.NewBot(
		tb.Settings{
			Token:       bot.BotToken,
			Poller:      &tb.LongPoller{Timeout: 10 * time.Second, LastUpdateID: -2},
			Synchronous: false,
			Verbose:     false,
		})
	if err != nil {
		log.Fatal("ERROR[bot] can't init telebot: ", err)
		return
	}

	// check super user
	superUser, err := bot.telebot.ChatMemberOf(&tb.Chat{ID: Config.OwnerID}, &tb.User{ID: Config.OwnerID})
	if err != nil {
		log.Fatalf("ERROR[bot] super user not found ID:%d: %v", Config.OwnerID, err)
	}
	log.Printf("[bot] super user: [\"%v\", %v]", superUser.User.Username, Config.OwnerID)

	// for owner
	bot.handle("/log", bot.onlyForOwner(bot.onLog))
	bot.handle("/logfile", bot.onlyForOwner(bot.onLogfile))
	bot.handle("/dbfile", bot.onlyForOwner(bot.onDbfile))
	bot.handle("/list", bot.onlyForOwner(bot.onList))
	bot.handle(tb.OnCallback, bot.onCallback)

	// for all
	bot.handle("/start", bot.onStart)
	bot.handle("/stat", bot.onStat)
	bot.handle(tb.OnText, bot.onText)

	log.Print("[bot] start")
	bot.telebot.Start()
}

package db

import (
	"errors"
	"time"

	"github.com/asdine/storm/q"
	"github.com/asdine/storm/v3"
)

type User struct {
	ID      int64 `storm:"id"`
	Name    string
	RegTime time.Time
}

type Highlight struct {
	ID     int64     `storm:"id,increment"`
	UserID int64     `storm:"index"`
	Time   time.Time `storm:"index"`
	Text   string
}

var (
	db *storm.DB
)

const (
	usersBucket     = "Users"
	highlightBucket = "Highlights"
)

func Open(fileName string) error {
	if db != nil {
		return errors.New("the database is already open")
	}

	var err error
	db, err = storm.Open(fileName) //, 0666, nil)
	if err != nil {
		return err
	}

	// Users
	users := db.From(usersBucket)
	err = users.Init(&User{})
	if err != nil {
		return err
	}

	// Highlights
	highlights := db.From(highlightBucket)
	err = highlights.Init(&Highlight{})
	if err != nil {
		return err
	}

	return nil
}

func Close() error {
	if db == nil {
		return errors.New("no database currently open")
	}

	return db.Close()
}

/*func NewUser(id int64) (*User, error) {
	user := &User{
		ID:      id,
		RegTime: time.Now().UTC(),
	}

	users := db.From(usersBucket)
	err := users.Save(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func FindUser(id int64) *User {
	users := db.From(usersBucket)

	var user User
	if users.One("ID", id, &user) != nil {
		return nil
	}
	return &user
}*/

func AddOrFindUser(id int64, isNew *bool) (*User, error) {
	users := db.From(usersBucket)

	tx, err := users.Begin(true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var user User
	err = tx.One("ID", id, &user)

	if isNew != nil {
		*isNew = err != nil
	}

	if err != nil {
		user = User{
			ID:      id,
			RegTime: time.Now().UTC(),
		}

		err := tx.Save(&user)
		if err != nil {
			return nil, err
		}
	}

	return &user, tx.Commit()
}

func ApplyName(id int64, name string) error {
	users := db.From(usersBucket)

	var user User
	err := users.One("ID", id, &user)
	if err != nil {
		return err
	}

	user.Name = name

	return users.Save(&user)
}

func NewHighlight(userID int64, text string) (*Highlight, error) {
	highlight := &Highlight{
		UserID: userID,
		Time:   time.Now().UTC(),
		Text:   text,
	}

	highlights := db.From(highlightBucket)
	err := highlights.Save(highlight)
	if err != nil {
		return nil, err
	}

	return highlight, nil
}

func NewHighlight_test(userID int64, text string, time time.Time) (*Highlight, error) {
	highlight := &Highlight{
		UserID: userID,
		Time:   time,
		Text:   text,
	}

	highlights := db.From(highlightBucket)
	err := highlights.Save(highlight)
	if err != nil {
		return nil, err
	}

	return highlight, nil
}

type Stat struct {
	UserCount int64
	Count     int64
}

func GetStat(userID int64) (*Stat, error) {
	highlights := db.From(highlightBucket)

	count, err := highlights.Count(&Highlight{})
	if err != nil {
		return nil, err
	}

	userCount, err := highlights.Select(q.Eq("UserID", userID)).Count(&Highlight{})
	if err != nil {
		return nil, err
	}

	return &Stat{
		Count:     int64(count),
		UserCount: int64(userCount),
	}, nil
}

func CountForDuration(userID int64, duration time.Duration) (int, error) {
	highlights := db.From(highlightBucket)

	t := time.Now().UTC().Add(-duration)

	count, err := highlights.Select(q.Eq("UserID", userID), q.Gte("Time", t)).Count(&Highlight{})

	return count, err
}

type Month struct {
	Year  int
	Mount int
}

// бред
func GetMounts(loc *time.Location) ([]Month, error) {
	mounts := []Month{}

	// first clip
	highlights := db.From(highlightBucket)
	first := Highlight{}
	if highlights.Select().OrderBy("Time").First(&first) != nil {
		return mounts, nil
	}

	beginTime := first.Time.In(loc) //time.Date(2020, time.Month(1), 1, 0, 0, 0, 0, loc)
	endTime := time.Now().In(loc)

	m := Month{
		Year:  beginTime.Year(),
		Mount: int(beginTime.Month()),
	}

	for m.Year < endTime.Year() || m.Mount <= int(endTime.Month()) {
		mounts = append(mounts, m)
		if m.Mount < 12 {
			m.Mount++
		} else {
			m.Mount = 1
			m.Year++
		}
	}

	return mounts, nil
}

func GetHighlights(year int, mounth int, loc *time.Location) ([]Highlight, error) {
	backet := db.From(highlightBucket)

	beginTime := time.Date(year, time.Month(mounth), 1, 0, 0, 0, 0, loc).UTC()
	nextMounth := mounth + 1
	nextYear := year
	if nextMounth == 13 {
		nextMounth = 1
		nextYear++
	}
	endTime := time.Date(nextYear, time.Month(nextMounth), 1, 0, 0, 0, 0, loc).UTC()

	highlight := []Highlight{}
	backet.Select(q.Gte("Time", beginTime), q.Lt("Time", endTime)).OrderBy("Time").Find(&highlight) // todo: check error

	return highlight, nil
}

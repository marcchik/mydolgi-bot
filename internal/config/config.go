package config

import (
	"log"
	"os"
	"strings"
)

type Config struct {
	BotToken        string
	DatabaseURL     string
	Timezone        string
	RemindDaysBefore []int // e.g. [7,1,0]
}

func MustLoad() Config {
	bt := os.Getenv("BOT_TOKEN")
	if bt == "" {
		log.Fatal("BOT_TOKEN is required")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	tz := os.Getenv("TZ")
	if tz == "" {
		tz = "Europe/London"
	}

	rdb := os.Getenv("REMIND_DAYS_BEFORE")
	if rdb == "" {
		rdb = "7,1,0"
	}
	var days []int
	for _, p := range strings.Split(rdb, ",") {
		p = strings.TrimSpace(p)
		switch p {
		case "7":
			days = append(days, 7)
		case "1":
			days = append(days, 1)
		case "0":
			days = append(days, 0)
		default:
			// ignore unknown; keep it strict if хочешь
		}
	}
	if len(days) == 0 {
		days = []int{7, 1, 0}
	}

	return Config{
		BotToken: bt,
		DatabaseURL: dsn,
		Timezone: tz,
		RemindDaysBefore: days,
	}
}

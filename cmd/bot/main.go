package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/yourname/dolgo-bot/internal/bot"
	"github.com/yourname/dolgo-bot/internal/config"
	"github.com/yourname/dolgo-bot/internal/db"
	"github.com/yourname/dolgo-bot/internal/repo"
)

func main() {
	cfg := config.MustLoad()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := db.MustConnect(ctx, cfg.DatabaseURL)
	defer pool.Close()

	botAPI, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Fatalf("bot init: %v", err)
	}
	botAPI.Debug = false

	rUsers := repo.NewUsers(pool)
	rContacts := repo.NewContacts(pool)
	rDebts := repo.NewDebts(pool)

	h := bot.NewHandler(botAPI, cfg, rUsers, rContacts, rDebts)

	// Graceful shutdown
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()

	// Run migrations automatically on start (simple approach)
	if err := db.ApplyMigrations(ctx, pool, "./migrations"); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	// Reminders worker
	go h.RunReminderWorker(ctx, 30*time.Second)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := botAPI.GetUpdatesChan(u)

	log.Printf("DolgoBot started as @%s", botAPI.Self.UserName)

	for {
		select {
		case <-ctx.Done():
			log.Println("shutdown")
			return
		case upd := <-updates:
			h.HandleUpdate(ctx, upd)
		}
	}
}

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"taxibot/config"
	"taxibot/pkg/bot"
	"taxibot/pkg/logger"
	"taxibot/storage/postgres"
)

func main() {
	// 1. Load Config
	cfg := config.Load()

	// 2. Initialize Logger
	log := logger.New(cfg.ServiceName)

	// 3. Initialize Shared Storage (Postgres)
	// postgres.New expects config.Config value
	pgStore, err := postgres.New(context.Background(), cfg, log)
	if err != nil {
		log.Error("Failed to connect to postgres", logger.Error(err))
		os.Exit(1)
	}
	defer pgStore.Close()

	log.Info("🚀 Dual Bot Backend is initializing...")

	// 4. Initialize Client Bot (Bot 1)
	clientBot, err := bot.New(bot.BotTypeClient, &cfg, pgStore, log)
	if err != nil {
		log.Error("Failed to initialize client bot", logger.Error(err))
		os.Exit(1)
	}

	// 5. Initialize Driver/Admin Bot (Bot 2)
	driverAdminBot, err := bot.New(bot.BotTypeDriverAdmin, &cfg, pgStore, log)
	if err != nil {
		log.Error("Failed to initialize driver/admin bot", logger.Error(err))
		os.Exit(1)
	}

	// 🛠 PEER LINKING: Botlarni bir-biriga bog'laymiz (Notifikatsiyalar uchun)
	clientBot.Peer = driverAdminBot
	driverAdminBot.Peer = clientBot

	// 6. Run bots in parallel goroutines
	go func() {
		log.Info("Bot 1 (Client) is starting...")
		clientBot.Start()
	}()

	go func() {
		log.Info("Bot 2 (Driver/Admin) is starting...")
		driverAdminBot.Start()
	}()

	log.Info("🚀 Both bots are now running successfully.")

	// 7. Graceful Shutdown listener
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	log.Info("Stopping bots and shutting down...")
}

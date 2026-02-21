package main

import (
	"context"
	"fmt"
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

	// 5. Initialize Driver Bot (Bot 2)
	driverBot, err := bot.New(bot.BotTypeDriver, &cfg, pgStore, log)
	if err != nil {
		log.Error("Failed to initialize driver bot", logger.Error(err))
		os.Exit(1)
	}

	// 6. Initialize Admin Bot (Bot 3)
	adminBot, err := bot.New(bot.BotTypeAdmin, &cfg, pgStore, log)
	if err != nil {
		log.Error("Failed to initialize admin bot", logger.Error(err))
		os.Exit(1)
	}

	// 🛠 PEER LINKING: Botlarni bir-biriga bog'laymiz (Notifikatsiyalar uchun)
	// Client Peers
	clientBot.Peers[bot.BotTypeDriver] = driverBot
	clientBot.Peers[bot.BotTypeAdmin] = adminBot

	// Driver Peers
	driverBot.Peers[bot.BotTypeClient] = clientBot
	driverBot.Peers[bot.BotTypeAdmin] = adminBot

	// Admin Peers
	adminBot.Peers[bot.BotTypeClient] = clientBot
	adminBot.Peers[bot.BotTypeDriver] = driverBot

	// 7. Initialize Web Server (Mini App API & Static)
	go func() {
		log.Info(fmt.Sprintf("🚀 Web Server is starting on :%d...", cfg.AppPort))
		if err := bot.RunServer(&cfg, pgStore, log, clientBot.HandlePaymentSuccess); err != nil {
			log.Error("Failed to start web server", logger.Error(err))
		}
	}()

	// 8. Run bots in parallel goroutines
	go func() {
		log.Info("Bot 1 (Client) is starting...")
		clientBot.Start()
	}()

	go func() {
		log.Info("Bot 2 (Driver) is starting...")
		driverBot.Start()
	}()

	go func() {
		log.Info("Bot 3 (Admin) is starting...")
		adminBot.Start()
	}()

	log.Info("🚀 All 3 bots are now running successfully.")

	// 7. Graceful Shutdown listener
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	log.Info("Stopping bots and shutting down...")
}

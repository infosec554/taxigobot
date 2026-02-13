package main

import (
	"context"
	"fmt"
	"taxibot/config"
	"taxibot/pkg/logger"
	"taxibot/storage/postgres"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.ServiceName)
	pg, err := postgres.New(context.Background(), cfg, log)

	if err != nil {
		panic(err)
	}
	defer pg.Close()

	// Truncate users and dependent tables
	// We use CASCADE to clean up orders and user_routes that reference users.
	// We keep locations and tariffs as they are 'system' data.
	_, err = pg.GetPool().Exec(context.Background(), "TRUNCATE TABLE users, orders, driver_routes CASCADE")
	if err != nil {
		log.Error(fmt.Sprintf("Failed to truncate tables: %v", err))
	} else {
		log.Info("Successfully truncated users, orders, and user_routes tables.")
	}
}

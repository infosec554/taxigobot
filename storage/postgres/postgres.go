package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	"taxibot/config"
	"taxibot/pkg/logger"
	"taxibot/storage"
)

type Store struct {
	pool *pgxpool.Pool
	log  logger.ILogger
}

func New(ctx context.Context, cfg config.Config, log logger.ILogger) (storage.IStorage, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDB,
	)

	// 🔹 Connection pool
	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Error("error while parsing Postgres config", logger.Error(err))
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Error("failed to connect Postgres", logger.Error(err))
		return nil, err
	}

	// 🔹 Migration path
	cwd, _ := os.Getwd()
	mPath := filepath.Join(cwd, "migrations")

	// Check if migrations/postgres exists, if so use it, else use migrations
	if _, err := os.Stat(filepath.Join(cwd, "migrations", "postgres")); err == nil {
		mPath = filepath.Join(cwd, "migrations", "postgres")
	}

	m, err := migrate.New("file://"+mPath, url)
	if err != nil {
		log.Error("migration init error or no migrations found", logger.Error(err))
	} else {
		if err = m.Up(); err != nil {
			if strings.Contains(err.Error(), "no change") {
				log.Info("no migrations to apply")
			} else {
				log.Error("migration up error", logger.Error(err))
				return nil, err
			}
		}
	}

	log.Info("Postgres connected")

	return &Store{
		pool: pool,
		log:  log,
	}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) GetPool() *pgxpool.Pool {
	return s.pool
}

func (s *Store) User() storage.IUserStorage           { return NewUserRepo(s.pool, s.log) }
func (s *Store) Order() storage.IOrderStorage         { return NewOrderRepo(s.pool, s.log) }
func (s *Store) Tariff() storage.ITariffStorage       { return NewTariffRepo(s.pool, s.log) }
func (s *Store) Direction() storage.IDirectionStorage { return NewDirectionRepo(s.pool, s.log) }

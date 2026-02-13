package config

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cast"
)

type Config struct {
	ServiceName string
	LoggerLevel string

	AppPort int

	PostgresHost     string
	PostgresPort     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string

	RedisHost     string
	RedisPort     string
	RedisPassword string

	TelegramBotToken string
	DriverBotToken   string
	AdminBotToken    string
	AdminID          int64
	AdminUsername    string
}

func Load() Config {
	_ = godotenv.Load(".env")

	cfg := Config{}

	cfg.ServiceName = cast.ToString(getOrReturnDefault("SERVICE_NAME", "taxibot"))
	cfg.LoggerLevel = cast.ToString(getOrReturnDefault("LOGGER_LEVEL", "debug"))
	cfg.AppPort = cast.ToInt(getOrReturnDefault("APP_PORT", 8080))

	cfg.PostgresHost = cast.ToString(getOrReturnDefault("POSTGRES_HOST", "localhost"))
	cfg.PostgresPort = cast.ToString(getOrReturnDefault("POSTGRES_PORT", "5432"))
	cfg.PostgresUser = cast.ToString(getOrReturnDefault("POSTGRES_USER", "postgres"))
	cfg.PostgresPassword = cast.ToString(getOrReturnDefault("POSTGRES_PASSWORD", "1234"))
	cfg.PostgresDB = cast.ToString(getOrReturnDefault("POSTGRES_DB", "taxibot"))

	cfg.RedisHost = cast.ToString(getOrReturnDefault("REDIS_HOST", "localhost"))
	cfg.RedisPort = cast.ToString(getOrReturnDefault("REDIS_PORT", "6379"))
	cfg.RedisPassword = cast.ToString(getOrReturnDefault("REDIS_PASSWORD", ""))

	cfg.TelegramBotToken = cast.ToString(getOrReturnDefault("TG_BOT_TOKEN", ""))
	cfg.DriverBotToken = cast.ToString(getOrReturnDefault("DRIVER_BOT_TOKEN", ""))
	cfg.AdminBotToken = cast.ToString(getOrReturnDefault("ADMIN_BOT_TOKEN", ""))
	cfg.AdminID = cast.ToInt64(getOrReturnDefault("ADMIN_ID", 0))
	cfg.AdminUsername = cast.ToString(getOrReturnDefault("ADMIN_USERNAME", ""))

	return cfg
}

func getOrReturnDefault(key string, defaultValue interface{}) interface{} {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return defaultValue
}

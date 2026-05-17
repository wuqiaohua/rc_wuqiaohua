package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr           string
	MySQLDSN           string
	RabbitMQURL        string
	VendorProfilesPath string
	RunWorkers         bool
	OutboxInterval     time.Duration
	HTTPTimeout        time.Duration
	ConsumerPrefetch   int
}

func Load() Config {
	return Config{
		HTTPAddr:           env("HTTP_ADDR", ":8080"),
		MySQLDSN:           env("MYSQL_DSN", "root:password@tcp(127.0.0.1:3306)/notifications?parseTime=true&charset=utf8mb4&loc=UTC"),
		RabbitMQURL:        env("RABBITMQ_URL", "amqp://guest:guest@127.0.0.1:5672/"),
		VendorProfilesPath: env("VENDOR_PROFILES_PATH", "config/vendor_profiles.toml"),
		RunWorkers:         envBool("RUN_WORKERS", true),
		OutboxInterval:     envDuration("OUTBOX_INTERVAL", 2*time.Second),
		HTTPTimeout:        envDuration("DELIVERY_HTTP_TIMEOUT", 10*time.Second),
		ConsumerPrefetch:   envInt("CONSUMER_PREFETCH", 10),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	AppEnv                 string
	HTTPAddr               string
	LogLevel               string
	DatabaseURL            string
	DatabaseConnectTimeout time.Duration
	ShutdownTimeout        time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:                 envOrDefault("APP_ENV", "development"),
		HTTPAddr:               envOrDefault("HTTP_ADDR", ":8080"),
		LogLevel:               envOrDefault("LOG_LEVEL", "info"),
		DatabaseURL:            strings.TrimSpace(os.Getenv("DATABASE_URL")),
		DatabaseConnectTimeout: durationOrDefault("DATABASE_CONNECT_TIMEOUT", 5*time.Second),
		ShutdownTimeout:        durationOrDefault("SHUTDOWN_TIMEOUT", 10*time.Second),
	}

	if cfg.AppEnv == "production" && cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required when APP_ENV=production")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationOrDefault(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

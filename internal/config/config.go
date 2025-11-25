package config

import (
	"fmt"
	"os"
)

// Config stores runtime configuration for the service.
type Config struct {
	AppPort     string
	DatabaseURL string
	LogLevel    string
}

// Load reads configuration from environment variables with sane defaults.
func Load() (Config, error) {
	cfg := Config{
		AppPort:     getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

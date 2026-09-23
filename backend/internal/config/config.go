// Package config loads backend configuration from environment variables.
package config

import "os"

// Config holds runtime configuration for the backend server.
type Config struct {
	AppEnv      string
	Port        string
	DatabaseURL string
	LogLevel    string
}

// Load reads configuration from environment variables, applying sane
// defaults for local development.
func Load() Config {
	return Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Package config handles Primarch configuration from environment variables.
package config

import (
	"os"
	"strconv"
)

// Config holds the Primarch configuration.
type Config struct {
	Port     int
	LogLevel string
	Seed     bool   // Seed Fortress Primus on startup
	DBUrl    string // PostgreSQL connection string (empty = in-memory store)
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:     envInt("PRIMARCH_PORT", 8401),
		LogLevel: envStr("PRIMARCH_LOG_LEVEL", "info"),
		Seed:     envBool("PRIMARCH_SEED", true),
		DBUrl:    envStr("PRIMARCH_DB_URL", ""),
	}
}

// UseDB returns true if a database URL is configured.
func (c *Config) UseDB() bool {
	return c.DBUrl != ""
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

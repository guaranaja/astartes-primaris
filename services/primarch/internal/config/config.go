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

	// Engine protocol
	ServiceToken string // Bearer token for engine-to-Primarch auth

	// CFO Engine (Firefly III) integration
	CFOEngineURL   string // Base URL, e.g. https://cfo-engine-dev-xxx.run.app
	CFOEngineToken string // Firefly III personal access token

	// Monarch Money integration
	MonarchToken string // Monarch session token
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:           envInt("PRIMARCH_PORT", 8401),
		LogLevel:       envStr("PRIMARCH_LOG_LEVEL", "info"),
		Seed:           envBool("PRIMARCH_SEED", true),
		DBUrl:          envStr("PRIMARCH_DB_URL", ""),
		ServiceToken:   envStr("PRIMARCH_SERVICE_TOKEN", ""),
		CFOEngineURL:   envStr("CFO_ENGINE_URL", ""),
		CFOEngineToken: envStr("CFO_ENGINE_TOKEN", ""),
		MonarchToken:   envStr("MONARCH_TOKEN", ""),
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

package config

import (
	"os"
	"strconv"
)

var (
	DbUri = getEnvOrDefault("LONGHOUSE_DB_URI", "postgresql://longhouse:devpass123@localhost:5432/longhouse_db?sslmode=disable")

	APIPort = getEnvAsIntOrDefault("LONGHOUSE_API_PORT", 6080)
	TCPPort = getEnvAsIntOrDefault("LONGHOUSE_TCP_PORT", 6081)

	LinkkeysDomain = getEnvOrDefault("LONGHOUSE_LINKKEYS_DOMAIN", "")
	LinkkeysURL    = getEnvOrDefault("LONGHOUSE_LINKKEYS_URL", "")

	InitialAdminDomain = getEnvOrDefault("LONGHOUSE_INITIAL_ADMIN_DOMAIN", "")
	InitialAdminUserID = getEnvOrDefault("LONGHOUSE_INITIAL_ADMIN_USER_ID", "")
	InitialHouseName   = getEnvOrDefault("LONGHOUSE_INITIAL_HOUSE_NAME", "Longhouse")
)

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsIntOrDefault(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

// ApplyFlags overrides config values from CLI flags.
func ApplyFlags(flags map[string]string) {
	if v, ok := flags["db-uri"]; ok {
		DbUri = v
	}
	if v, ok := flags["api-port"]; ok {
		if i, err := strconv.Atoi(v); err == nil {
			APIPort = i
		}
	}
	if v, ok := flags["tcp-port"]; ok {
		if i, err := strconv.Atoi(v); err == nil {
			TCPPort = i
		}
	}
	if v, ok := flags["initial-admin-domain"]; ok {
		InitialAdminDomain = v
	}
	if v, ok := flags["initial-admin-user-id"]; ok {
		InitialAdminUserID = v
	}
	if v, ok := flags["initial-house-name"]; ok {
		InitialHouseName = v
	}
}

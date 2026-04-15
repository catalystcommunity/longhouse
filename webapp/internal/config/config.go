package config

import (
	"os"
	"strconv"
)

var (
	WebPort = getEnvAsIntOrDefault("LONGHOUSE_WEB_PORT", 4080)
	APIURL  = getEnvOrDefault("LONGHOUSE_API_URL", "http://localhost:6080")
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

func ApplyFlags(flags map[string]string) {
	if v, ok := flags["port"]; ok {
		if i, err := strconv.Atoi(v); err == nil {
			WebPort = i
		}
	}
	if v, ok := flags["api-url"]; ok {
		APIURL = v
	}
}

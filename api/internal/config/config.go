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

	// Linkkeys RP PKI sidecar — the api uses it to verify signed assertions
	// presented by clients (webapp, CLIs, mobile, etc.) at /auth/login.
	LinkkeysPKIURL          = getEnvOrDefault("LONGHOUSE_LINKKEYS_PKI_URL", "")
	LinkkeysPKIAPIKey       = getEnvOrDefault("LONGHOUSE_LINKKEYS_PKI_API_KEY", "")
	LinkkeysPKIAllowInvalid = getEnvOrDefault("LONGHOUSE_LINKKEYS_PKI_ALLOW_INVALID_CERTS", "") == "true"

	// IDP whose assertions we trust. We pin a single IDP per deployment;
	// federated multi-IDP can come later.
	LinkkeysIDPDomain = getEnvOrDefault("LONGHOUSE_LINKKEYS_IDP_DOMAIN", "")

	// JWT signing secret. HMAC-SHA256 over a base64url'd JSON payload.
	// Required for the api to issue tokens at /auth/login.
	JWTSecret = getEnvOrDefault("LONGHOUSE_JWT_SECRET", "")

	// Recurrence worker. Disabled defaults to false (worker on by default);
	// set LONGHOUSE_RECURRENCE_DISABLED=true in environments where the
	// app is observed-only or under test. Tick interval is 60s by default
	// — small enough to keep latency tight, big enough to keep DB load
	// negligible.
	RecurrenceDisabled        = getEnvOrDefault("LONGHOUSE_RECURRENCE_DISABLED", "") == "true"
	RecurrenceTickIntervalSec = getEnvAsIntOrDefault("LONGHOUSE_RECURRENCE_TICK_SECONDS", 60)
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
	if v, ok := flags["linkkeys-pki-url"]; ok {
		LinkkeysPKIURL = v
	}
	if v, ok := flags["linkkeys-pki-api-key"]; ok {
		LinkkeysPKIAPIKey = v
	}
	if v, ok := flags["linkkeys-idp-domain"]; ok {
		LinkkeysIDPDomain = v
	}
	if v, ok := flags["jwt-secret"]; ok {
		JWTSecret = v
	}
}

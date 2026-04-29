package config

import (
	"os"
	"strconv"
)

var (
	WebPort = getEnvAsIntOrDefault("LONGHOUSE_WEB_PORT", 4080)
	APIURL  = getEnvOrDefault("LONGHOUSE_API_URL", "http://localhost:6080")

	// Linkkeys RP PKI sidecar — the service that holds our private keys.
	LinkkeysPKIURL          = getEnvOrDefault("LONGHOUSE_LINKKEYS_PKI_URL", "")
	LinkkeysPKIAPIKey       = getEnvOrDefault("LONGHOUSE_LINKKEYS_PKI_API_KEY", "")
	LinkkeysPKIAllowInvalid = getEnvOrDefault("LONGHOUSE_LINKKEYS_PKI_ALLOW_INVALID_CERTS", "") == "true"

	// Linkkeys IDP — where the user is redirected to authenticate.
	LinkkeysIDPURL    = getEnvOrDefault("LONGHOUSE_LINKKEYS_IDP_URL", "")
	LinkkeysIDPDomain = getEnvOrDefault("LONGHOUSE_LINKKEYS_IDP_DOMAIN", "")

	// This longhouse instance as a relying party.
	RPDomain      = getEnvOrDefault("LONGHOUSE_RP_DOMAIN", "")
	RPCallbackURL = getEnvOrDefault("LONGHOUSE_RP_CALLBACK_URL", "")

	// Session cookie signing secret. Must be non-empty for auth to work.
	SessionSecret = getEnvOrDefault("LONGHOUSE_SESSION_SECRET", "")
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
	if v, ok := flags["linkkeys-pki-url"]; ok {
		LinkkeysPKIURL = v
	}
	if v, ok := flags["linkkeys-pki-api-key"]; ok {
		LinkkeysPKIAPIKey = v
	}
	if v, ok := flags["linkkeys-idp-url"]; ok {
		LinkkeysIDPURL = v
	}
	if v, ok := flags["linkkeys-idp-domain"]; ok {
		LinkkeysIDPDomain = v
	}
	if v, ok := flags["rp-domain"]; ok {
		RPDomain = v
	}
	if v, ok := flags["rp-callback-url"]; ok {
		RPCallbackURL = v
	}
	if v, ok := flags["session-secret"]; ok {
		SessionSecret = v
	}
}

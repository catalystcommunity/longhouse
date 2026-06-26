package config

import (
	"os"
	"strconv"
)

var (
	DbUri = getEnvOrDefault("LONGHOUSE_DB_URI", "postgresql://longhouse:devpass123@localhost:5432/longhouse_db?sslmode=disable")

	APIPort = getEnvAsIntOrDefault("LONGHOUSE_API_PORT", 6080)
	TCPPort = getEnvAsIntOrDefault("LONGHOUSE_TCP_PORT", 6081)

	// LinkkeysDomain is our relying-party DNS identity. linkkeys binds each
	// assertion to it via the `audience` claim, so this is the value the auth
	// layer expects on assertion.Audience (see csilservices.AuthService.RPDomain).
	// In the single-IDP self-RP deployment it equals LinkkeysIDPDomain.
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

	// LinkkeysTransport selects how the api reaches the linkkeys RP: "http"
	// (the legacy JSON PKI sidecar, see linkkeys.Client) or "tcp" (the
	// canonical CSIL-RPC/TCP endpoint, see linkkeys.TCPClient). Defaults to
	// http for back-compat; flip to tcp once the deployment publishes a
	// reachable CSIL-RPC endpoint.
	LinkkeysTransport = getEnvOrDefault("LONGHOUSE_LINKKEYS_TRANSPORT", "http")

	// TCP transport knobs (used only when LinkkeysTransport == "tcp"). Addr
	// and Fingerprints are optional overrides; when empty they're discovered
	// from DNS off LinkkeysIDPDomain (the _linkkeys_apis / _linkkeys TXT
	// records). AllowInsecure skips server-cert fingerprint pinning — dev
	// clusters with self-signed certs only.
	LinkkeysTCPAddr          = getEnvOrDefault("LONGHOUSE_LINKKEYS_TCP_ADDR", "")
	LinkkeysTCPFingerprints  = getEnvOrDefault("LONGHOUSE_LINKKEYS_TCP_FINGERPRINTS", "")
	LinkkeysTCPAllowInsecure = getEnvOrDefault("LONGHOUSE_LINKKEYS_TCP_ALLOW_INSECURE", "") == "true"

	// IDP whose assertions we trust. We pin a single IDP per deployment;
	// federated multi-IDP can come later.
	LinkkeysIDPDomain = getEnvOrDefault("LONGHOUSE_LINKKEYS_IDP_DOMAIN", "")

	// LinkkeysIDPURL is the base URL of the IDP's authorize page (the host,
	// not the identity domain) — e.g. https://linkkeys.todandlorna.com. The
	// browser flow redirects to <IDPURL>/auth/authorize?signed_request=...
	LinkkeysIDPURL = getEnvOrDefault("LONGHOUSE_LINKKEYS_IDP_URL", "")

	// AppCallbackURL is the SPA route the IDP returns the encrypted token to —
	// e.g. https://app.example/auth/callback (prod) or
	// http://localhost:5173/auth/callback (dev). It's only the redirect target
	// we advertise via sign-request; the assertion's audience is the RP domain
	// (LinkkeysDomain), not this URL.
	AppCallbackURL = getEnvOrDefault("LONGHOUSE_APP_CALLBACK_URL", "")

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

	// Notification cull worker. The feed prunes itself: notification events
	// (and their per-recipient rows, via cascade) older than the retention
	// window are deleted. Defaults: keep ~6 months, sweep hourly. Disable in
	// observed-only or test environments.
	NotificationCullDisabled    = getEnvOrDefault("LONGHOUSE_NOTIFICATION_CULL_DISABLED", "") == "true"
	NotificationCullTickSeconds = getEnvAsIntOrDefault("LONGHOUSE_NOTIFICATION_CULL_TICK_SECONDS", 3600)
	NotificationRetentionDays   = getEnvAsIntOrDefault("LONGHOUSE_NOTIFICATION_RETENTION_DAYS", 180)

	// Trash purge worker. Soft-deleted (trashed) rows older than the retention
	// window are permanently deleted; the audit record of the delete survives
	// (audit retention is much longer), so "who deleted X" outlives the
	// ability to restore it. Defaults: 30-day trash, sweep hourly.
	TrashPurgeDisabled    = getEnvOrDefault("LONGHOUSE_TRASH_PURGE_DISABLED", "") == "true"
	TrashPurgeTickSeconds = getEnvAsIntOrDefault("LONGHOUSE_TRASH_PURGE_TICK_SECONDS", 3600)
	TrashRetentionDays    = getEnvAsIntOrDefault("LONGHOUSE_TRASH_RETENTION_DAYS", 30)

	// Audit partition maintenance worker. Creates monthly audit_log partitions
	// ahead of time and drops those older than the retention window. Defaults:
	// keep 24 months, keep 2 months of partitions pre-created, sweep hourly.
	AuditPartitionDisabled    = getEnvOrDefault("LONGHOUSE_AUDIT_PARTITION_DISABLED", "") == "true"
	AuditPartitionTickSeconds = getEnvAsIntOrDefault("LONGHOUSE_AUDIT_PARTITION_TICK_SECONDS", 3600)
	AuditRetentionMonths      = getEnvAsIntOrDefault("LONGHOUSE_AUDIT_RETENTION_MONTHS", 24)
	AuditPartitionAheadMonths = getEnvAsIntOrDefault("LONGHOUSE_AUDIT_PARTITION_AHEAD_MONTHS", 2)

	// Env classifies the deployment. Missing reads as "prod" — the safe
	// interpretation. Other accepted values: "nonprod" (shared non-prod
	// envs) and "dev" (local). Used only to gate dev-mode features today;
	// callers should always treat anything other than "dev" / "nonprod"
	// as production.
	Env = getEnvOrDefault("LONGHOUSE_ENV", "prod")

	// DevAuthEnabled, combined with Env != prod, registers a parallel
	// login endpoint at /api/v1/dev/login that mints real JWTs without
	// the linkkeys assertion exchange. The endpoint 404s (is not
	// registered) when this is false OR Env is prod. Every dev login is
	// logged at WARN so the trail is impossible to miss in shared envs.
	DevAuthEnabled = getEnvOrDefault("LONGHOUSE_DEV_AUTH_ENABLED", "") == "true"
)

// DevAuthAllowed reports whether the dev-auth endpoints should be wired
// into the router at startup. Both gates must pass; the env-must-not-be-prod
// rule means a stray DEV_AUTH_ENABLED=true in prod is a no-op (and is
// logged at WARN by Serve()).
func DevAuthAllowed() bool {
	return DevAuthEnabled && (Env == "dev" || Env == "nonprod")
}

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
	if v, ok := flags["linkkeys-transport"]; ok {
		LinkkeysTransport = v
	}
	if v, ok := flags["linkkeys-tcp-addr"]; ok {
		LinkkeysTCPAddr = v
	}
	if v, ok := flags["linkkeys-tcp-fingerprints"]; ok {
		LinkkeysTCPFingerprints = v
	}
	if v, ok := flags["linkkeys-idp-domain"]; ok {
		LinkkeysIDPDomain = v
	}
	if v, ok := flags["linkkeys-idp-url"]; ok {
		LinkkeysIDPURL = v
	}
	if v, ok := flags["app-callback-url"]; ok {
		AppCallbackURL = v
	}
	if v, ok := flags["jwt-secret"]; ok {
		JWTSecret = v
	}
}

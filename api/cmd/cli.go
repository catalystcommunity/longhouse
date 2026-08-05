package cmd

import (
	"fmt"
	"os"
	"strings"
)

const usage = `longhouse - coordination system for organizations and neighborhoods

Usage:
  longhouse serve [--db-uri=URI] [--api-port=PORT] [--jwt-secret=SECRET]
                  [--initial-admin-domain=DOMAIN] [--initial-admin-user-id=UUID]
                  [--initial-house-name=NAME]
                  [--linkkeys-idp-domain=DOMAIN] [--linkkeys-idp-url=URL]
                  [--app-callback-url=URL] [--linkkeys-transport=http|tcp]
                  [--linkkeys-pki-url=URL] [--linkkeys-pki-api-key=KEY]
                  [--linkkeys-tcp-addr=HOST:PORT] [--linkkeys-tcp-fingerprints=LIST]
  longhouse migrate [--db-uri=URI]
  longhouse --help
  longhouse --version

Commands:
  serve     Start the API server (runs migrations first, then listens on --api-port)
  migrate   Run database migrations

Options:
  --db-uri=URI                    PostgreSQL connection string [env: LONGHOUSE_DB_URI]
  --api-port=PORT                 HTTP API port [default: 6080] [env: LONGHOUSE_API_PORT]
  --jwt-secret=SECRET             HMAC secret for minting/verifying bearers; empty fails
                                  every authenticated op closed [env: LONGHOUSE_JWT_SECRET]
  --initial-admin-domain=DOMAIN   Linkkeys domain of the bootstrap admin [env: LONGHOUSE_INITIAL_ADMIN_DOMAIN]
  --initial-admin-user-id=UUID    Linkkeys user_id (UUID) of the bootstrap admin [env: LONGHOUSE_INITIAL_ADMIN_USER_ID]
  --initial-house-name=NAME       Name for the auto-created house on first boot [default: Longhouse] [env: LONGHOUSE_INITIAL_HOUSE_NAME]
  --linkkeys-idp-domain=DOMAIN    Identity domain whose assertions we trust [env: LONGHOUSE_LINKKEYS_IDP_DOMAIN]
  --linkkeys-idp-url=URL          Base URL of the IDP authorize page [env: LONGHOUSE_LINKKEYS_IDP_URL]
  --app-callback-url=URL          SPA route the IDP returns the token to [env: LONGHOUSE_APP_CALLBACK_URL]
  --linkkeys-transport=NAME       How to reach the linkkeys RP: http or tcp [default: http] [env: LONGHOUSE_LINKKEYS_TRANSPORT]
  --linkkeys-pki-url=URL          RP PKI sidecar base URL (http transport) [env: LONGHOUSE_LINKKEYS_PKI_URL]
  --linkkeys-pki-api-key=KEY      RP API key, both transports [env: LONGHOUSE_LINKKEYS_PKI_API_KEY]
  --linkkeys-tcp-addr=HOST:PORT   RP CSIL-RPC endpoint; empty discovers it from DNS (tcp transport) [env: LONGHOUSE_LINKKEYS_TCP_ADDR]
  --linkkeys-tcp-fingerprints=LIST  Comma-separated pinned server-cert fingerprints (tcp transport) [env: LONGHOUSE_LINKKEYS_TCP_FINGERPRINTS]
  --help                          Show this help message
  --version                       Show version

Env-only settings (no flag) are listed in api/internal/config/config.go — the
recurrence, notification-cull, trash-purge and audit-partition worker knobs,
LONGHOUSE_ENV, and LONGHOUSE_DEV_AUTH_ENABLED.
`

// version is bumped by .reactorcide/jobs/scripts/release.sh alongside
// version/VERSION.txt and helm_chart/Chart.yaml.
const version = "0.14.6"

// Run parses args and dispatches to the appropriate subcommand.
func Run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}

	// Flags may appear anywhere in the arg list, so --version/--help are
	// answered before we look for a subcommand — `longhouse --version` has
	// no command word and must still print the version, not the usage.
	flags := parseFlags(args)

	if flags["version"] == "true" {
		fmt.Printf("longhouse %s\n", version)
		return nil
	}

	command := findCommand(args)
	if flags["help"] == "true" || command == "" {
		fmt.Print(usage)
		return nil
	}

	switch command {
	case "serve":
		return Serve(flags)
	case "migrate":
		return Migrate(flags)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", command)
		fmt.Print(usage)
		return fmt.Errorf("unknown command: %s", command)
	}
}

// findCommand returns the first non-flag argument.
func findCommand(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

// parseFlags extracts --key=value and --flag style arguments from anywhere in the list.
func parseFlags(args []string) map[string]string {
	flags := make(map[string]string)
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		arg = strings.TrimLeft(arg, "-")
		if idx := strings.Index(arg, "="); idx >= 0 {
			flags[arg[:idx]] = arg[idx+1:]
		} else {
			flags[arg] = "true"
		}
	}
	return flags
}

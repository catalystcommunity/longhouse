package cmd

import (
	"fmt"
	"os"
	"strings"
)

const usage = `longhouse-web - web UI for longhouse

Usage:
  longhouse-web serve [--port=PORT] [--api-url=URL]
                      [--linkkeys-pki-url=URL] [--linkkeys-pki-api-key=KEY]
                      [--linkkeys-idp-url=URL] [--linkkeys-idp-domain=DOMAIN]
                      [--rp-callback-url=URL]
                      [--session-secret=SECRET]
  longhouse-web --help
  longhouse-web --version

Commands:
  serve    Start the web UI server

Options:
  --port=PORT                  Web UI port [default: 4080] [env: LONGHOUSE_WEB_PORT]
  --api-url=URL                API server URL [default: http://localhost:6080] [env: LONGHOUSE_API_URL]
  --linkkeys-pki-url=URL       Linkkeys RP PKI sidecar base URL [env: LONGHOUSE_LINKKEYS_PKI_URL]
  --linkkeys-pki-api-key=KEY   Bearer token for the PKI sidecar [env: LONGHOUSE_LINKKEYS_PKI_API_KEY]
  --linkkeys-idp-url=URL       Linkkeys IDP base URL for the /auth/authorize redirect [env: LONGHOUSE_LINKKEYS_IDP_URL]
  --linkkeys-idp-domain=DOMAIN Linkkeys IDP domain (also the RP identity domain
                                used to validate returned assertions' audience)
                                [env: LONGHOUSE_LINKKEYS_IDP_DOMAIN]
  --rp-callback-url=URL        Public callback URL for this RP [env: LONGHOUSE_RP_CALLBACK_URL]
  --session-secret=SECRET      HMAC secret for session cookies [env: LONGHOUSE_SESSION_SECRET]
  --help                       Show this help message
  --version                    Show version
`

const version = "0.1.0"

func Run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}

	flags := parseFlags(args)
	command := findCommand(args)

	if flags["help"] == "true" || command == "" {
		fmt.Print(usage)
		return nil
	}

	if flags["version"] == "true" {
		fmt.Printf("longhouse-web %s\n", version)
		return nil
	}

	switch command {
	case "serve":
		return Serve(flags)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", command)
		fmt.Print(usage)
		return fmt.Errorf("unknown command: %s", command)
	}
}

func findCommand(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

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

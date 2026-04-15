package cmd

import (
	"fmt"
	"os"
	"strings"
)

const usage = `longhouse - coordination system for organizations and neighborhoods

Usage:
  longhouse serve [--db-uri=URI] [--api-port=PORT] [--tcp-port=PORT]
  longhouse migrate [--db-uri=URI]
  longhouse --help
  longhouse --version

Commands:
  serve     Start the API server (HTTP + TCP)
  migrate   Run database migrations

Options:
  --db-uri=URI      PostgreSQL connection string [env: LONGHOUSE_DB_URI]
  --api-port=PORT   HTTP API port [default: 6080] [env: LONGHOUSE_API_PORT]
  --tcp-port=PORT   TCP/CSIL protocol port [default: 6081] [env: LONGHOUSE_TCP_PORT]
  --help            Show this help message
  --version         Show version
`

const version = "0.1.0"

// Run parses args and dispatches to the appropriate subcommand.
func Run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}

	// Parse flags from anywhere in the arg list
	flags := parseFlags(args)
	command := findCommand(args)

	if flags["help"] == "true" || command == "" {
		fmt.Print(usage)
		return nil
	}

	if flags["version"] == "true" {
		fmt.Printf("longhouse %s\n", version)
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

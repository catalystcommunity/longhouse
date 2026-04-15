package cmd

import (
	"fmt"
	"os"
	"strings"
)

const usage = `longhouse-web - web UI for longhouse

Usage:
  longhouse-web serve [--port=PORT] [--api-url=URL]
  longhouse-web --help
  longhouse-web --version

Commands:
  serve    Start the web UI server

Options:
  --port=PORT      Web UI port [default: 4080] [env: LONGHOUSE_WEB_PORT]
  --api-url=URL    API server URL [default: http://localhost:6080] [env: LONGHOUSE_API_URL]
  --help           Show this help message
  --version        Show version
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

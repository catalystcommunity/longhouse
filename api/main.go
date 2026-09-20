package main

import (
	"fmt"
	"os"

	"github.com/catalystcommunity/longhouse/api/cmd"
)

func main() {
	if err := cmd.Run(os.Args[1:]); err != nil {
		if !cmd.ErrorWasReported(err) {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		os.Exit(cmd.ExitCode(err))
	}
}

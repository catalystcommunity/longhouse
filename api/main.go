package main

import (
	"fmt"
	"os"
	_ "time/tzdata" // The scratch image has no system time-zone database.

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

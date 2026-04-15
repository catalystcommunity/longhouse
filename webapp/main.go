package main

import (
	"fmt"
	"os"

	"github.com/catalystcommunity/longhouse/webapp/cmd"
)

func main() {
	if err := cmd.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

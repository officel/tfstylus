package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/officel/tfstylus/internal/app"
)

var version = "dev"

func main() {
	if err := app.RunWithVersion(os.Args[1:], os.Stdout, version); err != nil {
		var exitError interface{ ExitCode() int }
		if errors.As(err, &exitError) {
			if exitError.ExitCode() != 1 {
				fmt.Fprintln(os.Stderr, err)
			}
			os.Exit(exitError.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

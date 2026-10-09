package app

import (
	"errors"
	"fmt"
	"io"
)

func Run(args []string, output io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "--help", "-h":
			_, err := fmt.Fprintln(output, "Usage: tfstylus <command> [flags]")
			return err
		case "--version", "-v":
			_, err := fmt.Fprintln(output, "Version: dev")
			return err
		case "check", "run":
			return errors.New("commands are not implemented yet")
		default:
			return fmt.Errorf("unknown command: %s", args[0])
		}
	}

	_, err := fmt.Fprintln(output, "Version: dev\nUsage: tfstylus <command> [flags]")
	return err
}

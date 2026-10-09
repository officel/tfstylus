package app

import (
	"errors"
	"fmt"
	"io"
)

func Run(args []string, output io.Writer) error {
	if len(args) > 0 {
		return errors.New("commands are not implemented yet")
	}

	_, err := fmt.Fprintln(output, "Version: dev\nUsage: tfstylus <command> [flags]")
	return err
}

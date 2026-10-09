package app

import (
	"errors"
	"fmt"
	"io"
)

func Run(args []string, output io.Writer) error {
	commandArgs, configPath, err := parseConfigArgs(args)
	if err != nil {
		return err
	}

	if len(commandArgs) > 0 {
		switch commandArgs[0] {
		case "--help", "-h":
			_, err = fmt.Fprintln(output, "Usage: tfstylus <command> [flags]")
			return err
		case "--version", "-v":
			_, err = fmt.Fprintln(output, "Version: dev")
			return err
		case "check", "run":
			if configPath != "" {
				if _, err := loadConfig(configPath); err != nil {
					return err
				}
			}
			return errors.New("commands are not implemented yet")
		default:
			return fmt.Errorf("unknown command: %s", commandArgs[0])
		}
	}

	_, err = fmt.Fprintln(output, "Version: dev\nUsage: tfstylus <command> [flags]")
	return err
}

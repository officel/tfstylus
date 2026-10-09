package app

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

const usage = "Usage: tfstylus <command> [flags]\n\nFlags:\n  -c, --config PATH  Load an explicit configuration file\n  --chdir DIR        Set the target directory (default: current directory)\n  --recursive        Search directories recursively"

type options struct {
	command    string
	configPath string
	directory  string
	recursive  bool
}

func Run(args []string, output io.Writer) error {
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}

	if opts.command != "" {
		switch opts.command {
		case "--help", "-h":
			_, err = fmt.Fprintln(output, usage)
			return err
		case "--version", "-v":
			_, err = fmt.Fprintln(output, "Version: dev")
			return err
		case "check", "run":
			targets, err := discoverTargetFiles(opts.directory, opts.recursive)
			if err != nil {
				return err
			}
			for _, target := range targets {
				configPath := opts.configPath
				if configPath == "" {
					configPath, err = discoverConfig(target.directory)
					if err != nil {
						return err
					}
				} else if !filepath.IsAbs(configPath) {
					configPath = filepath.Join(target.directory, configPath)
				}
				if configPath != "" {
					if _, err := loadConfig(configPath); err != nil {
						return err
					}
				}
			}
			var parseErrors []error
			for _, target := range targets {
				for _, filename := range target.files {
					if err := parseTargetFile(filename); err != nil {
						parseErrors = append(parseErrors, err)
					}
				}
			}
			if err := errors.Join(parseErrors...); err != nil {
				return err
			}
			return errors.New("commands are not implemented yet")
		default:
			return fmt.Errorf("unknown command: %s", opts.command)
		}
	}

	_, err = fmt.Fprintln(output, "Version: dev\n"+usage)
	return err
}

func parseArgs(args []string) (options, error) {
	opts := options{directory: "."}
	chdirSet := false
	for index := 0; index < len(args); index++ {
		argument := args[index]
		flag, value, hasValue := strings.Cut(argument, "=")
		switch flag {
		case "-c", "--config", "--chdir":
			if flag == "--chdir" && chdirSet {
				return options{}, errors.New("chdir may only be specified once")
			}
			if flag != "--chdir" && opts.configPath != "" {
				return options{}, errors.New("config may only be specified once")
			}
			if !hasValue {
				index++
				if index >= len(args) {
					return options{}, fmt.Errorf("missing value for %s", flag)
				}
				value = args[index]
			}
			if value == "" || strings.HasPrefix(value, "-") {
				return options{}, fmt.Errorf("missing value for %s", flag)
			}
			if flag == "--chdir" {
				opts.directory, chdirSet = value, true
			} else {
				opts.configPath = value
			}
		case "--recursive":
			opts.recursive = true
			if hasValue {
				recursive, err := strconv.ParseBool(value)
				if err != nil {
					return options{}, fmt.Errorf("invalid value for --recursive: %q", value)
				}
				opts.recursive = recursive
			}
		default:
			if strings.HasPrefix(argument, "-") && argument != "--help" && argument != "-h" && argument != "--version" && argument != "-v" {
				return options{}, fmt.Errorf("unknown option: %s", argument)
			}
			if opts.command != "" {
				return options{}, fmt.Errorf("unexpected argument: %s", argument)
			}
			opts.command = argument
		}
	}
	return opts, nil
}

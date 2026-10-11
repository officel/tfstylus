package app

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

const usage = "Usage: tfstylus <command> [flags]\n\nFlags:\n  -c, --config PATH  Load an explicit configuration file\n  --chdir DIR        Set the target directory (default: current directory)\n  --recursive        Search directories recursively\n  --diff             Show a unified diff (check only)"

type options struct {
	command    string
	configPath string
	directory  string
	recursive  bool
	diff       bool
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
				return withExitCode(2, err)
			}
			configs := make([]Config, len(targets))
			for index, target := range targets {
				config := Config{RequiredVersionOrder: true, BackendOrder: true, CloudOrder: true, CloudWorkspacesOrder: true, RequiredProvidersOrder: true, VariableOrder: true, OutputOrder: true}
				configPath := opts.configPath
				if configPath == "" {
					configPath, err = discoverConfig(target.directory)
					if err != nil {
						return withExitCode(2, err)
					}
				} else if !filepath.IsAbs(configPath) {
					configPath = filepath.Join(target.directory, configPath)
				}
				if configPath != "" {
					config, err = loadConfig(configPath)
					if err != nil {
						return withExitCode(2, err)
					}
				}
				configs[index] = config
			}
			if opts.command == "check" {
				return runCheck(targets, configs, opts.directory, opts.diff, output)
			}
			return runTargets(targets, configs)
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
		case "--diff":
			if hasValue {
				return options{}, errors.New("--diff does not accept a value")
			}
			opts.diff = true
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
	if opts.diff && opts.command != "check" {
		return options{}, errors.New("--diff is only supported by the check command")
	}
	return opts, nil
}

package app

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

const usage = "Usage: tfstylus <command> [flags]\n\nCommands:\n  check              Check files without modifying them\n  run                Apply style fixes\n\nFlags:\n  -c, --config PATH  Load an explicit configuration file\n  --chdir DIR        Set the target directory (default: current directory)\n  -r, --recursive    Search directories recursively\n  -d, --diff         Show a unified diff (check only)\n  -h, --help         Show this help\n  -v, --version      Show version"

type options struct {
	command    string
	configPath string
	directory  string
	recursive  bool
	diff       bool
}

func Run(args []string, output io.Writer) error {
	return RunWithVersion(args, output, "dev")
}

func RunWithVersion(args []string, output io.Writer, version string) error {
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
			_, err = fmt.Fprintln(output, "Version:", version)
			return err
		case "check", "run":
			targets, err := discoverTargetFiles(opts.directory, opts.recursive)
			if err != nil {
				return withExitCode(2, err)
			}
			configs := make([]Config, len(targets))
			for index, target := range targets {
				config := Config{RequiredVersionOrder: true, BackendOrder: true, CloudOrder: true, CloudWorkspacesOrder: true, RequiredProvidersOrder: true, ProviderOrder: true, VariableOrder: true, OutputOrder: true, DataOrder: true, ActionOrder: true, CheckOrder: true, EphemeralOrder: true, ResourceOrder: true, ModuleOrder: true, ListOrder: true, ImportOrder: true, MovedOrder: true, RemovedOrder: true, LocalsFileRename: true, OutputsFileRename: true, VariablesFileRename: true, TerraformFileRename: true}
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

	_, err = fmt.Fprintf(output, "Version: %s\n%s\n", version, usage)
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
		case "-r", "--recursive":
			opts.recursive = true
			if hasValue {
				recursive, err := strconv.ParseBool(value)
				if err != nil {
					return options{}, fmt.Errorf("invalid value for %s: %q", flag, value)
				}
				opts.recursive = recursive
			}
		case "-d", "--diff":
			if hasValue {
				return options{}, fmt.Errorf("%s does not accept a value", flag)
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

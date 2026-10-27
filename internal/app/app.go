package app

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const usage = "Usage: tfstylus <command> [flags]\n\nCommands:\n  init               Create a configuration with all rules disabled\n  plan               Show planned changes and a unified diff without modifying files\n  apply              Apply style fixes\n  version            Show version\n\nFlags:\n  -c, --config PATH  Configuration file path (init: output path)\n  --chdir DIR        Set the target directory (default: current directory)\n  -r, --recursive    Search directories recursively (plan/apply)\n  -d, --diff         Show a unified diff after applying changes (apply only)\n  -v, --verbose...  Use verbose output (-v: info, -vv: debug, -vvv: trace)\n  -h, --help         Show this help"

type options struct {
	verbosity  int
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
	return RunWithWriters(args, output, io.Discard, version)
}

// RunWithWriters keeps command output separate from diagnostic logs.
func RunWithWriters(args []string, output, diagnostics io.Writer, version string) (result error) {
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}

	log := func(level int, label, format string, values ...any) {
		if opts.verbosity >= level {
			_, _ = fmt.Fprintf(diagnostics, "[%s] %s\n", label, fmt.Sprintf(format, values...))
		}
	}
	if opts.command != "" && opts.command != "-h" && opts.command != "--help" {
		log(1, "INFO", "starting %s", opts.command)
		defer func() {
			status := "completed"
			if result != nil {
				status = "finished with a nonzero status"
			}
			log(1, "INFO", "%s %s", opts.command, status)
		}()
	}

	if opts.command != "" {
		switch opts.command {
		case "--help", "-h":
			_, err = fmt.Fprintln(output, usage)
			return err
		case "version":
			_, err = fmt.Fprintln(output, "Version:", version)
			return err
		case "init":
			if err := runInit(opts, output); err != nil {
				return withExitCode(2, err)
			}
			return nil
		case "plan", "apply":
			log(3, "TRACE", "discovering targets in %q (recursive=%t)", opts.directory, opts.recursive)
			targets, err := discoverTargetFiles(opts.directory, opts.recursive)
			if err != nil {
				return withExitCode(2, err)
			}
			fileCount := 0
			for _, target := range targets {
				fileCount += len(target.files)
			}
			log(1, "INFO", "found %d directories and %d files", len(targets), fileCount)
			configs := make([]Config, len(targets))
			for index, target := range targets {
				config := Config{RequiredVersionOrder: true, BackendOrder: true, CloudOrder: true, CloudWorkspacesOrder: true, RequiredProvidersOrder: true, ProviderOrder: true, VariableOrder: true, OutputOrder: true, DataOrder: true, ActionOrder: true, CheckOrder: true, EphemeralOrder: true, ResourceOrder: true, ModuleOrder: true, ListOrder: true, ImportOrder: true, MovedOrder: true, RemovedOrder: true, LocalsFileRename: true, OutputsFileRename: true, VariablesFileRename: true, TerraformFileRename: true}
				log(3, "TRACE", "resolving configuration for %q", target.directory)
				for _, file := range target.files {
					log(2, "DEBUG", "target %q", file)
				}
				configPath := opts.configPath
				if configPath == "" {
					configPath, err = discoverConfig(target.directory)
					if err != nil {
						return withExitCode(2, err)
					}
				}
				if configPath != "" {
					log(2, "DEBUG", "configuration %q", configPath)
					config, err = loadConfig(configPath)
					if err != nil {
						return withExitCode(2, err)
					}
				}
				if configPath == "" {
					log(2, "DEBUG", "using default configuration for %q", target.directory)
				}
				configs[index] = config
			}
			log(3, "TRACE", "preparing and validating %s candidates", opts.command)
			if opts.command == "plan" {
				return runPlan(targets, configs, opts.directory, output)
			}
			return applyTargets(targets, configs, opts.directory, opts.diff, output)
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
				remaining := args[index+1:]
				if len(remaining) == 0 {
					return options{}, fmt.Errorf("missing value for %s", flag)
				}
				value = remaining[0]
				index++
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
		case "-v", "--verbose":
			if hasValue {
				return options{}, fmt.Errorf("%s does not accept a value", flag)
			}
			if opts.verbosity < 3 {
				opts.verbosity++
			}
		case "-d", "--diff":
			if hasValue {
				return options{}, fmt.Errorf("%s does not accept a value", flag)
			}
			opts.diff = true
		default:
			if strings.HasPrefix(argument, "-v") && strings.TrimLeft(argument[1:], "v") == "" {
				opts.verbosity += len(argument) - 1
				if opts.verbosity > 3 {
					opts.verbosity = 3
				}
				continue
			}
			if strings.HasPrefix(argument, "-") && argument != "--help" && argument != "-h" {
				return options{}, fmt.Errorf("unknown option: %s", argument)
			}
			if opts.command != "" {
				return options{}, fmt.Errorf("unexpected argument: %s", argument)
			}
			opts.command = argument
		}
	}
	if opts.diff && opts.command != "apply" {
		return options{}, errors.New("--diff is only supported by the apply command")
	}
	return opts, nil
}

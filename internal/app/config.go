package app

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type Config struct{}

func loadConfig(path string) (Config, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	file, diagnostics := hclsyntax.ParseConfig(source, path, hcl.Pos{Line: 1, Column: 1})
	if diagnostics.HasErrors() {
		return Config{}, fmt.Errorf("parse config %q: %s", path, diagnostics.Error())
	}

	if _, diagnostics := file.Body.Content(&hcl.BodySchema{}); diagnostics.HasErrors() {
		return Config{}, fmt.Errorf("validate config %q: %s", path, diagnostics.Error())
	}

	return Config{}, nil
}

func parseConfigArgs(args []string) ([]string, string, error) {
	commandArgs := make([]string, 0, len(args))
	var configPath string

	for index := 0; index < len(args); index++ {
		if args[index] != "-c" && args[index] != "--config" {
			commandArgs = append(commandArgs, args[index])
			continue
		}

		if configPath != "" {
			return nil, "", errors.New("config may only be specified once")
		}
		if index+1 >= len(args) || args[index+1] == "" || strings.HasPrefix(args[index+1], "-") {
			return nil, "", fmt.Errorf("missing value for %s", args[index])
		}

		configPath = args[index+1]
		index++
	}

	return commandArgs, configPath, nil
}

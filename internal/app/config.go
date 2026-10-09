package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type Config struct{}

func discoverConfig(startDir string) (string, error) {
	directory, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolve config search directory %q: %w", startDir, err)
	}

	for {
		configPath, found, err := configInDirectory(directory)
		if err != nil {
			return "", err
		}
		if found {
			return configPath, nil
		}

		parent := filepath.Dir(directory)
		if parent == directory {
			homeDirectory, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve user home directory: %w", err)
			}
			homeDirectory, err = filepath.Abs(homeDirectory)
			if err != nil {
				return "", fmt.Errorf("resolve user home directory %q: %w", homeDirectory, err)
			}
			if homeDirectory == directory {
				return "", nil
			}
			configPath, found, err = configInDirectory(homeDirectory)
			if err != nil {
				return "", err
			}
			if found {
				return configPath, nil
			}
			return "", nil
		}
		directory = parent
	}
}

func configInDirectory(directory string) (string, bool, error) {
	candidates := []string{
		filepath.Join(directory, ".tfstylus.hcl"),
		filepath.Join(directory, ".config", ".tfstylus.hcl"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", false, fmt.Errorf("inspect config %q: %w", candidate, err)
		}
		if !info.Mode().IsRegular() {
			return "", false, fmt.Errorf("config %q is not a regular file", candidate)
		}
		return candidate, true, nil
	}
	return "", false, nil
}

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

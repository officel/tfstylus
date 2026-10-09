package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoArgsPrintsHelpAndVersion(t *testing.T) {
	var output bytes.Buffer

	err := Run(nil, &output)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := output.String()
	if !strings.Contains(got, "Usage:") {
		t.Errorf("Run() output = %q, want help text", got)
	}
	if !strings.Contains(got, "Version: dev") {
		t.Errorf("Run() output = %q, want development version", got)
	}
}

func TestHelpAndVersionOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "help", args: []string{"--help"}, want: "Usage: tfstylus <command> [flags]\n"},
		{name: "short help", args: []string{"-h"}, want: "Usage: tfstylus <command> [flags]\n"},
		{name: "version", args: []string{"--version"}, want: "Version: dev\n"},
		{name: "short version", args: []string{"-v"}, want: "Version: dev\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer

			if err := Run(test.args, &output); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := output.String(); got != test.want {
				t.Errorf("Run() output = %q, want %q", got, test.want)
			}
		})
	}
}

func TestUnknownCommand(t *testing.T) {
	var output bytes.Buffer

	err := Run([]string{"frobnicate"}, &output)
	if err == nil {
		t.Fatal("Run() error = nil, want unknown-command error")
	}
	if got, want := err.Error(), "unknown command: frobnicate"; got != want {
		t.Errorf("Run() error = %q, want %q", got, want)
	}
	if got := output.String(); got != "" {
		t.Errorf("Run() output = %q, want empty output", got)
	}
}

func TestKnownCommandNotImplemented(t *testing.T) {
	for _, command := range []string{"check", "run"} {
		t.Run(command, func(t *testing.T) {
			err := runFromDirectory(t, t.TempDir(), []string{command})
			if err == nil {
				t.Fatal("Run() error = nil, want not-implemented error")
			}
			if got, want := err.Error(), "commands are not implemented yet"; got != want {
				t.Errorf("Run() error = %q, want %q", got, want)
			}
		})
	}
}

func TestRunConfigDiscovery(t *testing.T) {
	t.Run("loads config from current directory", func(t *testing.T) {
		workingDirectory := t.TempDir()
		writeConfig(t, filepath.Join(workingDirectory, ".tfstylus.hcl"), "")

		err := runFromDirectory(t, workingDirectory, []string{"check"})
		if err == nil || err.Error() != "commands are not implemented yet" {
			t.Fatalf("Run() error = %v, want command-not-implemented error", err)
		}
	})

	t.Run("uses no config when none is found", func(t *testing.T) {
		err := runFromDirectory(t, t.TempDir(), []string{"run"})
		if err == nil || err.Error() != "commands are not implemented yet" {
			t.Fatalf("Run() error = %v, want command-not-implemented error", err)
		}
	})

	t.Run("does not fall back after selected config fails", func(t *testing.T) {
		parent := t.TempDir()
		writeConfig(t, filepath.Join(parent, ".tfstylus.hcl"), "")
		child := filepath.Join(parent, "child")
		if err := os.Mkdir(child, 0o700); err != nil {
			t.Fatal(err)
		}
		writeConfig(t, filepath.Join(child, ".tfstylus.hcl"), "invalid {")

		err := runFromDirectory(t, child, []string{"check"})
		if err == nil || !strings.Contains(err.Error(), "parse config") {
			t.Fatalf("Run() error = %v, want selected config parse error", err)
		}
	})

	t.Run("explicit config takes precedence", func(t *testing.T) {
		workingDirectory := t.TempDir()
		writeConfig(t, filepath.Join(workingDirectory, ".tfstylus.hcl"), "invalid {")
		explicitConfig := filepath.Join(t.TempDir(), "explicit.hcl")
		writeConfig(t, explicitConfig, "")

		err := runFromDirectory(t, workingDirectory, []string{"check", "--config", explicitConfig})
		if err == nil || err.Error() != "commands are not implemented yet" {
			t.Fatalf("Run() error = %v, want command-not-implemented error", err)
		}
	})
}

func TestExplicitConfigMissingFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.hcl")
	var output bytes.Buffer

	err := Run([]string{"check", "-c", configPath}, &output)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Run() error = %v, want an error matching os.ErrNotExist", err)
	}
}

func TestExplicitConfigFlagPositions(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "empty.hcl")
	if err := os.WriteFile(configPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{name: "short before command", args: []string{"-c", configPath, "check"}},
		{name: "short after command", args: []string{"check", "-c", configPath}},
		{name: "long before command", args: []string{"--config", configPath, "run"}},
		{name: "long after command", args: []string{"run", "--config", configPath}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer

			err := Run(test.args, &output)
			if err == nil || err.Error() != "commands are not implemented yet" {
				t.Fatalf("Run() error = %v, want command-not-implemented error", err)
			}
		})
	}
}

func TestExplicitConfigArgumentErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing value", args: []string{"check", "-c"}},
		{name: "repeated flag", args: []string{"check", "-c", "first.hcl", "--config", "second.hcl"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := Run(test.args, &bytes.Buffer{}); err == nil {
				t.Fatal("Run() error = nil, want argument error")
			}
		})
	}
}

func TestExplicitConfigRejectsInvalidHCL(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "invalid.hcl")
	if err := os.WriteFile(configPath, []byte("invalid {"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Run([]string{"check", "--config", configPath}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("Run() error = %v, want HCL parse error", err)
	}
}

func runFromDirectory(t *testing.T, directory string, args []string) error {
	t.Helper()
	t.Setenv("HOME", directory)
	t.Setenv("USERPROFILE", directory)
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	return Run(args, &bytes.Buffer{})
}

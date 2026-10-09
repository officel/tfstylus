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
		{name: "help", args: []string{"--help"}, want: "Usage: tfstylus <command> [flags]\n\nFlags:\n  -c, --config PATH  Load an explicit configuration file\n  --chdir DIR        Set the target directory (default: current directory)\n  --recursive        Search directories recursively\n"},
		{name: "short help", args: []string{"-h"}, want: "Usage: tfstylus <command> [flags]\n\nFlags:\n  -c, --config PATH  Load an explicit configuration file\n  --chdir DIR        Set the target directory (default: current directory)\n  --recursive        Search directories recursively\n"},
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

func TestRunCollectsTargetSyntaxErrorsWithoutModifyingFiles(t *testing.T) {
	directory := t.TempDir()
	files := map[string]string{
		"main.tf":           "locals {\n  value =\n}\n",
		"variables.tf.json": "{\n  \"unknown\": }\n",
	}
	for name, content := range files {
		writeConfig(t, filepath.Join(directory, name), content)
	}

	err := runFromDirectory(t, directory, []string{"check"})
	if err == nil {
		t.Fatal("Run() error = nil, want target syntax errors")
	}
	for name := range files {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("Run() error = %q, want it to contain %q", err, name)
		}
		content, readErr := os.ReadFile(filepath.Join(directory, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(content) != files[name] {
			t.Errorf("Run() modified %s: got %q, want %q", name, content, files[name])
		}
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

func TestParseArgs(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want options
	}{
		{"defaults", []string{"check"}, options{command: "check", directory: "."}},
		{"before and after", []string{"--chdir", "child", "run", "--recursive", "-c", "settings.hcl"}, options{command: "run", directory: "child", recursive: true, configPath: "settings.hcl"}},
		{"equals", []string{"check", "--recursive=true", "--chdir=child"}, options{command: "check", directory: "child", recursive: true}},
		{"false", []string{"--recursive=false", "check"}, options{command: "check", directory: "."}},
		{"repeated recursive", []string{"check", "--recursive", "--recursive"}, options{command: "check", directory: ".", recursive: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseArgs(test.args)
			if err != nil || got != test.want {
				t.Fatalf("parseArgs() = %#v, %v, want %#v", got, err, test.want)
			}
		})
	}
	for _, args := range [][]string{
		{"check", "--chdir"}, {"--chdir=", "check"}, {"check", "--chdir", ""},
		{"check", "--chdir", "--recursive"}, {"--chdir=a", "check", "--chdir=b"},
		{"check", "--recursive=invalid"}, {"check", "--recursive="},
		{"check", "--unknown"}, {"check", "child"}, {"check", "run"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, err := parseArgs(args); err == nil {
				t.Fatal("parseArgs() error = nil, want argument error")
			}
		})
	}
}

func TestRunTargetDiscovery(t *testing.T) {
	for _, args := range [][]string{
		{"check", "--chdir", "child"},
		{"--chdir=child", "run"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, filepath.Join(root, ".tfstylus.hcl"), "invalid {")
			writeConfig(t, filepath.Join(root, "child", ".tfstylus.hcl"), "")
			if err := runFromDirectory(t, root, args); err == nil || err.Error() != "commands are not implemented yet" {
				t.Fatalf("Run() error = %v, want command-not-implemented error", err)
			}
			if directory, err := os.Getwd(); err != nil || directory != root {
				t.Fatalf("working directory = %q, %v, want %q", directory, err, root)
			}
		})
	}
	for _, test := range []struct {
		name    string
		args    []string
		configs map[string]string
		wantErr string
	}{
		{"recursive child config", []string{"check", "--recursive"}, map[string]string{"child/.tfstylus.hcl": "invalid {"}, "parse config"},
		{"direct skips child", []string{"check"}, map[string]string{"child/.tfstylus.hcl": "invalid {"}, "commands are not implemented yet"},
		{"false skips child", []string{"run", "--recursive=false"}, map[string]string{"child/.tfstylus.hcl": "invalid {"}, "commands are not implemented yet"},
		{"both flags", []string{"--recursive", "run", "--chdir=child"}, map[string]string{"child/grandchild/.tfstylus.hcl": "invalid {"}, "parse config"},
		{"chdir inherits parent", []string{"check", "--chdir=child"}, map[string]string{".tfstylus.hcl": "invalid {"}, "parse config"},
		{"relative explicit after chdir", []string{"check", "--chdir=child", "-c", "explicit.hcl"}, map[string]string{"explicit.hcl": "invalid {", "child/explicit.hcl": ""}, "commands are not implemented yet"},
		{"recursive relative explicit", []string{"check", "--recursive", "--config", "explicit.hcl"}, map[string]string{"explicit.hcl": ""}, "read config"},
		{"recursive absolute explicit", []string{"check", "--recursive", "--config", "{root}/explicit.hcl"}, map[string]string{"explicit.hcl": "", "child/.tfstylus.hcl": "invalid {"}, "commands are not implemented yet"},
		{"hidden child ignored", []string{"check", "--recursive"}, map[string]string{".terraform/.tfstylus.hcl": "invalid {", ".git/.tfstylus.hcl": "invalid {"}, "commands are not implemented yet"},
		{"missing directory", []string{"check", "--chdir=missing"}, nil, "inspect target directory"},
		{"file as directory", []string{"run", "--recursive", "--chdir=main.tf"}, nil, "not a directory"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, filepath.Join(root, ".tfstylus.hcl"), "")
			writeConfig(t, filepath.Join(root, "main.tf"), "locals { value = 1 }\n")
			writeConfig(t, filepath.Join(root, "child", "main.tf"), "")
			for path, source := range test.configs {
				writeConfig(t, filepath.Join(root, path), source)
			}
			args := make([]string, len(test.args))
			for index, argument := range test.args {
				args[index] = strings.ReplaceAll(argument, "{root}", root)
			}
			err := runFromDirectory(t, root, args)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Run() error = %v, want %q", err, test.wantErr)
			}
			if directory, err := os.Getwd(); err != nil || directory != root {
				t.Fatalf("working directory = %q, %v, want %q", directory, err, root)
			}
		})
	}
	t.Run("validates empty child config", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, filepath.Join(root, ".tfstylus.hcl"), "")
		writeConfig(t, filepath.Join(root, "empty", ".tfstylus.hcl"), "invalid {")
		err := runFromDirectory(t, root, []string{"check", "--recursive"})
		if err == nil || !strings.Contains(err.Error(), "parse config") {
			t.Fatalf("Run() error = %v, want empty child config error", err)
		}
	})
	t.Run("validates candidate links", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, filepath.Join(root, ".tfstylus.hcl"), "")
		if err := os.Symlink("missing", filepath.Join(root, "broken.tf")); err != nil {
			t.Fatal(err)
		}
		err := runFromDirectory(t, root, []string{"run"})
		if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "broken.tf") {
			t.Fatalf("Run() error = %v, want broken candidate error", err)
		}
	})
}

func TestHelpAndVersionDoNotDiscoverTargets(t *testing.T) {
	for _, command := range []string{"--help", "--version"} {
		if err := Run([]string{"--chdir=missing", "--recursive", "--config", "missing.hcl", command}, &bytes.Buffer{}); err != nil {
			t.Fatalf("Run(%s) error = %v, want no filesystem checks", command, err)
		}
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

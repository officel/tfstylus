package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{name: "empty file"},
		{name: "whitespace", content: "\n  \t\n"},
		{name: "invalid syntax", content: "invalid {", wantErr: "parse config"},
		{name: "unknown attribute", content: "unknown = true", wantErr: "validate config"},
		{name: "unknown block", content: "unknown {}", wantErr: "validate config"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(configPath, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}

			_, err := loadConfig(configPath)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("loadConfig() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("loadConfig() error = %v, want error containing %q", err, test.wantErr)
			}
		})
	}
}

func TestLoadConfigFileErrors(t *testing.T) {
	directory := t.TempDir()
	missingPath := filepath.Join(directory, "missing.hcl")

	if _, err := loadConfig(missingPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("loadConfig() error = %v, want an error matching os.ErrNotExist", err)
	}
	if _, err := loadConfig(directory); err == nil {
		t.Error("loadConfig() error = nil for a directory, want read error")
	}
}

func TestDiscoverConfig(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, root string) string
		wantPath    string
		wantErr     string
		wantLoadErr bool
	}{
		{
			name: "root config takes priority over config directory",
			setup: func(t *testing.T, root string) string {
				writeConfig(t, filepath.Join(root, ".tfstylus.hcl"), "invalid {")
				writeConfig(t, filepath.Join(root, ".config", ".tfstylus.hcl"), "")
				return root
			},
			wantPath:    ".tfstylus.hcl",
			wantLoadErr: true,
		},
		{
			name: "config directory is next candidate",
			setup: func(t *testing.T, root string) string {
				writeConfig(t, filepath.Join(root, ".config", ".tfstylus.hcl"), "")
				return root
			},
			wantPath: filepath.Join(".config", ".tfstylus.hcl"),
		},
		{
			name: "child config takes priority over parent",
			setup: func(t *testing.T, root string) string {
				writeConfig(t, filepath.Join(root, ".tfstylus.hcl"), "")
				child := filepath.Join(root, "child")
				if err := os.Mkdir(child, 0o700); err != nil {
					t.Fatal(err)
				}
				writeConfig(t, filepath.Join(child, ".tfstylus.hcl"), "invalid {")
				return child
			},
			wantPath:    filepath.Join("child", ".tfstylus.hcl"),
			wantLoadErr: true,
		},
		{
			name: "searches parent config",
			setup: func(t *testing.T, root string) string {
				child := filepath.Join(root, "child")
				if err := os.Mkdir(child, 0o700); err != nil {
					t.Fatal(err)
				}
				writeConfig(t, filepath.Join(root, ".tfstylus.hcl"), "")
				return child
			},
			wantPath: ".tfstylus.hcl",
		},
		{
			name: "no config found",
			setup: func(t *testing.T, root string) string {
				child := filepath.Join(root, "child")
				if err := os.Mkdir(child, 0o700); err != nil {
					t.Fatal(err)
				}
				return child
			},
		},
		{
			name: "searches user home after filesystem root",
			setup: func(t *testing.T, root string) string {
				workingDirectory := filepath.Join(root, "working", "nested")
				if err := os.MkdirAll(workingDirectory, 0o700); err != nil {
					t.Fatal(err)
				}
				writeConfig(t, filepath.Join(root, "home", ".tfstylus.hcl"), "")
				return workingDirectory
			},
			wantPath: filepath.Join("home", ".tfstylus.hcl"),
		},
		{
			name: "directory candidate is an error",
			setup: func(t *testing.T, root string) string {
				if err := os.Mkdir(filepath.Join(root, ".tfstylus.hcl"), 0o700); err != nil {
					t.Fatal(err)
				}
				return root
			},
			wantErr: "not a regular file",
		},
		{
			name: "invalid first config does not fall back",
			setup: func(t *testing.T, root string) string {
				writeConfig(t, filepath.Join(root, ".tfstylus.hcl"), "invalid {")
				writeConfig(t, filepath.Join(filepath.Dir(root), ".tfstylus.hcl"), "")
				return root
			},
			wantPath:    ".tfstylus.hcl",
			wantLoadErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", filepath.Join(root, "home"))
			t.Setenv("USERPROFILE", filepath.Join(root, "home"))
			startDir := test.setup(t, root)

			gotPath, err := discoverConfig(startDir)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("discoverConfig() error = %v, want error containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("discoverConfig() error = %v, want nil", err)
			}
			if test.wantPath == "" {
				if gotPath != "" {
					t.Fatalf("discoverConfig() path = %q, want empty", gotPath)
				}
				return
			}

			wantPath := filepath.Join(root, test.wantPath)
			if gotPath != wantPath {
				t.Fatalf("discoverConfig() path = %q, want %q", gotPath, wantPath)
			}
			if _, err := loadConfig(gotPath); test.wantLoadErr {
				if err == nil || !strings.Contains(err.Error(), "parse config") {
					t.Fatalf("loadConfig() error = %v, want parse error", err)
				}
			} else if err != nil {
				t.Fatalf("loadConfig() error = %v, want nil", err)
			}
		})
	}
}

func TestDiscoverConfigFollowsSymlink(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", directory)
	t.Setenv("USERPROFILE", directory)
	target := filepath.Join(directory, "target.hcl")
	configPath := filepath.Join(directory, ".tfstylus.hcl")
	writeConfig(t, target, "")
	if err := os.Symlink(target, configPath); err != nil {
		t.Fatal(err)
	}

	gotPath, err := discoverConfig(directory)
	if err != nil {
		t.Fatalf("discoverConfig() error = %v, want nil", err)
	}
	if gotPath != configPath {
		t.Fatalf("discoverConfig() path = %q, want %q", gotPath, configPath)
	}
	if _, err := loadConfig(gotPath); err != nil {
		t.Fatalf("loadConfig() error = %v, want nil", err)
	}
}

func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

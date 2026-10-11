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

func TestLoadConfigVariableOrder(t *testing.T) {
	for _, value := range []string{"", "true", "false", "null", "0", "\"false\"", "var.enabled"} {
		t.Run(value, func(t *testing.T) {
			content := ""
			if value != "" {
				content = "variable_order = " + value
			}
			path := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := loadConfig(path)
			valid := value == "" || value == "true" || value == "false"
			if valid {
				if err != nil || config.VariableOrder != (value != "false") {
					t.Fatalf("config = %+v; error = %v", config, err)
				}
			} else if err == nil {
				t.Fatal("invalid variable_order accepted")
			}
		})
	}
}

func TestLoadConfigOutputOrder(t *testing.T) {
	for _, value := range []string{"", "true", "false", "null", "0", `"false"`, "var.enabled"} {
		t.Run(value, func(t *testing.T) {
			content := ""
			if value != "" {
				content = "output_order = " + value
			}
			path := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := loadConfig(path)
			valid := value == "" || value == "true" || value == "false"
			if valid {
				if err != nil || config.OutputOrder != (value != "false") {
					t.Fatalf("config = %+v; error = %v", config, err)
				}
			} else if err == nil {
				t.Fatal("invalid output_order accepted")
			}
		})
	}
}

func TestLoadConfigRuleToggle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "required_version_order = true", want: true},
		{name: "disabled", content: "required_version_order = false", want: false},
		{name: "string is invalid", content: `required_version_order = "false"`, wantErr: true},
		{name: "number is invalid", content: "required_version_order = 0", wantErr: true},
		{name: "expression is invalid", content: "required_version_order = var.enabled", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(configPath, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}

			config, err := loadConfig(configPath)
			if test.wantErr {
				if err == nil {
					t.Fatal("loadConfig() error = nil, want invalid rule setting error")
				}
				return
			}
			if err != nil {
				t.Fatalf("loadConfig() error = %v, want nil", err)
			}
			if config.RequiredVersionOrder != test.want {
				t.Errorf("loadConfig() RequiredVersionOrder = %t, want %t", config.RequiredVersionOrder, test.want)
			}
		})
	}
}

func TestLoadConfigAcceptsRequiredProvidersOrder(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "required_providers_order = true", want: true},
		{name: "disabled", content: "required_providers_order = false", want: false},
		{name: "non-boolean is rejected", content: `required_providers_order = "false"`, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(configPath, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}

			config, err := loadConfig(configPath)
			if test.wantErr {
				if err == nil {
					t.Fatal("loadConfig() error = nil, want invalid rule setting error")
				}
				return
			}
			if err != nil {
				t.Fatalf("loadConfig() error = %v, want nil", err)
			}
			if config.RequiredProvidersOrder != test.want {
				t.Errorf("loadConfig() RequiredProvidersOrder = %t, want %t", config.RequiredProvidersOrder, test.want)
			}
		})
	}
}

func TestLoadConfigCloudOrderToggle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "cloud_order = true", want: true},
		{name: "disabled", content: "cloud_order = false", want: false},
		{name: "non-boolean is rejected", content: `cloud_order = "false"`, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(configPath, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}

			config, err := loadConfig(configPath)
			if test.wantErr {
				if err == nil {
					t.Fatal("loadConfig() error = nil, want invalid cloud_order error")
				}
				return
			}
			if err != nil {
				t.Fatalf("loadConfig() error = %v, want nil", err)
			}
			if config.CloudOrder != test.want {
				t.Errorf("loadConfig() CloudOrder = %t, want %t", config.CloudOrder, test.want)
			}
		})
	}
}

func TestLoadConfigCloudWorkspacesOrderToggle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "cloud_workspaces_order = true", want: true},
		{name: "disabled", content: "cloud_workspaces_order = false", want: false},
		{name: "non-boolean is rejected", content: `cloud_workspaces_order = "false"`, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(configPath, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}

			config, err := loadConfig(configPath)
			if test.wantErr {
				if err == nil {
					t.Fatal("loadConfig() error = nil, want invalid cloud_workspaces_order error")
				}
				return
			}
			if err != nil {
				t.Fatalf("loadConfig() error = %v, want nil", err)
			}
			if config.CloudWorkspacesOrder != test.want {
				t.Errorf("loadConfig() CloudWorkspacesOrder = %t, want %t", config.CloudWorkspacesOrder, test.want)
			}
		})
	}
}

func TestDiscoveredConfigRuleToggleMatchesExplicitConfig(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	configPath := filepath.Join(directory, ".tfstylus.hcl")
	writeConfig(t, configPath, "required_version_order = false")

	explicitConfig, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("loadConfig() error = %v, want nil", err)
	}

	discoveredPath, err := discoverConfig(directory)
	if err != nil {
		t.Fatalf("discoverConfig() error = %v, want nil", err)
	}
	if discoveredPath != configPath {
		t.Fatalf("discoverConfig() path = %q, want %q", discoveredPath, configPath)
	}
	discoveredConfig, err := loadConfig(discoveredPath)
	if err != nil {
		t.Fatalf("loadConfig(discovered path) error = %v, want nil", err)
	}
	if discoveredConfig.RequiredVersionOrder != explicitConfig.RequiredVersionOrder {
		t.Errorf("discovered RequiredVersionOrder = %t, explicit = %t", discoveredConfig.RequiredVersionOrder, explicitConfig.RequiredVersionOrder)
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

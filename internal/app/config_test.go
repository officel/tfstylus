package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestDiscoveredConfigRuleToggleMatchesExplicitConfig(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	configPath := filepath.Join(directory, ".tfstylus.hcl")
	writeConfig(t, configPath, "order_required_version = false")

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
	if discoveredConfig.OrderRequiredVersion != explicitConfig.OrderRequiredVersion {
		t.Errorf("discovered OrderRequiredVersion = %t, explicit = %t", discoveredConfig.OrderRequiredVersion, explicitConfig.OrderRequiredVersion)
	}
}

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

func TestLoadConfigAcceptsOrderProvider(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "order_provider = true", want: true},
		{name: "disabled", content: "order_provider = false", want: false},
		{name: "non-boolean is rejected", content: `order_provider = "false"`, wantErr: true},
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
			if config.OrderProvider != test.want {
				t.Errorf("loadConfig() OrderProvider = %t, want %t", config.OrderProvider, test.want)
			}
		})
	}
}

func TestLoadConfigAcceptsOrderRequiredProviders(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "order_required_providers = true", want: true},
		{name: "disabled", content: "order_required_providers = false", want: false},
		{name: "non-boolean is rejected", content: `order_required_providers = "false"`, wantErr: true},
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
			if config.OrderRequiredProviders != test.want {
				t.Errorf("loadConfig() OrderRequiredProviders = %t, want %t", config.OrderRequiredProviders, test.want)
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

func TestLoadConfigOrderAction(t *testing.T) {
	for _, value := range []string{"", "true", "false", "null", "0", `"false"`, "var.enabled"} {
		t.Run(value, func(t *testing.T) {
			content := ""
			if value != "" {
				content = "order_action = " + value
			}
			path := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := loadConfig(path)
			valid := value == "" || value == "true" || value == "false"
			if valid {
				if err != nil || config.OrderAction != (value != "false") {
					t.Fatalf("config = %+v; error = %v", config, err)
				}
			} else if err == nil {
				t.Fatal("invalid order_action accepted")
			}
		})
	}
}

func TestLoadConfigOrderCheck(t *testing.T) {
	for _, value := range []string{"", "true", "false", "null", "0", `"false"`, "var.enabled"} {
		t.Run(value, func(t *testing.T) {
			content := ""
			if value != "" {
				content = "order_check = " + value
			}
			path := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := loadConfig(path)
			valid := value == "" || value == "true" || value == "false"
			if valid {
				if err != nil || config.OrderCheck != (value != "false") {
					t.Fatalf("config = %+v; error = %v", config, err)
				}
			} else if err == nil {
				t.Fatal("invalid order_check accepted")
			}
		})
	}
}

func TestLoadConfigOrderCloudToggle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "order_cloud = true", want: true},
		{name: "disabled", content: "order_cloud = false", want: false},
		{name: "non-boolean is rejected", content: `order_cloud = "false"`, wantErr: true},
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
					t.Fatal("loadConfig() error = nil, want invalid order_cloud error")
				}
				return
			}
			if err != nil {
				t.Fatalf("loadConfig() error = %v, want nil", err)
			}
			if config.OrderCloud != test.want {
				t.Errorf("loadConfig() OrderCloud = %t, want %t", config.OrderCloud, test.want)
			}
		})
	}
}

func TestLoadConfigOrderCloudWorkspacesToggle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "order_cloud_workspaces = true", want: true},
		{name: "disabled", content: "order_cloud_workspaces = false", want: false},
		{name: "non-boolean is rejected", content: `order_cloud_workspaces = "false"`, wantErr: true},
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
					t.Fatal("loadConfig() error = nil, want invalid order_cloud_workspaces error")
				}
				return
			}
			if err != nil {
				t.Fatalf("loadConfig() error = %v, want nil", err)
			}
			if config.OrderCloudWorkspaces != test.want {
				t.Errorf("loadConfig() OrderCloudWorkspaces = %t, want %t", config.OrderCloudWorkspaces, test.want)
			}
		})
	}
}

func TestLoadConfigOrderData(t *testing.T) {
	for _, value := range []string{"", "true", "false", "null", "0", `"false"`, "var.enabled"} {
		t.Run(value, func(t *testing.T) {
			content := ""
			if value != "" {
				content = "order_data = " + value
			}
			path := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := loadConfig(path)
			valid := value == "" || value == "true" || value == "false"
			if valid {
				if err != nil || config.OrderData != (value != "false") {
					t.Fatalf("config = %+v; error = %v", config, err)
				}
			} else if err == nil {
				t.Fatal("invalid order_data accepted")
			}
		})
	}
}

func TestLoadConfigOrderEphemeral(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "order_ephemeral = true", want: true},
		{name: "disabled", content: "order_ephemeral = false", want: false},
		{name: "non-boolean is rejected", content: `order_ephemeral = "false"`, wantErr: true},
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
			if config.OrderEphemeral != test.want {
				t.Errorf("loadConfig() OrderEphemeral = %t, want %t", config.OrderEphemeral, test.want)
			}
		})
	}
}

func TestLoadConfigOrderImport(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "order_import = true", want: true},
		{name: "disabled", content: "order_import = false", want: false},
		{name: "non-boolean is rejected", content: `order_import = "false"`, wantErr: true},
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
			if config.OrderImport != test.want {
				t.Errorf("loadConfig() OrderImport = %t, want %t", config.OrderImport, test.want)
			}
		})
	}
}

func TestLoadConfigOrderList(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "order_list = true", want: true},
		{name: "disabled", content: "order_list = false"},
		{name: "string rejected", content: `order_list = "false"`, wantErr: true},
		{name: "null rejected", content: "order_list = null", wantErr: true},
		{name: "number rejected", content: "order_list = 1", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "config.hcl")
			writeConfig(t, filename, test.content)
			config, err := loadConfig(filename)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected invalid rule setting error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if config.OrderList != test.want {
				t.Fatalf("OrderList = %t, want %t", config.OrderList, test.want)
			}
		})
	}
}

func TestLoadConfigOrderModule(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted", want: true},
		{name: "enabled", content: "order_module = true", want: true},
		{name: "disabled", content: "order_module = false"},
		{name: "string rejected", content: `order_module = "false"`, wantErr: true},
		{name: "null rejected", content: "order_module = null", wantErr: true},
		{name: "number rejected", content: "order_module = 1", wantErr: true},
		{name: "reference rejected", content: "order_module = var.enabled", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "config.hcl")
			writeConfig(t, filename, test.content)
			config, err := loadConfig(filename)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected invalid rule setting error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if config.OrderModule != test.want {
				t.Fatalf("OrderModule = %t, want %t", config.OrderModule, test.want)
			}
		})
	}
}

func TestLoadConfigOrderMoved(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "order_moved = true", want: true},
		{name: "disabled", content: "order_moved = false"},
		{name: "non-boolean", content: `order_moved = "false"`, wantErr: true},
		{name: "null", content: "order_moved = null", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.hcl")
			writeConfig(t, path, test.content)
			config, err := loadConfig(path)
			if test.wantErr {
				if err == nil {
					t.Fatal("loadConfig() accepted an invalid setting")
				}
				return
			}
			if err != nil || config.OrderMoved != test.want {
				t.Fatalf("loadConfig() OrderMoved = %t; error = %v, want %t", config.OrderMoved, err, test.want)
			}
		})
	}
}

func TestLoadConfigOrderOutput(t *testing.T) {
	for _, value := range []string{"", "true", "false", "null", "0", `"false"`, "var.enabled"} {
		t.Run(value, func(t *testing.T) {
			content := ""
			if value != "" {
				content = "order_output = " + value
			}
			path := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := loadConfig(path)
			valid := value == "" || value == "true" || value == "false"
			if valid {
				if err != nil || config.OrderOutput != (value != "false") {
					t.Fatalf("config = %+v; error = %v", config, err)
				}
			} else if err == nil {
				t.Fatal("invalid order_output accepted")
			}
		})
	}
}

func TestLoadConfigOrderRemoved(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "order_removed = true", want: true},
		{name: "disabled", content: "order_removed = false"},
		{name: "non-boolean", content: `order_removed = "false"`, wantErr: true},
		{name: "null", content: "order_removed = null", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.hcl")
			writeConfig(t, path, test.content)
			config, err := loadConfig(path)
			if test.wantErr {
				if err == nil {
					t.Fatal("loadConfig() accepted an invalid setting")
				}
				return
			}
			if err != nil || config.OrderRemoved != test.want {
				t.Fatalf("loadConfig() OrderRemoved = %t; error = %v, want %t", config.OrderRemoved, err, test.want)
			}
		})
	}
}

func TestLoadConfigOrderResource(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "order_resource = true", want: true},
		{name: "disabled", content: "order_resource = false", want: false},
		{name: "non-boolean is rejected", content: `order_resource = "false"`, wantErr: true},
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
			if config.OrderResource != test.want {
				t.Errorf("loadConfig() OrderResource = %t, want %t", config.OrderResource, test.want)
			}
		})
	}
}

func TestLoadConfigOrderVariable(t *testing.T) {
	for _, value := range []string{"", "true", "false", "null", "0", "\"false\"", "var.enabled"} {
		t.Run(value, func(t *testing.T) {
			content := ""
			if value != "" {
				content = "order_variable = " + value
			}
			path := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := loadConfig(path)
			valid := value == "" || value == "true" || value == "false"
			if valid {
				if err != nil || config.OrderVariable != (value != "false") {
					t.Fatalf("config = %+v; error = %v", config, err)
				}
			} else if err == nil {
				t.Fatal("invalid order_variable accepted")
			}
		})
	}
}

func TestLoadConfigRenameLocalsFile(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "rename_locals_file = true", want: true},
		{name: "disabled", content: "rename_locals_file = false"},
		{name: "non-boolean", content: `rename_locals_file = "false"`, wantErr: true},
		{name: "null", content: "rename_locals_file = null", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.hcl")
			writeConfig(t, path, test.content)
			config, err := loadConfig(path)
			if test.wantErr {
				if err == nil {
					t.Fatal("loadConfig() accepted an invalid setting")
				}
				return
			}
			if err != nil || config.RenameLocalsFile != test.want {
				t.Fatalf("loadConfig() RenameLocalsFile = %t; error = %v, want %t", config.RenameLocalsFile, err, test.want)
			}
		})
	}
}

func TestLoadConfigRenameOutputsFile(t *testing.T) {
	testLoadConfigNamedFileRename(t, "outputs")
}

func TestLoadConfigRenameTerraformFile(t *testing.T) {
	testLoadConfigNamedFileRename(t, "terraform")
}

func TestLoadConfigRenameVariablesFile(t *testing.T) {
	testLoadConfigNamedFileRename(t, "variables")
}

func TestLoadConfigReplaceRequiredVersion(t *testing.T) {
	tests := []struct {
		content string
		want    string
		wantErr bool
	}{
		{},
		{content: `replace_required_version = false`},
		{content: `replace_required_version = ">= 1.7.0"`, want: ">= 1.7.0"},
		{content: `replace_required_version = "~> 1.7"`, want: "~> 1.7"},
		{content: `replace_required_version = ">= 1.7.0, < 2.0.0, != 1.8.0"`, want: ">= 1.7.0, < 2.0.0, != 1.8.0"},
		{content: `replace_required_version = "1.7.0"`, want: "1.7.0"},
		{content: `replace_required_version = "= 1.7.0-beta1"`, want: "= 1.7.0-beta1"},
		{content: `replace_required_version = " >= 1.7.0 "`, want: " >= 1.7.0 "},
		{content: `replace_required_version = ""`, wantErr: true},
		{content: `replace_required_version = " "`, wantErr: true},
		{content: `replace_required_version = true`, wantErr: true},
		{content: `replace_required_version = null`, wantErr: true},
		{content: `replace_required_version = 1.7`, wantErr: true},
		{content: `replace_required_version = "false"`, wantErr: true},
		{content: `replace_required_version = "^1.7.0"`, wantErr: true},
		{content: `replace_required_version = ">= 1.7.0 || < 1.0"`, wantErr: true},
		{content: `replace_required_version = var.version`, wantErr: true},
		{content: `replace_required_version = "${1.7}"`, wantErr: true},
		{content: "replace_required_version = <<EOT\n>= 1.7.0\nEOT\n", wantErr: true},
		{content: `replace_required_version = (">= 1.7.0")`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.content, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.hcl")
			writeConfig(t, path, test.content)
			config, err := loadConfig(path)
			if test.wantErr {
				if err == nil {
					t.Fatal("invalid replacement setting accepted")
				}
				return
			}
			if err != nil || config.ReplaceRequiredVersion != test.want {
				t.Fatalf("replacement = %q, error = %v; want %q", config.ReplaceRequiredVersion, err, test.want)
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
		{name: "enabled", content: "order_required_version = true", want: true},
		{name: "disabled", content: "order_required_version = false", want: false},
		{name: "string is invalid", content: `order_required_version = "false"`, wantErr: true},
		{name: "number is invalid", content: "order_required_version = 0", wantErr: true},
		{name: "expression is invalid", content: "order_required_version = var.enabled", wantErr: true},
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
			if config.OrderRequiredVersion != test.want {
				t.Errorf("loadConfig() OrderRequiredVersion = %t, want %t", config.OrderRequiredVersion, test.want)
			}
		})
	}
}

func testLoadConfigNamedFileRename(t *testing.T, kind string) {
	t.Helper()
	key := "rename_" + kind + "_file"
	for _, test := range []struct {
		name, content string
		want, wantErr bool
	}{
		{name: "omitted", want: true},
		{name: "enabled", content: key + " = true", want: true},
		{name: "disabled", content: key + " = false"},
		{name: "non boolean", content: key + ` = "false"`, wantErr: true},
		{name: "null", content: key + " = null", wantErr: true},
		{name: "unknown reference", content: key + " = unknown", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.hcl")
			writeConfig(t, path, test.content)
			config, err := loadConfig(path)
			if test.wantErr {
				if err == nil {
					t.Fatal("accepted invalid setting")
				}
				return
			}
			wantOutputs, wantVariables, wantTerraform := true, true, true
			switch kind {
			case "outputs":
				wantOutputs = test.want
			case "variables":
				wantVariables = test.want
			case "terraform":
				wantTerraform = test.want
			}
			if err != nil || config.RenameOutputsFile != wantOutputs || config.RenameVariablesFile != wantVariables || config.RenameTerraformFile != wantTerraform || !config.RenameLocalsFile || !config.OrderOutput || !config.OrderVariable {
				t.Fatalf("loadConfig() = %+v, %v, want %s rename %t and other rules enabled", config, err, kind, test.want)
			}
		})
	}
}

func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil { // #nosec G703 -- All callers use test-owned temporary directories and fixed fixture names.
		t.Fatal(err)
	}
}

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

func TestLoadConfigDataOrder(t *testing.T) {
	for _, value := range []string{"", "true", "false", "null", "0", `"false"`, "var.enabled"} {
		t.Run(value, func(t *testing.T) {
			content := ""
			if value != "" {
				content = "data_order = " + value
			}
			path := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := loadConfig(path)
			valid := value == "" || value == "true" || value == "false"
			if valid {
				if err != nil || config.DataOrder != (value != "false") {
					t.Fatalf("config = %+v; error = %v", config, err)
				}
			} else if err == nil {
				t.Fatal("invalid data_order accepted")
			}
		})
	}
}

func TestLoadConfigActionOrder(t *testing.T) {
	for _, value := range []string{"", "true", "false", "null", "0", `"false"`, "var.enabled"} {
		t.Run(value, func(t *testing.T) {
			content := ""
			if value != "" {
				content = "action_order = " + value
			}
			path := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := loadConfig(path)
			valid := value == "" || value == "true" || value == "false"
			if valid {
				if err != nil || config.ActionOrder != (value != "false") {
					t.Fatalf("config = %+v; error = %v", config, err)
				}
			} else if err == nil {
				t.Fatal("invalid action_order accepted")
			}
		})
	}
}

func TestLoadConfigCheckOrder(t *testing.T) {
	for _, value := range []string{"", "true", "false", "null", "0", `"false"`, "var.enabled"} {
		t.Run(value, func(t *testing.T) {
			content := ""
			if value != "" {
				content = "check_order = " + value
			}
			path := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := loadConfig(path)
			valid := value == "" || value == "true" || value == "false"
			if valid {
				if err != nil || config.CheckOrder != (value != "false") {
					t.Fatalf("config = %+v; error = %v", config, err)
				}
			} else if err == nil {
				t.Fatal("invalid check_order accepted")
			}
		})
	}
}

func TestLoadConfigEphemeralOrder(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "ephemeral_order = true", want: true},
		{name: "disabled", content: "ephemeral_order = false", want: false},
		{name: "non-boolean is rejected", content: `ephemeral_order = "false"`, wantErr: true},
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
			if config.EphemeralOrder != test.want {
				t.Errorf("loadConfig() EphemeralOrder = %t, want %t", config.EphemeralOrder, test.want)
			}
		})
	}
}

func TestLoadConfigResourceOrder(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "resource_order = true", want: true},
		{name: "disabled", content: "resource_order = false", want: false},
		{name: "non-boolean is rejected", content: `resource_order = "false"`, wantErr: true},
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
			if config.ResourceOrder != test.want {
				t.Errorf("loadConfig() ResourceOrder = %t, want %t", config.ResourceOrder, test.want)
			}
		})
	}
}

func TestLoadConfigModuleOrder(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted", want: true},
		{name: "enabled", content: "module_order = true", want: true},
		{name: "disabled", content: "module_order = false"},
		{name: "string rejected", content: `module_order = "false"`, wantErr: true},
		{name: "null rejected", content: "module_order = null", wantErr: true},
		{name: "number rejected", content: "module_order = 1", wantErr: true},
		{name: "reference rejected", content: "module_order = var.enabled", wantErr: true},
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
			if config.ModuleOrder != test.want {
				t.Fatalf("ModuleOrder = %t, want %t", config.ModuleOrder, test.want)
			}
		})
	}
}

func TestLoadConfigListOrder(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "list_order = true", want: true},
		{name: "disabled", content: "list_order = false"},
		{name: "string rejected", content: `list_order = "false"`, wantErr: true},
		{name: "null rejected", content: "list_order = null", wantErr: true},
		{name: "number rejected", content: "list_order = 1", wantErr: true},
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
			if config.ListOrder != test.want {
				t.Fatalf("ListOrder = %t, want %t", config.ListOrder, test.want)
			}
		})
	}
}

func TestLoadConfigImportOrder(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "import_order = true", want: true},
		{name: "disabled", content: "import_order = false", want: false},
		{name: "non-boolean is rejected", content: `import_order = "false"`, wantErr: true},
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
			if config.ImportOrder != test.want {
				t.Errorf("loadConfig() ImportOrder = %t, want %t", config.ImportOrder, test.want)
			}
		})
	}
}

func TestLoadConfigMovedOrder(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "moved_order = true", want: true},
		{name: "disabled", content: "moved_order = false"},
		{name: "non-boolean", content: `moved_order = "false"`, wantErr: true},
		{name: "null", content: "moved_order = null", wantErr: true},
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
			if err != nil || config.MovedOrder != test.want {
				t.Fatalf("loadConfig() MovedOrder = %t; error = %v, want %t", config.MovedOrder, err, test.want)
			}
		})
	}
}

func TestLoadConfigRemovedOrder(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "removed_order = true", want: true},
		{name: "disabled", content: "removed_order = false"},
		{name: "non-boolean", content: `removed_order = "false"`, wantErr: true},
		{name: "null", content: "removed_order = null", wantErr: true},
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
			if err != nil || config.RemovedOrder != test.want {
				t.Fatalf("loadConfig() RemovedOrder = %t; error = %v, want %t", config.RemovedOrder, err, test.want)
			}
		})
	}
}

func TestLoadConfigOutputsFileRename(t *testing.T) {
	testLoadConfigNamedFileRename(t, "outputs")
}

func TestLoadConfigVariablesFileRename(t *testing.T) {
	testLoadConfigNamedFileRename(t, "variables")
}

func TestLoadConfigTerraformFileRename(t *testing.T) {
	testLoadConfigNamedFileRename(t, "terraform")
}

func testLoadConfigNamedFileRename(t *testing.T, kind string) {
	t.Helper()
	key := kind + "_file_rename"
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
			if err != nil || config.OutputsFileRename != wantOutputs || config.VariablesFileRename != wantVariables || config.TerraformFileRename != wantTerraform || !config.LocalsFileRename || !config.OutputOrder || !config.VariableOrder {
				t.Fatalf("loadConfig() = %+v, %v, want %s rename %t and other rules enabled", config, err, kind, test.want)
			}
		})
	}
}

func TestLoadConfigLocalsFileRename(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "locals_file_rename = true", want: true},
		{name: "disabled", content: "locals_file_rename = false"},
		{name: "non-boolean", content: `locals_file_rename = "false"`, wantErr: true},
		{name: "null", content: "locals_file_rename = null", wantErr: true},
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
			if err != nil || config.LocalsFileRename != test.want {
				t.Fatalf("loadConfig() LocalsFileRename = %t; error = %v, want %t", config.LocalsFileRename, err, test.want)
			}
		})
	}
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

func TestLoadConfigAcceptsProviderOrder(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
		wantErr bool
	}{
		{name: "omitted defaults to enabled", want: true},
		{name: "enabled", content: "provider_order = true", want: true},
		{name: "disabled", content: "provider_order = false", want: false},
		{name: "non-boolean is rejected", content: `provider_order = "false"`, wantErr: true},
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
			if config.ProviderOrder != test.want {
				t.Errorf("loadConfig() ProviderOrder = %t, want %t", config.ProviderOrder, test.want)
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
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil { // #nosec G703 -- All callers use test-owned temporary directories and fixed fixture names.
		t.Fatal(err)
	}
}

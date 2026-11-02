package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

var configRuleRenames = []struct {
	oldName string
	name    string
	enabled func(Config) bool
}{
	{"action_order", "order_action", func(c Config) bool { return c.OrderAction }},
	{"backend_order", "order_backend", func(c Config) bool { return c.OrderBackend }},
	{"check_order", "order_check", func(c Config) bool { return c.OrderCheck }},
	{"cloud_order", "order_cloud", func(c Config) bool { return c.OrderCloud }},
	{"cloud_workspaces_order", "order_cloud_workspaces", func(c Config) bool { return c.OrderCloudWorkspaces }},
	{"data_order", "order_data", func(c Config) bool { return c.OrderData }},
	{"ephemeral_order", "order_ephemeral", func(c Config) bool { return c.OrderEphemeral }},
	{"import_order", "order_import", func(c Config) bool { return c.OrderImport }},
	{"list_order", "order_list", func(c Config) bool { return c.OrderList }},
	{"module_order", "order_module", func(c Config) bool { return c.OrderModule }},
	{"moved_order", "order_moved", func(c Config) bool { return c.OrderMoved }},
	{"output_order", "order_output", func(c Config) bool { return c.OrderOutput }},
	{"provider_order", "order_provider", func(c Config) bool { return c.OrderProvider }},
	{"removed_order", "order_removed", func(c Config) bool { return c.OrderRemoved }},
	{"required_providers_order", "order_required_providers", func(c Config) bool { return c.OrderRequiredProviders }},
	{"required_version_order", "order_required_version", func(c Config) bool { return c.OrderRequiredVersion }},
	{"resource_order", "order_resource", func(c Config) bool { return c.OrderResource }},
	{"variable_order", "order_variable", func(c Config) bool { return c.OrderVariable }},
	{"locals_file_rename", "rename_locals_file", func(c Config) bool { return c.RenameLocalsFile }},
	{"outputs_file_rename", "rename_outputs_file", func(c Config) bool { return c.RenameOutputsFile }},
	{"terraform_file_rename", "rename_terraform_file", func(c Config) bool { return c.RenameTerraformFile }},
	{"variables_file_rename", "rename_variables_file", func(c Config) bool { return c.RenameVariablesFile }},
}

func TestConfigPrefixCommandsRejectRetiredKeysWithoutWriting(t *testing.T) {
	for _, rule := range configRuleRenames {
		for _, command := range []string{"plan", "apply"} {
			for _, mode := range []string{"explicit", "discovered"} {
				t.Run(rule.oldName+"/"+command+"/"+mode, func(t *testing.T) {
					directory := t.TempDir()
					path := filepath.Join(directory, ".tfstylus.hcl")
					writeConfig(t, path, rule.oldName+" = false\n")
					source := "locals {\n  value = true\n}\n"
					filename := filepath.Join(directory, "local.tf")
					writeConfig(t, filename, source)
					args := []string{command, "--chdir", directory}
					if mode == "explicit" {
						args = append(args, "-c", path)
					}
					var output bytes.Buffer
					err := Run(args, &output)
					var exitError interface{ ExitCode() int }
					if !errors.As(err, &exitError) || exitError.ExitCode() != 2 || !strings.Contains(err.Error(), `"`+rule.oldName+`"`) {
						t.Fatalf("Run() = %v, want exit 2 naming %q", err, rule.oldName)
					}
					if output.Len() != 0 {
						t.Fatalf("unexpected success output: %q", &output)
					}
					if got, err := os.ReadFile(filename); err != nil || string(got) != source {
						t.Fatalf("source changed: %q; error = %v", got, err)
					}
					if _, err := os.Lstat(filepath.Join(directory, "locals.tf")); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("unexpected rename destination: %v", err)
					}
				})
			}
		}
	}
}

func TestConfigPrefixIgnoreDirectives(t *testing.T) {
	for _, rule := range configRuleRenames {
		for _, scope := range []string{"file", "block", "json"} {
			for _, names := range []string{rule.oldName, rule.name, rule.oldName + ", " + rule.name} {
				t.Run(scope+"/"+names, func(t *testing.T) {
					source := "# tfstylus-ignore-file: " + names + "\nlocals {}\n"
					if scope == "block" {
						source = "# tfstylus-ignore: " + names + "\nlocals {}\n"
					}
					parser := parseHCLSource
					if scope == "json" {
						source = `{"//": "tfstylus-ignore-file: ` + names + `", "locals": {}}`
						parser = parseJSONSource
					}
					parsed, diagnostics := parser("main.tf", []byte(source))
					if diagnostics.HasErrors() {
						t.Fatal(diagnostics.Error())
					}
					rules := parsed.ignores.file
					if scope == "block" {
						body, ok := parsed.body.(*hclsyntax.Body)
						if !ok {
							t.Fatal("expected HCL syntax")
						}
						rules = parsed.ignores.blocks[body.Blocks[0].Range().Start.Byte]
					}
					want := names != rule.oldName && (scope != "block" || !strings.HasPrefix(rule.name, "rename_"))
					_, got := rules[rule.name]
					if got != want {
						t.Errorf("%s ignored = %t, want %t", rule.name, got, want)
					}
					wantCount := 0
					if want {
						wantCount = 1
					}
					if len(rules) != wantCount {
						t.Errorf("ignored rules = %v; want %d new rules and no retired rules", rules, wantCount)
					}
				})
			}
		}
	}
}

func TestConfigPrefixInitAndDocumentationInventory(t *testing.T) {
	directory := t.TempDir()
	if err := Run([]string{"init", "--chdir", directory}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(directory, ".tfstylus.hcl"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, diagnostics := parseHCLSource(".tfstylus.hcl", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	body, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		t.Fatal("generated config is not HCL syntax")
	}
	if len(body.Attributes) != len(configRuleRenames)+1 || len(ignoreRuleNames) != len(body.Attributes) {
		t.Fatalf("config/ignore inventory differs: %d attributes, %d ignore rules", len(body.Attributes), len(ignoreRuleNames))
	}
	for _, rule := range configRuleRenames {
		if body.Attributes[rule.name] == nil || body.Attributes[rule.oldName] != nil {
			t.Errorf("generated config must contain only the new key %q, not %q", rule.name, rule.oldName)
		}
	}
	for name := range body.Attributes {
		if _, ok := ignoreRuleNames[name]; !ok {
			t.Errorf("generated setting %q is missing from ignore rules", name)
		}
	}
	config, err := loadConfig(filepath.Join(directory, ".tfstylus.hcl"))
	if err != nil || config != (Config{}) || body.Attributes["replace_required_version"] == nil {
		t.Fatalf("generated config = %+v, error = %v; want all 23 settings disabled", config, err)
	}
	documentation, err := os.ReadFile(filepath.Join("..", "..", "docs", "config.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(documentation, source) {
		t.Fatal("docs/config.md does not contain the exact generated config")
	}
	for _, rule := range configRuleRenames {
		if !bytes.Contains(documentation, []byte("| `"+rule.oldName+"` | `"+rule.name+"` |")) {
			t.Errorf("docs/config.md is missing the migration from %q to %q", rule.oldName, rule.name)
		}
	}
}

func TestConfigPrefixKeysToggleIndependently(t *testing.T) {
	for _, rule := range configRuleRenames {
		for _, value := range []string{"", "true", "false"} {
			t.Run(rule.name+"/"+value, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "config.hcl")
				content := ""
				if value != "" {
					content = rule.name + " = " + value
				}
				writeConfig(t, path, content)
				config, err := loadConfig(path)
				if err != nil {
					t.Fatal(err)
				}
				for _, other := range configRuleRenames {
					want := other.name != rule.name || value != "false"
					if other.enabled(config) != want {
						t.Errorf("%s = %t, want %t", other.name, other.enabled(config), want)
					}
				}
				if config.ReplaceRequiredVersion != "" {
					t.Fatalf("replacement unexpectedly enabled: %q", config.ReplaceRequiredVersion)
				}
			})
		}
	}
}

func TestConfigPrefixPlanApplyCombination(t *testing.T) {
	for _, test := range []struct {
		name    string
		rules   string
		ignored bool
	}{
		{name: "no ignore"},
		{name: "retired ignore", rules: "variable_order, variables_file_rename"},
		{name: "prefix ignore", rules: "order_variable, rename_variables_file, replace_required_version", ignored: true},
	} {
		for _, mode := range []string{"explicit", "discovered"} {
			t.Run(test.name+"/"+mode, func(t *testing.T) {
				directory := t.TempDir()
				configPath := filepath.Join(directory, ".tfstylus.hcl")
				writeConfig(t, configPath, "order_variable = true\nrename_variables_file = true\nreplace_required_version = \">= 1.7.0\"\n")
				source := "terraform {\n  required_version = \">= 1.0\"\n}\n\nvariable \"example\" {\n  default = 1\n  type = number\n}\n"
				if test.rules != "" {
					source = "# tfstylus-ignore-file: " + test.rules + "\n" + source
				}
				sourcePath := filepath.Join(directory, "variable.tf")
				writeConfig(t, sourcePath, source)
				args := []string{"--chdir", directory}
				if mode == "explicit" {
					args = append(args, "-c", configPath)
				}
				var output bytes.Buffer
				err := Run(append(args, "plan"), &output)
				if test.ignored {
					if err != nil || output.String() != "No changes.\n" {
						t.Fatalf("plan = %v; output = %q; want no changes", err, &output)
					}
				} else {
					var exitError interface{ ExitCode() int }
					if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
						t.Fatalf("plan = %v; want exit 1", err)
					}
					for _, marker := range []string{"rename to variables.tf", "[file-name/variables]", ": order_variable:", ": terraform/replace-required-version:"} {
						if !strings.Contains(output.String(), marker) {
							t.Errorf("plan output %q lacks %q", &output, marker)
						}
					}
				}
				if got, err := os.ReadFile(sourcePath); err != nil || string(got) != source {
					t.Fatalf("plan changed input: %q; error = %v", got, err)
				}
				destinationPath := filepath.Join(directory, "variables.tf")
				if _, err := os.Lstat(destinationPath); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("plan created destination: %v", err)
				}

				want, finalPath := source, sourcePath
				if !test.ignored {
					want = strings.ReplaceAll(want, `">= 1.0"`, `">= 1.7.0"`)
					want = strings.ReplaceAll(want, "  default = 1\n  type = number\n", "  type = number\n  default = 1\n")
					finalPath = destinationPath
				}
				for iteration := 0; iteration < 2; iteration++ {
					output.Reset()
					if err := Run(append(args, "apply"), &output); err != nil || output.Len() != 0 {
						t.Fatalf("apply %d = %v; output = %q", iteration, err, &output)
					}
					if got, err := os.ReadFile(finalPath); err != nil || string(got) != want {
						t.Fatalf("apply %d output = %q; error = %v; want %q", iteration, got, err, want)
					}
					absentPath := sourcePath
					if test.ignored {
						absentPath = destinationPath
					}
					if _, err := os.Lstat(absentPath); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("unexpected file %s after apply: %v", absentPath, err)
					}
					output.Reset()
					if err := Run(append(args, "plan"), &output); err != nil || output.String() != "No changes.\n" {
						t.Fatalf("plan after apply = %v; output = %q; want no changes", err, &output)
					}
				}
			})
		}
	}
}

func TestConfigPrefixRejectsRetiredKeys(t *testing.T) {
	for _, rule := range configRuleRenames {
		for _, mixed := range []bool{false, true} {
			content := rule.oldName + " = false\n"
			if mixed {
				content += rule.name + " = true\n"
			}
			t.Run(content, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "config.hcl")
				writeConfig(t, path, content)
				_, err := loadConfig(path)
				if err == nil || !strings.Contains(err.Error(), "Unsupported argument") || !strings.Contains(err.Error(), `"`+rule.oldName+`"`) {
					t.Fatalf("loadConfig() = %v, want unsupported %q", err, rule.oldName)
				}
			})
		}
	}
}

package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVariableOrderComposition(t *testing.T) {
	terraformSource := "terraform {\n  cloud {\n    token = \"dummy-token\"\n    workspaces {\n      project = \"project\"\n      name = \"example\"\n    }\n    organization = \"example\"\n  }\n  required_providers {\n    example = {\n      version = \"~> 1.0\"\n      source = \"example/example\"\n    }\n  }\n  required_version = \">= 1.0\"\n}\n"
	terraformWant := "terraform {\n  required_version = \">= 1.0\"\n  cloud {\n    organization = \"example\"\n    token = \"dummy-token\"\n    workspaces {\n      name = \"example\"\n      project = \"project\"\n    }\n  }\n  required_providers {\n    example = {\n      source = \"example/example\"\n      version = \"~> 1.0\"\n    }\n  }\n}\n"
	variableSource := "variable \"first\" {\n  default = [\n    \"\u65e5\u672c\u8a9e\", # keep UTF-8\n    \"second\",\n  ]\n  # type\n  type = list(string)\n}\n"
	variableWant := "variable \"first\" {\n  # type\n  type = list(string)\n  default = [\n    \"\u65e5\u672c\u8a9e\", # keep UTF-8\n    \"second\",\n  ]\n}\n"
	prefix := "# header\n"
	suffix := "locals { untouched = [2, 1] }\n# footer"
	for _, newline := range []string{"\n", "\r\n"} {
		for _, terraformEnabled := range []bool{false, true} {
			for _, variableEnabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("newline=%q/terraform=%t/variable=%t", newline, terraformEnabled, variableEnabled), func(t *testing.T) {
					source := prefix + variableSource + terraformSource + strings.Replace(variableSource, "first", "second", 1) + suffix
					selectedTerraform, selectedVariable := terraformSource, variableSource
					if terraformEnabled {
						selectedTerraform = terraformWant
					}
					if variableEnabled {
						selectedVariable = variableWant
					}
					want := prefix + selectedVariable + selectedTerraform + strings.Replace(selectedVariable, "first", "second", 1) + suffix
					source = strings.ReplaceAll(source, "\n", newline)
					want = strings.ReplaceAll(want, "\n", newline)
					parsed, diagnostics := parseHCLSource("main.tf", []byte(source))
					if diagnostics.HasErrors() {
						t.Fatal(diagnostics.Error())
					}
					config := Config{RequiredVersionOrder: terraformEnabled, BackendOrder: terraformEnabled, CloudOrder: terraformEnabled, CloudWorkspacesOrder: terraformEnabled, RequiredProvidersOrder: terraformEnabled, VariableOrder: variableEnabled}
					candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, config)
					wantChanges := 0
					if terraformEnabled {
						wantChanges += 4
					}
					if variableEnabled {
						wantChanges += 2
					}
					if string(candidate) != want || len(changes) != wantChanges || len(warnings) != 0 {
						t.Fatalf("candidate = %s; changes = %v; warnings = %v; want %s", candidate, changes, warnings, want)
					}
					for _, change := range changes {
						if change.Rule == "variable_order" {
							if !strings.HasPrefix(source[change.Subject.Start.Byte:], "variable ") || change.Subject.Start.Line != strings.Count(source[:change.Subject.Start.Byte], "\n")+1 {
								t.Fatalf("change is not at original variable position: %v", change)
							}
						}
					}
					directory := t.TempDir()
					filename := filepath.Join(directory, "main.tf")
					writeConfig(t, filename, source)
					writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), fmt.Sprintf("required_version_order = %t\nbackend_order = %t\ncloud_order = %t\ncloud_workspaces_order = %t\nrequired_providers_order = %t\nvariable_order = %t\n", terraformEnabled, terraformEnabled, terraformEnabled, terraformEnabled, terraformEnabled, variableEnabled))
					jsonFilename := filepath.Join(directory, "opaque.tf.json")
					jsonSource := `{"variable":{"json":{"default":1,"type":"number","future":true}}}`
					writeConfig(t, jsonFilename, jsonSource)
					for _, diff := range []bool{false, true} {
						args := []string{"check", "--chdir", directory}
						if diff {
							args = append(args, "--diff")
						}
						var output bytes.Buffer
						err := Run(args, &output)
						var exitError interface{ ExitCode() int }
						if wantChanges == 0 {
							if err != nil || output.Len() != 0 {
								t.Fatalf("disabled check = %v; %s", err, output.String())
							}
						} else if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
							t.Fatalf("check = %v; %s", err, output.String())
						}
						if diff && wantChanges > 0 && !strings.Contains(output.String(), "--- a/main.tf") {
							t.Fatalf("missing diff: %s", output.String())
						}
						if got, err := os.ReadFile(filename); err != nil || string(got) != source {
							t.Fatalf("check modified input: %s; %v", got, err)
						}
					}
					for _, command := range []string{"run", "run", "check"} {
						var output bytes.Buffer
						if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
							t.Fatalf("%s = %v; %s", command, err, output.String())
						}
						if got, err := os.ReadFile(filename); err != nil || !bytes.Equal(got, candidate) {
							t.Fatalf("%s disagrees with candidate: %s; %v", command, got, err)
						}
						if got, err := os.ReadFile(jsonFilename); err != nil || string(got) != jsonSource {
							t.Fatalf("JSON changed: %s; %v", got, err)
						}
					}
				})
			}
		}
	}
}

func TestVariableCompositionWarningLocations(t *testing.T) {
	terraSource := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
	terraWant := "terraform {\n  required_version = \">= 1.0\"\n  backend \"local\" {}\n}\n"
	opaque := "variable \"unknown\" {\n  default = \"\u65e5\u672c\u8a9e\"\n  future = true\n}\n"
	variableSource := "variable \"valid\" {\n  default = 1\n  type = number\n}\n"
	variableWant := "variable \"valid\" {\n  type = number\n  default = 1\n}\n"
	source := terraSource + opaque + variableSource
	parsed, diagnostics := parseHCLSource("main.tf", []byte(source))
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	for _, enabled := range []bool{true, false} {
		config := Config{RequiredVersionOrder: true, BackendOrder: true, VariableOrder: enabled}
		candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, config)
		want := terraWant + opaque + variableSource
		if enabled {
			want = terraWant + opaque + variableWant
			if len(warnings) != 1 || warnings[0].Subject.Start.Byte != strings.Index(source, "future =") || warnings[0].Subject.Start.Line != 7 || warnings[0].Subject.Start.Column != 3 {
				t.Fatalf("warnings do not identify original input: %v", warnings)
			}
			if len(changes) != 2 || changes[1].Subject.Start.Byte != strings.Index(source, "variable \"valid\"") {
				t.Fatalf("change does not identify original input: %v", changes)
			}
		} else if len(warnings) != 0 || len(changes) != 1 {
			t.Fatalf("disabled variable interferes with terraform: %v; %v", changes, warnings)
		}
		if string(candidate) != want {
			t.Fatalf("candidate = %s; want %s", candidate, want)
		}
	}
}

func TestVariableOrderPreservesChunks(t *testing.T) {
	chunks := []string{
		"  /* description */\n  description = <<-EOT\n    # not a comment\n    variable description\n  EOT\n",
		"  deprecated = \"old\"\n",
		"  sensitive = false\n",
		"  nullable = false\n",
		"  ephemeral = false\n",
		"  const = false\n",
		"  # type\n  // constraint\n  type = object({\n    z = optional(string)\n    a = list(number)\n  })\n",
		"  default = {\n    z = \"value\" # inner\n    a = [2, 1]\n  } // default\n\n",
		"  # first rule\n  validation {\n    error_message = \"first\"\n    condition = length(var.example.a) > 0\n  } // first end\n",
		"  validation {\n    condition = var.example.z != null\n    error_message = \"second\"\n  }\n",
	}
	prefix := "# file header\nlocals { untouched = [3, 1] }\nvariable \"example\" {\n"
	suffix := "}\n# footer\noutput \"value\" { value = var.example }"
	source := prefix + chunks[8] + chunks[6] + chunks[7] + chunks[5] + chunks[4] + chunks[3] + chunks[2] + chunks[1] + chunks[9] + chunks[0] + suffix
	want := prefix + strings.Join(chunks, "") + suffix
	for _, newline := range []string{"\n", "\r\n"} {
		t.Run(strings.ReplaceAll(newline, "\n", "LF"), func(t *testing.T) {
			input := []byte(strings.ReplaceAll(source, "\n", newline))
			expected := []byte(strings.ReplaceAll(want, "\n", newline))
			parsed, diagnostics := parseHCLSource("main.tf", input)
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{VariableOrder: true})
			if !bytes.Equal(candidate, expected) || len(changes) != 1 || len(warnings) != 0 {
				t.Fatalf("candidate = %s; changes = %v; warnings = %v; want %s", candidate, changes, warnings, expected)
			}
			if !bytes.Equal(parsed.source, input) || changes[0].Subject.Start.Line != 3 || changes[0].Rule != "variable_order" {
				t.Fatalf("source mutated or incorrect change: %v", changes)
			}
			secondParsed, diagnostics := parseHCLSource("main.tf", candidate)
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			second, secondChanges, secondWarnings := transformTerraformOrderWithChanges(secondParsed, Config{VariableOrder: true})
			if !bytes.Equal(second, candidate) || len(secondChanges) != 0 || len(secondWarnings) != 0 {
				t.Fatalf("not idempotent: %s; %v; %v", second, secondChanges, secondWarnings)
			}
		})
	}
}

func TestVariableOrderBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		warning string
	}{
		{"empty", "variable \"example\" {\n}\n", ""},
		{"omitted type", "variable \"example\" {\n  default = [2, 1]\n}\n", ""},
		{"sorted boundary comment", "variable \"example\" {\n  type = string\n\n  # section\n\n  default = \"x\"\n  # trailing\n}\n", ""},
		{"single line", "variable \"example\" { default = 1 }\n", "single-line"},
		{"unknown attribute", "variable \"example\" {\n  type = string\n  future = true\n}\n", "unknown"},
		{"unknown block", "variable \"example\" {\n  future {}\n}\n", "unknown"},
		{"validation label", "variable \"example\" {\n  validation \"rule\" {}\n}\n", "labeled validation"},
		{"missing label", "variable {\n  default = 1\n  type = number\n}\n", "identifier label"},
		{"extra label", "variable \"example\" \"extra\" {\n}\n", "identifier label"},
		{"invalid identifier", "variable \"bad name\" {\n}\n", "identifier label"},
		{"independent comment", "variable \"example\" {\n  default = 1\n  # section\n\n  type = number\n}\n", "ambiguous"},
		{"footer comment", "variable \"example\" {\n  default = 1\n  type = number\n  # footer\n}\n", "ambiguous"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, diagnostics := parseHCLSource("main.tf", []byte(test.source))
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			for _, enabled := range []bool{true, false} {
				candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{VariableOrder: enabled})
				if string(candidate) != test.source || len(changes) != 0 {
					t.Fatalf("changed preserved input: %s; %v", candidate, changes)
				}
				if !enabled || test.warning == "" {
					if len(warnings) != 0 {
						t.Fatalf("unexpected warnings: %v", warnings)
					}
				} else if len(warnings) != 1 || !strings.Contains(warnings[0].Reason, test.warning) || warnings[0].Subject.Start.Line < 1 {
					t.Fatalf("warnings = %v; want %q at input location", warnings, test.warning)
				}
			}
		})
	}
}

func TestVariableCandidateRejectsUnpermittedChanges(t *testing.T) {
	source := "# header\nvariable \"example\" {\n  default = 1 # default\n  type = number # type\n  validation { condition = true }\n  validation { condition = false }\n}\nlocals { value = 1 }\n"
	valid := "# header\nvariable \"example\" {\n  type = number # type\n  default = 1 # default\n  validation { condition = true }\n  validation { condition = false }\n}\nlocals { value = 1 }\n"
	parsed, diagnostics := parseHCLSource("main.tf", []byte(source))
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	if diagnostic := validateVariableCandidate(parsed, []byte(valid)); diagnostic != nil {
		t.Fatal(diagnostic)
	}
	for name, candidate := range map[string]string{
		"value":             strings.Replace(valid, "default = 1", "default = 2", 1),
		"type":              strings.Replace(valid, "number", "string", 1),
		"comment ownership": strings.ReplaceAll(strings.ReplaceAll(valid, "# default", "# TYPE"), "# type", "# default"),
		"validation order":  strings.Replace(valid, "  validation { condition = true }\n  validation { condition = false }", "  validation { condition = false }\n  validation { condition = true }", 1),
		"outside":           strings.Replace(valid, "value = 1", "value = 2", 1),
		"header":            strings.Replace(valid, "header", "HEADER", 1),
		"label":             strings.Replace(valid, "example", "changed", 1),
		"syntax":            valid + "}",
	} {
		t.Run(name, func(t *testing.T) {
			if diagnostic := validateVariableCandidate(parsed, []byte(candidate)); diagnostic == nil {
				t.Fatal("invalid candidate accepted")
			}
		})
	}
}

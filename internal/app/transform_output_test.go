package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestOutputOrderPreservesChunks(t *testing.T) {
	description := "  # output description\n  description = \"Example output\"\n"
	deprecated := "  deprecated = \"Use the replacement\"\n"
	sensitive := "  sensitive = true\n"
	ephemeral := "  ephemeral = false\n"
	typeConstraint := "  type = object({\n    name = string\n    count = number\n  })\n"
	value := "  value = <<-EOT\n    # expression comment\n    { name = \"example\", count = 2 }\n  EOT\n"
	firstPrecondition := "  # first precondition\n  precondition {\n    condition = true\n    error_message = \"first\"\n  } # first end\n"
	secondPrecondition := "  precondition {\n    condition = false\n    error_message = \"second\"\n  }\n"
	dependsOn := "  depends_on = [\n    # dependency comment\n    local.first,\n    local.second,\n  ]\n"
	prefix := "# header\nlocals { untouched = [2, 1] }\noutput \"example\" {\n"
	suffix := "}\n# footer\n"
	sourceBody := dependsOn + secondPrecondition + typeConstraint + value + description + firstPrecondition + ephemeral + sensitive + deprecated
	wantBody := description + deprecated + sensitive + ephemeral + typeConstraint + value + secondPrecondition + firstPrecondition + dependsOn
	source := prefix + sourceBody + suffix
	want := prefix + wantBody + suffix
	for _, newline := range []string{"\n", "\r\n"} {
		t.Run(strings.ReplaceAll(newline, "\n", "LF"), func(t *testing.T) {
			input := []byte(strings.ReplaceAll(source, "\n", newline))
			expected := []byte(strings.ReplaceAll(want, "\n", newline))
			parsed, diagnostics := parseHCLSource("main.tf", input)
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{OutputOrder: true})
			if !bytes.Equal(candidate, expected) || len(changes) != 1 || len(warnings) != 0 {
				t.Fatalf("candidate = %s; changes = %v; warnings = %v; want %s", candidate, changes, warnings, expected)
			}
			if changes[0].Rule != "output_order" || changes[0].Subject.Start.Byte != strings.Index(string(input), "output ") {
				t.Fatalf("change does not identify the original output: %v", changes[0])
			}
			if !bytes.Equal(parsed.source, input) {
				t.Fatal("transform mutated the parsed source")
			}
			secondParsed, diagnostics := parseHCLSource("main.tf", candidate)
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			second, secondChanges, secondWarnings := transformTerraformOrderWithChanges(secondParsed, Config{OutputOrder: true})
			if !bytes.Equal(second, candidate) || len(secondChanges) != 0 || len(secondWarnings) != 0 {
				t.Fatalf("not idempotent: %s; %v; %v", second, secondChanges, secondWarnings)
			}
		})
	}
}

func TestOutputOrderBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		warning string
	}{
		{"empty", "output \"example\" {\n}\n", ""},
		{"omitted attributes", "output \"example\" {\n  value = 1\n}\n", ""},
		{"sorted boundary comment", "output \"example\" {\n  description = \"x\"\n\n  # section\n\n  value = 1\n  # trailing\n}\n", ""},
		{"single line", "output \"example\" { value = 1 }\n", "single-line"},
		{"unknown attribute", "output \"example\" {\n  value = 1\n  future = true\n}\n", "unknown element"},
		{"postcondition", "output \"example\" {\n  value = 1\n  postcondition {}\n}\n", "unknown element"},
		{"labeled precondition", "output \"example\" {\n  value = 1\n  precondition \"rule\" {}\n}\n", "unknown element"},
		{"missing label", "output {\n  value = 1\n}\n", "identifier label"},
		{"extra label", "output \"example\" \"extra\" {\n  value = 1\n}\n", "identifier label"},
		{"invalid identifier", "output \"bad name\" {\n  value = 1\n}\n", "identifier label"},
		{"independent comment", "output \"example\" {\n  value = 1\n  # section\n\n  description = \"x\"\n}\n", "ambiguous comment"},
		{"footer comment", "output \"example\" {\n  value = 1\n  description = \"x\"\n  # footer\n}\n", "ambiguous comment"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, diagnostics := parseHCLSource("main.tf", []byte(test.source))
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			for _, enabled := range []bool{true, false} {
				candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{OutputOrder: enabled})
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

func TestOutputCandidateRejectsUnpermittedChanges(t *testing.T) {
	source := "# header\noutput \"example\" {\n  value = 1 # value\n  precondition { condition = true }\n  precondition { condition = false }\n  description = \"x\" # description\n}\nlocals { value = 1 }\n"
	valid := "# header\noutput \"example\" {\n  description = \"x\" # description\n  value = 1 # value\n  precondition { condition = true }\n  precondition { condition = false }\n}\nlocals { value = 1 }\n"
	parsed, diagnostics := parseHCLSource("main.tf", []byte(source))
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	if diagnostic := validateOutputCandidate(parsed, []byte(valid)); diagnostic != nil {
		t.Fatal(diagnostic)
	}
	for name, candidate := range map[string]string{
		"value":              strings.Replace(valid, "value = 1", "value = 2", 1),
		"comment ownership":  strings.Replace(valid, "# description", "# changed", 1),
		"precondition order": strings.Replace(valid, "condition = true }\n  precondition { condition = false", "condition = false }\n  precondition { condition = true", 1),
		"outside":            strings.Replace(valid, "value = 1 }\n", "value = 2 }\n", 1),
		"label":              strings.Replace(valid, `output "example"`, `output "changed"`, 1),
		"syntax":             valid + "}\n",
	} {
		t.Run(name, func(t *testing.T) {
			if diagnostic := validateOutputCandidate(parsed, []byte(candidate)); diagnostic == nil {
				t.Fatal("invalid candidate accepted")
			}
		})
	}
}

func TestOutputOrderComposition(t *testing.T) {
	terraformSource := "terraform {\n  experiments = [\"future\"]\n  required_version = \">= 1.0\"\n}\n"
	terraformWant := "terraform {\n  required_version = \">= 1.0\"\n  experiments = [\"future\"]\n}\n"
	variableSource := "variable \"example\" {\n  default = [2, 1]\n  type = list(number)\n}\n"
	variableWant := "variable \"example\" {\n  type = list(number)\n  default = [2, 1]\n}\n"
	outputSource := "output \"example\" {\n  value = [2, 1]\n  description = \"example\"\n}\n"
	outputWant := "output \"example\" {\n  description = \"example\"\n  value = [2, 1]\n}\n"
	prefix := "# header\n"
	suffix := "locals { untouched = [3, 1] }\n# footer"
	for _, newline := range []string{"\n", "\r\n"} {
		for _, terraformEnabled := range []bool{false, true} {
			for _, variableEnabled := range []bool{false, true} {
				for _, outputEnabled := range []bool{false, true} {
					t.Run(strings.ReplaceAll(newline, "\n", "LF")+"/terraform="+string(rune('0'+boolToInt(terraformEnabled)))+"/variable="+string(rune('0'+boolToInt(variableEnabled)))+"/output="+string(rune('0'+boolToInt(outputEnabled))), func(t *testing.T) {
						source := prefix + terraformSource + outputSource + variableSource + suffix
						wantTerraform, wantOutput, wantVariable := terraformSource, outputSource, variableSource
						if terraformEnabled {
							wantTerraform = terraformWant
						}
						if variableEnabled {
							wantVariable = variableWant
						}
						if outputEnabled {
							wantOutput = outputWant
						}
						want := prefix + wantTerraform + wantOutput + wantVariable + suffix
						input := []byte(strings.ReplaceAll(source, "\n", newline))
						expected := []byte(strings.ReplaceAll(want, "\n", newline))
						parsed, diagnostics := parseHCLSource("main.tf", input)
						if diagnostics.HasErrors() {
							t.Fatal(diagnostics.Error())
						}
						config := Config{RequiredVersionOrder: terraformEnabled, VariableOrder: variableEnabled, OutputOrder: outputEnabled}
						candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, config)
						wantChanges := boolToInt(terraformEnabled) + boolToInt(variableEnabled) + boolToInt(outputEnabled)
						if !bytes.Equal(candidate, expected) || len(changes) != wantChanges || len(warnings) != 0 {
							t.Fatalf("candidate = %s; changes = %v; warnings = %v; want %s", candidate, changes, warnings, expected)
						}
						for _, change := range changes {
							var originalPosition int
							switch change.Rule {
							case "required_version_order":
								originalPosition = strings.Index(string(input), "required_version")
							case "variable_order":
								originalPosition = strings.Index(string(input), "variable ")
							case "output_order":
								originalPosition = strings.Index(string(input), "output ")
							default:
								t.Fatalf("unexpected change rule: %s", change.Rule)
							}
							if change.Subject.Start.Byte != originalPosition {
								t.Fatalf("change %s starts at %d; want original byte %d", change.Rule, change.Subject.Start.Byte, originalPosition)
							}
						}
					})
				}
			}
		}
	}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

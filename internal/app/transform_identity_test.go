package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestIdentityAttributeOrder(t *testing.T) {
	const sourceBody = `  depends_on = [local.ready]
  tags = { Name = "example" }
  lifecycle {}
  name {
    display_name = "nested"
    id = "unchanged"
  }
  other_name = "ordinary first"
  display_name = var.display_name # display
  instance_id = "ordinary second"
  name = var.name
  provider = example.primary
  # identity
  id = var.id
  for_each = local.items
  count = 1
`
	const wantBody = `  for_each = local.items
  count = 1
  # identity
  id = var.id
  name = var.name
  display_name = var.display_name # display
  provider = example.primary
  other_name = "ordinary first"
  instance_id = "ordinary second"
  name {
    display_name = "nested"
    id = "unchanged"
  }
  tags = { Name = "example" }
  lifecycle {}
  depends_on = [local.ready]
`
	cases := []struct {
		name   string
		header string
		footer string
		body   string
		want   string
		config Config
	}{
		{"resource", "resource \"example\" \"one\" {\n", "}\n", sourceBody, wantBody, Config{ResourceOrder: true}},
		{"data", "data \"example\" \"one\" {\n", "}\n", sourceBody, wantBody, Config{DataOrder: true}},
		{"ephemeral", "ephemeral \"example\" \"one\" {\n", "}\n", sourceBody, wantBody, Config{EphemeralOrder: true}},
		{"check", "check \"one\" {\n  data \"example\" \"one\" {\n", "  }\n}\n", sourceBody, wantBody, Config{CheckOrder: true}},
		{
			"list", "list \"example\" \"one\" {\n", "}\n",
			`  config {
    display_name = "nested"
    id = "unchanged"
  }
  other_name = "ordinary first"
  display_name = var.display_name # display
  instance_id = "ordinary second"
  limit = 5
  name = var.name
  include_resource = true
  provider = example.primary
  # identity
  id = var.id
  for_each = local.items
  count = 1
`,
			`  for_each = local.items
  count = 1
  # identity
  id = var.id
  name = var.name
  display_name = var.display_name # display
  provider = example.primary
  include_resource = true
  limit = 5
  other_name = "ordinary first"
  instance_id = "ordinary second"
  config {
    display_name = "nested"
    id = "unchanged"
  }
`,
			Config{ListOrder: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filename := "main.tf"
			if tc.name == "list" {
				filename = "query.tfquery.hcl"
			}
			for _, newline := range []string{"\n", "\r\n"} {
				source := strings.ReplaceAll(strings.TrimSuffix(tc.header+tc.body+tc.footer, "\n"), "\n", newline)
				want := strings.ReplaceAll(strings.TrimSuffix(tc.header+tc.want+tc.footer, "\n"), "\n", newline)
				parsed, diagnostics := parseHCLSource(filename, []byte(source))
				if diagnostics.HasErrors() {
					t.Fatal(diagnostics.Error())
				}
				candidate, changes, warnings := transformIdentityTestFile(parsed, tc.config, tc.name == "list")
				if string(candidate) != want || len(changes) != 1 || len(warnings) != 0 {
					t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
				}
				sorted, diagnostics := parseHCLSource(filename, candidate)
				if diagnostics.HasErrors() {
					t.Fatal(diagnostics.Error())
				}
				second, changes, warnings := transformIdentityTestFile(sorted, tc.config, tc.name == "list")
				if !bytes.Equal(second, candidate) || len(changes) != 0 || len(warnings) != 0 {
					t.Fatalf("second pass = %s; changes = %v; warnings = %v", second, changes, warnings)
				}
				disabled, changes, warnings := transformIdentityTestFile(parsed, Config{}, tc.name == "list")
				if string(disabled) != source || len(changes) != 0 || len(warnings) != 0 {
					t.Fatalf("disabled = %s; changes = %v; warnings = %v", disabled, changes, warnings)
				}
			}
		})
	}
}

func TestIdentityAttributeOrderPartialAndUnsafe(t *testing.T) {
	for _, blockType := range []string{"resource", "data", "ephemeral", "check", "list"} {
		t.Run(blockType, func(t *testing.T) {
			filename := "main.tf"
			header, footer := blockType+" \"example\" \"one\" {\n", "}\n"
			if blockType == "check" {
				header, footer = "check \"one\" {\n  data \"example\" \"one\" {\n", "  }\n}\n"
			}
			if blockType == "list" {
				filename = "query.tfquery.hcl"
			}
			config := Config{ResourceOrder: true, DataOrder: true, EphemeralOrder: true, CheckOrder: true, ListOrder: true}
			for _, tc := range []struct {
				body string
				want string
				warn bool
			}{
				{"  provider = example.primary\n  display_name = \"only\"\n", "  display_name = \"only\"\n  provider = example.primary\n", false},
				{"  provider = example.primary\n  name = \"only\"\n", "  name = \"only\"\n  provider = example.primary\n", false},
				{"  provider = example.primary\n  id = \"only\"\n", "  id = \"only\"\n  provider = example.primary\n", false},
				{"  provider = example.primary\n\n  # section\n\n  id = var.id\n", "  provider = example.primary\n\n  # section\n\n  id = var.id\n", true},
			} {
				parsed, diagnostics := parseHCLSource(filename, []byte(header+tc.body+footer))
				if diagnostics.HasErrors() {
					t.Fatal(diagnostics.Error())
				}
				candidate, changes, warnings := transformIdentityTestFile(parsed, config, blockType == "list")
				if string(candidate) != header+tc.want+footer {
					t.Fatalf("candidate = %s; want %s", candidate, header+tc.want+footer)
				}
				if tc.warn {
					if len(changes) != 0 || len(warnings) != 1 || !strings.Contains(warnings[0].Reason, "ambiguous comment") {
						t.Fatalf("changes = %v; warnings = %v", changes, warnings)
					}
				} else if len(changes) != 1 || len(warnings) != 0 {
					t.Fatalf("changes = %v; warnings = %v", changes, warnings)
				}
			}
		})
	}
}

func TestIdentityAttributeOrderExcludesOtherContexts(t *testing.T) {
	source := []byte(`module "one" {
  source = "./example"
  display_name = "display"
  name = "name"
  id = "id"
}
provider "example" {
  display_name = "display"
  name = "name"
  id = "id"
}
action "example" "one" {
  provider = example.primary
  display_name = "display"
  name = "name"
  id = "id"
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{
		ModuleOrder: true, ProviderOrder: true, ActionOrder: true,
		ResourceOrder: true, DataOrder: true, EphemeralOrder: true, CheckOrder: true, ListOrder: true,
	})
	if !bytes.Equal(candidate, source) || len(changes) != 0 || len(warnings) != 0 {
		t.Fatalf("candidate = %s; changes = %v; warnings = %v; want unchanged", candidate, changes, warnings)
	}
}

func transformIdentityTestFile(parsed *parsedFile, config Config, query bool) ([]byte, []transformChange, []transformDiagnostic) {
	if query {
		return runTransformPipelineWithFinalValidation(parsed, []transformPipelineStep{
			{enabled: config.ListOrder, transform: transformListOrderWithChanges},
		}, validateProviderMetaCandidate)
	}
	return transformTerraformOrderWithChanges(parsed, config)
}

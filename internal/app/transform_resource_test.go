package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestResourceOrderGroups(t *testing.T) {
	source := []byte(`resource "example" "one" {
  depends_on = [local.first, local.second]
  nested_first {
    value = var.first
  }
  name = var.name
  provider = example.west
  count = 2
  nested_second {
    value = var.second
  }
  for_each = local.items
  id = var.id
}
`)
	want := []byte(`resource "example" "one" {
  count = 2
  for_each = local.items
  provider = example.west
  name = var.name
  id = var.id
  nested_first {
    value = var.first
  }
  nested_second {
    value = var.second
  }
  depends_on = [local.first, local.second]
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ResourceOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || changes[0].Rule != "resource_order" || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestResourceOrderPreservesBytesAndIsIdempotent(t *testing.T) {
	source := []byte(`resource "example" "one" {
	depends_on = [local.first, local.second] # dependency
	# provider selection
	provider = example.west
	# first provider argument
	second = var.second # inline comment
	first = <<-EOT
		# heredoc text
		value
	EOT
	dynamic "label" {
		for_each = ["name"]
		content { value = label.value }
	}
	lifecycle {
		create_before_destroy = true
	}
	count = 1
}`)
	want := []byte(`resource "example" "one" {
	count = 1
	# provider selection
	provider = example.west
	# first provider argument
	second = var.second # inline comment
	first = <<-EOT
		# heredoc text
		value
	EOT
	dynamic "label" {
		for_each = ["name"]
		content { value = label.value }
	}
	lifecycle {
		create_before_destroy = true
	}
	depends_on = [local.first, local.second] # dependency
}`)
	for _, newline := range []string{"\n", "\r\n"} {
		input := bytes.ReplaceAll(source, []byte("\n"), []byte(newline))
		expected := bytes.ReplaceAll(want, []byte("\n"), []byte(newline))
		parsed, diagnostics := parseHCLSource("main.tf", input)
		if diagnostics.HasErrors() {
			t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
		}
		candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ResourceOrder: true})
		if !bytes.Equal(candidate, expected) || len(changes) != 1 || changes[0].Rule != "resource_order" || len(warnings) != 0 {
			t.Fatalf("candidate = %q; changes = %v; warnings = %v; want %q", candidate, changes, warnings, expected)
		}
		parsedCandidate, diagnostics := parseHCLSource("main.tf", candidate)
		if diagnostics.HasErrors() {
			t.Fatalf("candidate parse diagnostics = %s, want none", diagnostics.Error())
		}
		second, secondChanges, secondWarnings := transformTerraformOrderWithChanges(parsedCandidate, Config{ResourceOrder: true})
		if !bytes.Equal(second, candidate) || len(secondChanges) != 0 || len(secondWarnings) != 0 {
			t.Fatalf("second pass = %q; changes = %v; warnings = %v, want unchanged", second, secondChanges, secondWarnings)
		}
	}
}

func TestResourceOrderSafetyAndDisabledRule(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		enabled     bool
		wantWarning string
	}{
		{
			name: "invalid label count",
			source: `resource "one" {
  depends_on = [local.first]
  count = 1
}
`,
			enabled:     true,
			wantWarning: "resource block must have two labels",
		},
		{
			name: "ambiguous comment blocks movement",
			source: `resource "example" "one" {
  depends_on = [local.first]

  # section

  count = 1
}
`,
			enabled:     true,
			wantWarning: "ambiguous comment",
		},
		{
			name: "disabled",
			source: `resource "example" "one" {
  depends_on = [local.first]
  count = 1
}
`,
			enabled: false,
		},
		{
			name: "sorted with ambiguous boundary comment",
			source: `resource "example" "one" {
  count = 1

  # section

  value = "x"
}
`,
			enabled: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, diagnostics := parseHCLSource("main.tf", []byte(test.source))
			if diagnostics.HasErrors() {
				t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
			}
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ResourceOrder: test.enabled})
			if test.wantWarning != "" {
				if !bytes.Equal(candidate, []byte(test.source)) || len(changes) != 0 || len(warnings) != 1 || !strings.Contains(warnings[0].Reason, test.wantWarning) {
					t.Fatalf("candidate = %s; changes = %v; warnings = %v", candidate, changes, warnings)
				}
				return
			}
			if len(warnings) != 0 {
				t.Fatalf("warnings = %v, want none", warnings)
			}
			if !test.enabled && (!bytes.Equal(candidate, []byte(test.source)) || len(changes) != 0) {
				t.Fatalf("disabled rule changed input: %s; changes = %v", candidate, changes)
			}
		})
	}
}

func TestResourceOrderSkipsUnsafeBlockIndependently(t *testing.T) {
	source := []byte(`resource "example" "valid" {
  depends_on = [local.first]
  count = 1
}
resource "invalid" {
  depends_on = [local.second]
  count = 1
}
`)
	want := []byte(`resource "example" "valid" {
  count = 1
  depends_on = [local.first]
}
resource "invalid" {
  depends_on = [local.second]
  count = 1
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ResourceOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || changes[0].Rule != "resource_order" || len(warnings) != 1 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestResourceOrderComposesWithDataOrder(t *testing.T) {
	source := []byte(`data "example" "one" {
  depends_on = [local.data]
  count = 1
}
resource "example" "one" {
  depends_on = [local.resource]
  count = 1
}
`)
	want := []byte(`data "example" "one" {
  count = 1
  depends_on = [local.data]
}
resource "example" "one" {
  count = 1
  depends_on = [local.resource]
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{DataOrder: true, ResourceOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 2 || changes[0].Rule != "data_order" || changes[1].Rule != "resource_order" || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

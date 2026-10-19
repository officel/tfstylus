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
  id = var.id
  name = var.name
  provider = example.west
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

func TestResourceOrderDoesNotValidateLabelCount(t *testing.T) {
	source := []byte(`resource "example" {
  depends_on = [local.first]
  count = 1
}
`)
	want := []byte(`resource "example" {
  count = 1
  depends_on = [local.first]
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ResourceOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || len(warnings) != 0 {
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
resource "example" "unsafe" {
  depends_on = [local.second]

	# section

  count = 1
}
`)
	want := []byte(`resource "example" "valid" {
  count = 1
  depends_on = [local.first]
}
resource "example" "unsafe" {
  depends_on = [local.second]

	# section

  count = 1
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ResourceOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || changes[0].Rule != "resource_order" || len(warnings) != 1 || !strings.Contains(warnings[0].Reason, "ambiguous comment") {
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

func TestTagElementsOrderAfterGeneralBlocks(t *testing.T) {
	cases := []struct {
		name   string
		config Config
		source string
		want   string
	}{
		{
			name:   "resource",
			config: Config{ResourceOrder: true},
			source: `resource "example" "one" {
  depends_on = [local.first]
  tags = { Name = "one" }
  lifecycle {
    prevent_destroy = true
  }
  tag {
    key = "a"
  }
  name = var.name
  nested {
    value = var.value
  }
  count = 1
}
`,
			want: `resource "example" "one" {
  count = 1
  name = var.name
  nested {
    value = var.value
  }
  tags = { Name = "one" }
  tag {
    key = "a"
  }
  lifecycle {
    prevent_destroy = true
  }
  depends_on = [local.first]
}
`,
		},
		{
			name:   "data",
			config: Config{DataOrder: true},
			source: `data "example" "one" {
  tags = { Name = "one" }
  filter {
    name = "x"
  }
  name = var.name
}
`,
			want: `data "example" "one" {
  name = var.name
  filter {
    name = "x"
  }
  tags = { Name = "one" }
}
`,
		},
		{
			name:   "ephemeral",
			config: Config{EphemeralOrder: true},
			source: `ephemeral "example" "one" {
  tags = { Name = "one" }
  nested {
    value = 1
  }
}
`,
			want: `ephemeral "example" "one" {
  nested {
    value = 1
  }
  tags = { Name = "one" }
}
`,
		},
		{
			name:   "check data",
			config: Config{CheckOrder: true},
			source: `check "one" {
  data "example" "one" {
    depends_on = [local.first]
    tags = { Name = "one" }
    name = var.name
  }
  assert {
    condition = true
    error_message = "x"
  }
}
`,
			want: `check "one" {
  data "example" "one" {
    name = var.name
    tags = { Name = "one" }
    depends_on = [local.first]
  }
  assert {
    condition = true
    error_message = "x"
  }
}
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, diagnostics := parseHCLSource("main.tf", []byte(tc.source))
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, tc.config)
			if string(candidate) != tc.want || len(changes) == 0 || len(warnings) != 0 {
				t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, tc.want)
			}
		})
	}
}

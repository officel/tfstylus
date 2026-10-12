package app

import (
	"bytes"
	"testing"
)

func TestEphemeralOrderGroups(t *testing.T) {
	source := []byte(`ephemeral "example" "one" {
  depends_on = [local.first]
  provider = example.west
  first = var.first
  count = 2
}
`)
	want := []byte(`ephemeral "example" "one" {
  count = 2
  provider = example.west
  first = var.first
  depends_on = [local.first]
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{EphemeralOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || changes[0].Rule != "ephemeral_order" || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestEphemeralLifecycleConditionOrder(t *testing.T) {
	source := []byte(`ephemeral "example" "one" {
  lifecycle {
    postcondition {
      error_message = "post"
      condition = true
    }
    precondition {
      error_message = "pre"
      condition = true
    }
  }
}
`)
	want := []byte(`ephemeral "example" "one" {
  lifecycle {
    precondition {
      condition = true
      error_message = "pre"
    }
    postcondition {
      condition = true
      error_message = "post"
    }
  }
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{EphemeralOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || changes[0].Rule != "ephemeral_order" || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestEphemeralConditionKeepsUnknownElements(t *testing.T) {
	source := []byte(`ephemeral "example" "one" {
	lifecycle {
		precondition {
			future_block {}
			future_attribute = true
			error_message = "required"
			condition = true
		}
	}
}
`)
	want := []byte(`ephemeral "example" "one" {
	lifecycle {
		precondition {
			condition = true
			error_message = "required"
			future_attribute = true
			future_block {}
		}
	}
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{EphemeralOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestEphemeralOrderDoesNotValidateLabels(t *testing.T) {
	source := []byte(`ephemeral "example" "one" "extra" {
	lifecycle "extra" {
		precondition "extra" {
			error_message = "required"
			condition = true
		}
	}
	depends_on = [local.first]
	count = 1
}
`)
	want := []byte(`ephemeral "example" "one" "extra" {
	count = 1
	lifecycle "extra" {
		precondition "extra" {
			condition = true
			error_message = "required"
		}
	}
	depends_on = [local.first]
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{EphemeralOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestEphemeralOrderComposesWithDataOrder(t *testing.T) {
	source := []byte(`data "example" "one" {
  depends_on = [local.data]
  count = 1
}
ephemeral "example" "one" {
  depends_on = [local.ephemeral]
  count = 1
}
`)
	want := []byte(`data "example" "one" {
  count = 1
  depends_on = [local.data]
}
ephemeral "example" "one" {
  count = 1
  depends_on = [local.ephemeral]
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{DataOrder: true, EphemeralOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 2 || changes[0].Rule != "data_order" || changes[1].Rule != "ephemeral_order" || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestEphemeralOrderPreservesBytesAndIsIdempotent(t *testing.T) {
	source := []byte(`ephemeral "example" "one" {
	depends_on = [local.first, local.second]
	filter {
		values = ["one"]
	}
	dynamic "label" {
		for_each = ["name"]
		content { value = label.value }
	}
	provider = example.west
	# provider argument
	second = var.second # inline comment
	first = <<-EOT
		# heredoc text
		value
	EOT
	for_each = local.items
	count = 2
}`)
	want := []byte(`ephemeral "example" "one" {
	for_each = local.items
	count = 2
	provider = example.west
	# provider argument
	second = var.second # inline comment
	first = <<-EOT
		# heredoc text
		value
	EOT
	filter {
		values = ["one"]
	}
	dynamic "label" {
		for_each = ["name"]
		content { value = label.value }
	}
	depends_on = [local.first, local.second]
}`)
	for _, newline := range []string{"\n", "\r\n"} {
		input := bytes.ReplaceAll(source, []byte("\n"), []byte(newline))
		expected := bytes.ReplaceAll(want, []byte("\n"), []byte(newline))
		parsed, diagnostics := parseHCLSource("main.tf", input)
		if diagnostics.HasErrors() {
			t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
		}
		candidate, changes, warnings := transformEphemeralOrderWithChanges(parsed)
		if !bytes.Equal(candidate, expected) || len(changes) != 1 || len(warnings) != 0 {
			t.Fatalf("candidate = %q; changes = %v; warnings = %v; want %q", candidate, changes, warnings, expected)
		}
		parsedCandidate, diagnostics := parseHCLSource("main.tf", candidate)
		if diagnostics.HasErrors() {
			t.Fatalf("candidate parse diagnostics = %s, want none", diagnostics.Error())
		}
		second, secondChanges, secondWarnings := transformEphemeralOrderWithChanges(parsedCandidate)
		if !bytes.Equal(second, candidate) || len(secondChanges) != 0 || len(secondWarnings) != 0 {
			t.Fatalf("second pass = %q; changes = %v; warnings = %v, want unchanged", second, secondChanges, secondWarnings)
		}
	}
}

func TestEphemeralOrderSkipsUnsafeBlockIndependently(t *testing.T) {
	source := []byte(`ephemeral "example" "valid" {
  depends_on = [local.first]
  count = 1
}
ephemeral "example" "unsafe" {
  depends_on = [local.second]

	# section

  count = 1
}
`)
	want := []byte(`ephemeral "example" "valid" {
  count = 1
  depends_on = [local.first]
}
ephemeral "example" "unsafe" {
  depends_on = [local.second]

	# section

  count = 1
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{EphemeralOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || changes[0].Rule != "ephemeral_order" || len(warnings) != 1 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestEphemeralOrderSafetyAndDisabledRule(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		enabled  bool
		wantWarn bool
	}{
		{
			name: "ambiguous comment blocks movement",
			source: `ephemeral "example" "one" {
  depends_on = [local.first]

  # section

  count = 1
}
`,
			enabled:  true,
			wantWarn: true,
		},
		{
			name: "disabled",
			source: `ephemeral "example" "one" {
  depends_on = [local.first]
  count = 1
}
`,
			enabled: false,
		},
		{
			name: "sorted with ambiguous boundary comment",
			source: `ephemeral "example" "one" {
  count = 1

  # section

  value = "x"
}
`,
			enabled: true,
		},
		{
			name: "multiple lifecycle blocks during reorder",
			source: `ephemeral "example" "one" {
  depends_on = [local.first]
  lifecycle {}
  lifecycle {}
  count = 1
}
`,
			enabled:  true,
			wantWarn: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, diagnostics := parseHCLSource("main.tf", []byte(test.source))
			if diagnostics.HasErrors() {
				t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
			}
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{EphemeralOrder: test.enabled})
			if string(candidate) != test.source || len(changes) != 0 {
				t.Fatalf("candidate = %q; changes = %v, want unchanged", candidate, changes)
			}
			if test.wantWarn != (len(warnings) == 1) {
				t.Fatalf("warnings = %v; wantWarn = %t", warnings, test.wantWarn)
			}
			if test.wantWarn && warnings[0].Subject.Start.Line < 1 {
				t.Fatalf("warning subject = %v, want original source location", warnings[0].Subject)
			}
		})
	}
}

package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestActionOrderGroups(t *testing.T) {
	source := []byte(`action "example" "one" {
  config {
    second = jsonencode({ value = "unchanged" })
    settings {
      name = "nested"
    }
  }
  provider = example.west
  count = 2
}
`)
	want := []byte(`action "example" "one" {
  count = 2
  provider = example.west
  config {
    second = jsonencode({ value = "unchanged" })
    settings {
      name = "nested"
    }
  }
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ActionOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || changes[0].Rule != "action_order" || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestActionOrderPreservesStableGroupsAndConfig(t *testing.T) {
	source := []byte(`action "example" "one" {
  config {
    second = <<-EOT
      unchanged body
    EOT
    # inside config
    settings {
      name = "nested"
    }
  }
  provider = example.west
  for_each = { item = "value" }
  config {
    first = var.first
  }
  count = 2
}
`)
	want := []byte(`action "example" "one" {
  for_each = { item = "value" }
  count = 2
  provider = example.west
  config {
    second = <<-EOT
      unchanged body
    EOT
    # inside config
    settings {
      name = "nested"
    }
  }
  config {
    first = var.first
  }
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ActionOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || changes[0].Rule != "action_order" || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
	parsedCandidate, diagnostics := parseHCLSource("main.tf", candidate)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	second, secondChanges, secondWarnings := transformTerraformOrderWithChanges(parsedCandidate, Config{ActionOrder: true})
	if !bytes.Equal(second, candidate) || len(secondChanges) != 0 || len(secondWarnings) != 0 {
		t.Fatalf("not idempotent: %q; %v; %v", second, secondChanges, secondWarnings)
	}
}

func TestActionOrderGroupsUnknownElementsWithoutRejectingThem(t *testing.T) {
	source := []byte(`action "example" "one" {
	config {
		setting = true
	}
	future_block {
		value = "kept"
	}
	future_attribute = true
	provider = example.west
	count = 1
}
`)
	want := []byte(`action "example" "one" {
	count = 1
	provider = example.west
	future_attribute = true
	config {
		setting = true
	}
	future_block {
		value = "kept"
	}
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ActionOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestActionOrderDoesNotValidateLabelCount(t *testing.T) {
	source := []byte(`action "example" {
  config {
    setting = true
  }
  count = 1
}
`)
	want := []byte(`action "example" {
  count = 1
  config {
    setting = true
  }
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ActionOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestActionOrderUnsafeBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		warning string
	}{
		{
			name:    "single-line body",
			source:  "action \"example\" \"one\" { count = 1 }\n",
			warning: "single-line",
		},
		{
			name:    "ambiguous comment while moving",
			source:  "action \"example\" \"one\" {\n  config { setting = true }\n\n  # section\n\n  count = 1\n}\n",
			warning: "ambiguous comment",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, diagnostics := parseHCLSource("main.tf", []byte(test.source))
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ActionOrder: true})
			if string(candidate) != test.source || len(changes) != 0 {
				t.Fatalf("candidate = %q; changes = %v; want unchanged input", candidate, changes)
			}
			if len(warnings) != 1 || !strings.Contains(warnings[0].Reason, test.warning) || warnings[0].Subject.Start.Line < 1 {
				t.Fatalf("warnings = %v; want %q at input location", warnings, test.warning)
			}
		})
	}
}

func TestActionOrderDoesNotWarnForSortedBoundaryCommentOrDisabledRule(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		enabled bool
	}{
		{
			name:    "sorted boundary comment",
			source:  "action \"example\" \"one\" {\n  count = 1\n\n  # section\n\n  provider = example.west\n  config { setting = true }\n}\n",
			enabled: true,
		},
		{
			name:    "disabled rule",
			source:  "action \"example\" \"one\" {\n  config { setting = true }\n  count = 1\n}\n",
			enabled: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, diagnostics := parseHCLSource("main.tf", []byte(test.source))
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ActionOrder: test.enabled})
			if string(candidate) != test.source || len(changes) != 0 || len(warnings) != 0 {
				t.Fatalf("candidate = %q; changes = %v; warnings = %v", candidate, changes, warnings)
			}
		})
	}
}

func TestActionOrderPreservesCRLFUTF8AndEOF(t *testing.T) {
	source := []byte(`action "example" "one" {
  config {
    payload = <<-EOT
      keep UTF-8: 値
    EOT
  }
  count = 1
}`)
	want := []byte(`action "example" "one" {
  count = 1
  config {
    payload = <<-EOT
      keep UTF-8: 値
    EOT
  }
}`)
	for _, newline := range []string{"\n", "\r\n"} {
		input := bytes.ReplaceAll(source, []byte("\n"), []byte(newline))
		expected := bytes.ReplaceAll(want, []byte("\n"), []byte(newline))
		parsed, diagnostics := parseHCLSource("main.tf", input)
		if diagnostics.HasErrors() {
			t.Fatal(diagnostics.Error())
		}
		candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ActionOrder: true})
		if !bytes.Equal(candidate, expected) || len(changes) != 1 || len(warnings) != 0 {
			t.Fatalf("candidate = %q; changes = %v; warnings = %v; want %q", candidate, changes, warnings, expected)
		}
	}
}

func TestValidateActionCandidateRejectsUnpermittedChanges(t *testing.T) {
	source := []byte(`action "example" "one" {
  config {
    setting = true
  }
  count = 1
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ActionOrder: true})
	if len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("changes = %v; warnings = %v", changes, warnings)
	}
	if diagnostic := validateActionCandidate(parsed, candidate); diagnostic != nil {
		t.Fatalf("validateActionCandidate() = %v for valid candidate", diagnostic)
	}
	mutated := bytes.Replace(candidate, []byte("setting = true"), []byte("setting = false"), 1)
	if diagnostic := validateActionCandidate(parsed, mutated); diagnostic == nil {
		t.Fatal("validateActionCandidate() = nil for altered config expression")
	}
}

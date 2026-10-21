package app

import (
	"bytes"
	"testing"
)

func TestLifecycleElementOrderAcrossBlockTypes(t *testing.T) {
	lifecycle := `  lifecycle {
    # postcondition comment
    postcondition {
      condition = true
    }
    replace_triggered_by = [local.replace]
    ignore_changes = [all]
    destroy = false
    prevent_destroy = true
    create_before_destroy = true
    # action trigger comment
    action_trigger {
      events = ["after_create"]
    }
    precondition {
      condition = true
    }
    custom = "keep"
    future_block {}
  }
`
	wantLifecycle := `  lifecycle {
    # action trigger comment
    action_trigger {
      events = ["after_create"]
    }
    create_before_destroy = true
    prevent_destroy = true
    ignore_changes = [all]
    replace_triggered_by = [local.replace]
    precondition {
      condition = true
    }
    # postcondition comment
    postcondition {
      condition = true
    }
    destroy = false
    custom = "keep"
    future_block {}
  }
`
	tests := []struct {
		name      string
		source    string
		want      string
		transform func(*parsedFile) ([]byte, []transformChange, []transformDiagnostic)
	}{
		{
			name: "resource",
			source: `resource "unregistered_example" "one" {
` + lifecycle + `}
`,
			want: `resource "unregistered_example" "one" {
` + wantLifecycle + `}
`,
			transform: transformResourceOrderWithChanges,
		},
		{
			name: "data",
			source: `data "unregistered_example" "one" {
` + lifecycle + `}
`,
			want: `data "unregistered_example" "one" {
` + wantLifecycle + `}
`,
			transform: transformDataOrderWithChanges,
		},
		{
			name: "ephemeral",
			source: `ephemeral "unregistered_example" "one" {
` + lifecycle + `}
`,
			want: `ephemeral "unregistered_example" "one" {
` + wantLifecycle + `}
`,
			transform: transformEphemeralOrderWithChanges,
		},
		{
			name: "removed",
			source: `removed {
` + lifecycle + `  from = unregistered_example.old
}
`,
			want: `removed {
  from = unregistered_example.old
` + wantLifecycle + `}
`,
			transform: transformRemovedOrderWithChanges,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, diagnostics := parseHCLSource("main.tf", []byte(test.source))
			if diagnostics.HasErrors() {
				t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
			}
			candidate, changes, warnings := test.transform(parsed)
			if !bytes.Equal(candidate, []byte(test.want)) || len(changes) != 1 || len(warnings) != 0 {
				t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, test.want)
			}
			parsedCandidate, diagnostics := parseHCLSource("main.tf", candidate)
			if diagnostics.HasErrors() {
				t.Fatalf("candidate parse diagnostics = %s, want none", diagnostics.Error())
			}
			second, secondChanges, secondWarnings := test.transform(parsedCandidate)
			if !bytes.Equal(second, candidate) || len(secondChanges) != 0 || len(secondWarnings) != 0 {
				t.Fatalf("second pass = %q; changes = %v; warnings = %v, want unchanged", second, secondChanges, secondWarnings)
			}
		})
	}
}

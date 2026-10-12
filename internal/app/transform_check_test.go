package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestCheckOrderGroupsAndNestedBodies(t *testing.T) {
	source := []byte(`check "health" {
  assert {
    error_message = "second assertion"
    condition = true
  }
  data "http" "endpoint" {
    provider = http.primary
    depends_on = [aws_instance.server]
    url = "https://example.com/health"
  }
  assert {
    error_message = "first assertion"
    condition = false
  }
  data "http" "fallback" {
    provider = http.secondary
    depends_on = [aws_instance.backup]
    url = "https://backup.example.com/health"
  }
}
`)
	want := []byte(`check "health" {
  data "http" "endpoint" {
    url = "https://example.com/health"
    depends_on = [aws_instance.server]
    provider = http.primary
  }
  data "http" "fallback" {
    url = "https://backup.example.com/health"
    depends_on = [aws_instance.backup]
    provider = http.secondary
  }
  assert {
    condition = true
    error_message = "second assertion"
  }
  assert {
    condition = false
    error_message = "first assertion"
  }
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{CheckOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || changes[0].Rule != "check_order" || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestCheckOrderKeepsUnknownElementsWithoutValidatingLabels(t *testing.T) {
	source := []byte(`check "health" "extra" {
	future_block {}
	future = true
	assert "extra" {
		future_assertion = true
		error_message = "failed"
		condition = true
	}
	data "http" {
		provider = http.primary
		depends_on = [aws_instance.server]
		url = "https://example.com"
	}
}
`)
	want := []byte(`check "health" "extra" {
	data "http" {
		url = "https://example.com"
		depends_on = [aws_instance.server]
		provider = http.primary
	}
	assert "extra" {
		condition = true
		error_message = "failed"
		future_assertion = true
	}
	future = true
	future_block {}
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{CheckOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestCheckOrderUnsafeStructuresPreserveWholeCheck(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		warning string
	}{
		{
			name:    "ambiguous comment while moving",
			source:  "check \"health\" {\n  assert {\n    condition = true\n    error_message = \"failed\"\n  }\n\n  # section\n\n  data \"http\" \"endpoint\" {\n    url = \"https://example.com\"\n  }\n}\n",
			warning: "ambiguous comment",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, diagnostics := parseHCLSource("main.tf", []byte(test.source))
			if diagnostics.HasErrors() {
				t.Fatalf("parseHCLSource() diagnostics = %s", diagnostics.Error())
			}
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{CheckOrder: true})
			if string(candidate) != test.source || len(changes) != 0 {
				t.Fatalf("candidate = %q; changes = %v; want unchanged input", candidate, changes)
			}
			if len(warnings) != 1 || !strings.Contains(warnings[0].Reason, test.warning) || warnings[0].Subject.Start.Line < 1 {
				t.Fatalf("warnings = %v; want %q at input location", warnings, test.warning)
			}
		})
	}
}

func TestCheckOrderDoesNotWarnForSortedCommentOrDisabledRule(t *testing.T) {
	source := "check \"health\" {\n  data \"http\" \"endpoint\" {\n    url = \"https://example.com\"\n  }\n\n  # assertion section\n\n  assert {\n    condition = true\n    error_message = \"failed\"\n  }\n}\n"
	for _, enabled := range []bool{true, false} {
		parsed, diagnostics := parseHCLSource("main.tf", []byte(source))
		if diagnostics.HasErrors() {
			t.Fatal(diagnostics.Error())
		}
		candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{CheckOrder: enabled})
		if string(candidate) != source || len(changes) != 0 || len(warnings) != 0 {
			t.Fatalf("enabled = %t; candidate = %q; changes = %v; warnings = %v", enabled, candidate, changes, warnings)
		}
	}
}

func TestCheckOrderPreservesCRLFUTF8AndEOF(t *testing.T) {
	source := []byte("check \"health\" {\n  assert {\n    error_message = \"失敗\"\n    condition = true\n  }\n  data \"http\" \"endpoint\" {\n    provider = http.primary\n    url = <<-EOT\n      https://example.com/health\n      keep UTF-8: 値\n    EOT\n  }\n}")
	want := []byte("check \"health\" {\n  data \"http\" \"endpoint\" {\n    url = <<-EOT\n      https://example.com/health\n      keep UTF-8: 値\n    EOT\n    provider = http.primary\n  }\n  assert {\n    condition = true\n    error_message = \"失敗\"\n  }\n}")
	for _, newline := range []string{"\n", "\r\n"} {
		input := bytes.ReplaceAll(source, []byte("\n"), []byte(newline))
		expected := bytes.ReplaceAll(want, []byte("\n"), []byte(newline))
		parsed, diagnostics := parseHCLSource("main.tf", input)
		if diagnostics.HasErrors() {
			t.Fatal(diagnostics.Error())
		}
		candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{CheckOrder: true})
		if !bytes.Equal(candidate, expected) || len(changes) != 1 || len(warnings) != 0 {
			t.Fatalf("candidate = %q; changes = %v; warnings = %v; want %q", candidate, changes, warnings, expected)
		}
	}
}

func TestValidateCheckCandidateRejectsUnpermittedChanges(t *testing.T) {
	source := []byte("check \"health\" {\n  assert {\n    error_message = \"failed\"\n    condition = true\n  }\n  data \"http\" \"endpoint\" {\n    url = \"https://example.com\"\n  }\n}\n")
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{CheckOrder: true})
	if len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("changes = %v; warnings = %v", changes, warnings)
	}
	if diagnostic := validateCheckCandidate(parsed, candidate); diagnostic != nil {
		t.Fatalf("validateCheckCandidate() = %v for valid candidate", diagnostic)
	}
	mutated := bytes.Replace(candidate, []byte("condition = true"), []byte("condition = null"), 1)
	if diagnostic := validateCheckCandidate(parsed, mutated); diagnostic == nil {
		t.Fatal("validateCheckCandidate() = nil for altered assertion expression")
	}
}

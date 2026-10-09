package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRequiredVersionOrderMovesAttributeToTerraformBlockStart(t *testing.T) {
	source := []byte(`terraform {
  backend "local" {}
  required_version = ">= 1.0"
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, transformDiagnostics := transformRequiredVersionOrder(parsed)
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformRequiredVersionOrder() diagnostics = %v, want none", transformDiagnostics)
	}
	want := []byte(`terraform {
  required_version = ">= 1.0"
  backend "local" {}
}`)
	if !bytes.Equal(candidate, want) {
		t.Errorf("transformRequiredVersionOrder() =\n%s\nwant:\n%s", candidate, want)
	}
}

func TestRequiredVersionOrderMovesTrailingCommentWithAttribute(t *testing.T) {
	source := []byte(`terraform {
  backend "local" {}
  required_version = ">= 1.0" # keep with version
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, transformDiagnostics := transformRequiredVersionOrder(parsed)
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformRequiredVersionOrder() diagnostics = %v, want none", transformDiagnostics)
	}
	want := []byte(`terraform {
  required_version = ">= 1.0" # keep with version
  backend "local" {}
}`)
	if !bytes.Equal(candidate, want) {
		t.Errorf("transformRequiredVersionOrder() =\n%s\nwant:\n%s", candidate, want)
	}
}

func TestRequiredVersionOrderMovesLeadingCommentWithAttribute(t *testing.T) {
	source := []byte(`terraform {
  backend "local" {}
	# keep with version
	// preserve comment style
	/* preserve block comment */
  required_version = ">= 1.0"
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, transformDiagnostics := transformRequiredVersionOrder(parsed)
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformRequiredVersionOrder() diagnostics = %v, want none", transformDiagnostics)
	}
	want := []byte(`terraform {
	# keep with version
	// preserve comment style
	/* preserve block comment */
  required_version = ">= 1.0"
  backend "local" {}
}`)
	if !bytes.Equal(candidate, want) {
		t.Errorf("transformRequiredVersionOrder() =\n%s\nwant:\n%s", candidate, want)
	}
}

func TestRequiredVersionOrderSkipsIndependentComment(t *testing.T) {
	source := []byte(`terraform {
  backend "local" {}
  # independent section comment

  required_version = ">= 1.0"
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, transformDiagnostics := transformRequiredVersionOrder(parsed)
	if len(transformDiagnostics) == 0 {
		t.Fatal("transformRequiredVersionOrder() diagnostics are empty, want independent comment warning")
	}
	if !bytes.Equal(candidate, source) {
		t.Errorf("transformRequiredVersionOrder() changed ambiguous input:\n%s\nwant unchanged:\n%s", candidate, source)
	}
}

func TestRequiredVersionOrderPreservesHeredocExpression(t *testing.T) {
	source := []byte(`terraform {
  backend "local" {}
  required_version = <<-EOT
    >= 1.0
  EOT
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, transformDiagnostics := transformRequiredVersionOrder(parsed)
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformRequiredVersionOrder() diagnostics = %v, want none", transformDiagnostics)
	}
	want := []byte(`terraform {
  required_version = <<-EOT
    >= 1.0
  EOT
  backend "local" {}
}`)
	if !bytes.Equal(candidate, want) {
		t.Errorf("transformRequiredVersionOrder() =\n%s\nwant:\n%s", candidate, want)
	}
}

func TestRequiredVersionOrderHandlesMultipleTerraformBlocks(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "required-version", "multiple-blocks.input.tf"))
	if err != nil {
		t.Fatalf("read input fixture: %v", err)
	}

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, transformDiagnostics := transformRequiredVersionOrder(parsed)
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformRequiredVersionOrder() diagnostics = %v, want none", transformDiagnostics)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "required-version", "multiple-blocks.want.tf"))
	if err != nil {
		t.Fatalf("read expected fixture: %v", err)
	}
	if !bytes.Equal(candidate, want) {
		t.Errorf("transformRequiredVersionOrder() =\n%s\nwant:\n%s", candidate, want)
	}

	if _, candidateDiagnostics := parseHCLSource("candidate.tf", candidate); candidateDiagnostics.HasErrors() {
		t.Fatalf("transformed candidate diagnostics = %s, want no errors", candidateDiagnostics.Error())
	}
}

func TestRequiredVersionOrderIsNoOpWhenAlreadyOrderedOrMissing(t *testing.T) {
	tests := []struct {
		name   string
		source []byte
	}{
		{
			name: "already ordered",
			source: []byte(`terraform {
  required_version = ">= 1.0"
  backend "local" {}
}`),
		},
		{
			name: "attribute absent",
			source: []byte(`terraform {
  backend "local" {}
}`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, diagnostics := parseHCLSource("main.tf", test.source)
			if diagnostics.HasErrors() {
				t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
			}

			candidate, transformDiagnostics := transformRequiredVersionOrder(parsed)
			if len(transformDiagnostics) != 0 {
				t.Fatalf("transformRequiredVersionOrder() diagnostics = %v, want none", transformDiagnostics)
			}
			if !bytes.Equal(candidate, test.source) {
				t.Errorf("transformRequiredVersionOrder() changed no-op input:\n%s\nwant unchanged:\n%s", candidate, test.source)
			}
		})
	}
}

func TestRequiredVersionOrderIsIdempotent(t *testing.T) {
	source := []byte(`terraform {
  backend "local" {}
  # required by the tooling
  required_version = <<-EOT
    >= 1.0
  EOT
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	first, transformDiagnostics := transformRequiredVersionOrder(parsed)
	if len(transformDiagnostics) != 0 {
		t.Fatalf("first transform diagnostics = %v, want none", transformDiagnostics)
	}
	secondParsed, diagnostics := parseHCLSource("first.tf", first)
	if diagnostics.HasErrors() {
		t.Fatalf("first candidate diagnostics = %s, want no errors", diagnostics.Error())
	}
	second, transformDiagnostics := transformRequiredVersionOrder(secondParsed)
	if len(transformDiagnostics) != 0 {
		t.Fatalf("second transform diagnostics = %v, want none", transformDiagnostics)
	}
	if !bytes.Equal(second, first) {
		t.Errorf("second transform changed candidate:\n%s\nwant unchanged:\n%s", second, first)
	}
}

func TestValidateRequiredVersionCandidateRejectsInvalidOrChangedCandidate(t *testing.T) {
	source := []byte(`terraform {
  backend "local" {}
  required_version = ">= 1.0"
}`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	tests := []struct {
		name      string
		candidate []byte
	}{
		{
			name:      "invalid HCL",
			candidate: []byte(`terraform {`),
		},
		{
			name: "changed non-target attribute",
			candidate: []byte(`terraform {
  required_version = ">= 1.0"
  backend "remote" {}
}`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if diagnostic := validateRequiredVersionCandidate(parsed, test.candidate); diagnostic == nil {
				t.Fatal("validateRequiredVersionCandidate() returned no diagnostic")
			}
		})
	}
}

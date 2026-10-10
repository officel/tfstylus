package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRuleToggle(t *testing.T) {
	tests := []struct {
		name       string
		configText string
		source     []byte
		want       []byte
	}{
		{
			name:       "disabled leaves ambiguous input unchanged without diagnostics",
			configText: "required_version_order = false",
			source: []byte(`terraform {
  backend "local" {}
  # independent section comment

  required_version = ">= 1.0"
}`),
			want: []byte(`terraform {
  backend "local" {}
  # independent section comment

  required_version = ">= 1.0"
}`),
		},
		{
			name:   "enabled applies the rule",
			source: []byte("terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}"),
			want:   []byte("terraform {\n  required_version = \">= 1.0\"\n  backend \"local\" {}\n}"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(configPath, []byte(test.configText), 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := loadConfig(configPath)
			if err != nil {
				t.Fatalf("loadConfig() error = %v, want nil", err)
			}

			parsed, diagnostics := parseHCLSource("main.tf", test.source)
			if diagnostics.HasErrors() {
				t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
			}
			candidate, transformDiagnostics := transformRequiredVersionOrder(parsed, config)
			if len(transformDiagnostics) != 0 {
				t.Fatalf("transformRequiredVersionOrder() diagnostics = %v, want none", transformDiagnostics)
			}
			if !bytes.Equal(candidate, test.want) {
				t.Errorf("transformRequiredVersionOrder() =\n%s\nwant:\n%s", candidate, test.want)
			}
		})
	}
}

func TestRequiredVersionOrderMovesAttributeToTerraformBlockStart(t *testing.T) {
	source := []byte(`terraform {
  backend "local" {}
  required_version = ">= 1.0"
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, transformDiagnostics := transformRequiredVersionOrder(parsed, Config{RequiredVersionOrder: true})
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

	candidate, transformDiagnostics := transformRequiredVersionOrder(parsed, Config{RequiredVersionOrder: true})
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

func TestRequiredVersionOrderMovesMultilineTrailingCommentWithAttribute(t *testing.T) {
	source := []byte(`terraform {
  backend "local" {}
  required_version = ">= 1.0" /* keep
    with version */
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, transformDiagnostics := transformRequiredVersionOrder(parsed, Config{RequiredVersionOrder: true})
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformRequiredVersionOrder() diagnostics = %v, want none", transformDiagnostics)
	}
	want := []byte(`terraform {
  required_version = ">= 1.0" /* keep
    with version */
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

	candidate, transformDiagnostics := transformRequiredVersionOrder(parsed, Config{RequiredVersionOrder: true})
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

	candidate, transformDiagnostics := transformRequiredVersionOrder(parsed, Config{RequiredVersionOrder: true})
	if len(transformDiagnostics) == 0 {
		t.Fatal("transformRequiredVersionOrder() diagnostics are empty, want independent comment warning")
	}
	if !bytes.Equal(candidate, source) {
		t.Errorf("transformRequiredVersionOrder() changed ambiguous input:\n%s\nwant unchanged:\n%s", candidate, source)
	}
}

func TestRequiredVersionOrderPreservesNestedComments(t *testing.T) {
	source := []byte(`terraform {
  backend "local" {
    path = "state.tfstate" # keep inside backend
  }
  required_version = ">= 1.0"
}`)
	backend := []byte(`backend "local" {
    path = "state.tfstate" # keep inside backend
  }`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, transformDiagnostics := transformRequiredVersionOrder(parsed, Config{RequiredVersionOrder: true})
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformRequiredVersionOrder() diagnostics = %v, want none", transformDiagnostics)
	}
	want := []byte(`terraform {
  required_version = ">= 1.0"
  backend "local" {
    path = "state.tfstate" # keep inside backend
  }
}`)
	if !bytes.Equal(candidate, want) {
		t.Errorf("transformRequiredVersionOrder() =\n%s\nwant:\n%s", candidate, want)
	}
	if !bytes.Contains(candidate, backend) {
		t.Errorf("transformed candidate does not preserve backend bytes:\n%s", candidate)
	}

	validated, diagnostics := parseHCLSource("candidate.tf", candidate)
	if diagnostics.HasErrors() {
		t.Fatalf("candidate parse diagnostics = %s, want no errors", diagnostics.Error())
	}
	second, transformDiagnostics := transformRequiredVersionOrder(validated, Config{RequiredVersionOrder: true})
	if len(transformDiagnostics) != 0 {
		t.Fatalf("second transform diagnostics = %v, want none", transformDiagnostics)
	}
	if !bytes.Equal(second, candidate) {
		t.Errorf("second transform changed candidate:\n%s\nwant unchanged:\n%s", second, candidate)
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

	candidate, transformDiagnostics := transformRequiredVersionOrder(parsed, Config{RequiredVersionOrder: true})
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

	candidate, transformDiagnostics := transformRequiredVersionOrder(parsed, Config{RequiredVersionOrder: true})
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
			name: "already ordered with trailing boundary comment",
			source: []byte(`terraform {
  required_version = ">= 1.15.0"
  # コメント
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

			candidate, transformDiagnostics := transformRequiredVersionOrder(parsed, Config{RequiredVersionOrder: true})
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

	first, transformDiagnostics := transformRequiredVersionOrder(parsed, Config{RequiredVersionOrder: true})
	if len(transformDiagnostics) != 0 {
		t.Fatalf("first transform diagnostics = %v, want none", transformDiagnostics)
	}
	secondParsed, diagnostics := parseHCLSource("first.tf", first)
	if diagnostics.HasErrors() {
		t.Fatalf("first candidate diagnostics = %s, want no errors", diagnostics.Error())
	}
	second, transformDiagnostics := transformRequiredVersionOrder(secondParsed, Config{RequiredVersionOrder: true})
	if len(transformDiagnostics) != 0 {
		t.Fatalf("second transform diagnostics = %v, want none", transformDiagnostics)
	}
	if !bytes.Equal(second, first) {
		t.Errorf("second transform changed candidate:\n%s\nwant unchanged:\n%s", second, first)
	}
}

func TestValidateRequiredVersionCandidateRejectsInvalidOrChangedCandidate(t *testing.T) {
	source := []byte(`terraform {
  # backend comment
  backend "local" {}
  # keep with version
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
		{
			name: "moved leading comment to a different element",
			candidate: []byte(`terraform {
  # backend comment
  required_version = ">= 1.0"
  # keep with version
  backend "local" {}
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

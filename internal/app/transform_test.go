package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
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

func TestBackendOrderMovesBlockAfterRequiredVersion(t *testing.T) {
	source := []byte(`terraform {
  backend "local" {}
  required_version = ">= 1.0"
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, transformDiagnostics := transformBackendOrder(parsed, Config{RequiredVersionOrder: true, BackendOrder: true})
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformBackendOrder() diagnostics = %v, want none", transformDiagnostics)
	}
	want := []byte(`terraform {
  required_version = ">= 1.0"
  backend "local" {}
}`)
	if !bytes.Equal(candidate, want) {
		t.Errorf("transformBackendOrder() =\n%s\nwant:\n%s", candidate, want)
	}
}

func TestBackendOrderMovesBlockToTerraformBlockStartWhenVersionMissing(t *testing.T) {
	source := []byte(`terraform {
  cloud {
    organization = "example"
  }
  backend "local" {}
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, transformDiagnostics := transformBackendOrder(parsed, Config{RequiredVersionOrder: true, BackendOrder: true})
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformBackendOrder() diagnostics = %v, want none", transformDiagnostics)
	}
	want := []byte(`terraform {
  backend "local" {}
  cloud {
    organization = "example"
  }
}`)
	if !bytes.Equal(candidate, want) {
		t.Errorf("transformBackendOrder() =\n%s\nwant:\n%s", candidate, want)
	}
}

func TestBackendOrderSkipsMultipleBackends(t *testing.T) {
	source := []byte(`terraform {
  backend "local" {}
  backend "s3" {}
  required_version = ">= 1.0"
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, transformDiagnostics := transformBackendOrder(parsed, Config{RequiredVersionOrder: true, BackendOrder: true})
	if len(transformDiagnostics) != 1 {
		t.Fatalf("transformBackendOrder() diagnostics = %v, want exactly one warning", transformDiagnostics)
	}
	if !bytes.Equal(candidate, source) {
		t.Errorf("transformBackendOrder() changed ambiguous input:\n%s\nwant unchanged:\n%s", candidate, source)
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

func TestRequiredVersionOrderSkipsAmbiguousBoundaryComments(t *testing.T) {
	tests := []struct {
		name             string
		source           []byte
		ambiguousComment []byte
	}{
		{
			name: "comment before closing brace",
			source: []byte(`# fixed file header
terraform {
  backend "local" {}
  required_version = ">= 1.0"
  # ambiguous body-end comment
}
# fixed file footer
`),
			ambiguousComment: []byte("# ambiguous body-end comment"),
		},
		{
			name: "multiline section comment between elements",
			source: []byte(`# fixed file header
terraform {
  backend "local" {}

  /* shared by backend and required_version
   * across multiple lines
   */

  required_version = ">= 1.0"
}
# fixed file footer
`),
			ambiguousComment: []byte("/* shared by backend and required_version\n   * across multiple lines\n   */"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, parseDiagnostics := parseHCLSource("main.tf", test.source)
			if parseDiagnostics.HasErrors() {
				t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", parseDiagnostics.Error())
			}

			var expectedSubject *hclsyntax.Token
			for index := range parsed.tokens {
				token := &parsed.tokens[index]
				tokenText := bytes.TrimRight(test.source[token.Range.Start.Byte:token.Range.End.Byte], "\r\n")
				if token.Type == hclsyntax.TokenComment && bytes.Equal(tokenText, test.ambiguousComment) {
					if expectedSubject != nil {
						t.Fatalf("ambiguous comment %q appears more than once", test.ambiguousComment)
					}
					expectedSubject = token
				}
			}
			if expectedSubject == nil {
				t.Fatalf("ambiguous comment %q was not found in HCL tokens", test.ambiguousComment)
			}

			candidate, transformDiagnostics := transformRequiredVersionOrder(parsed, Config{RequiredVersionOrder: true})
			if !bytes.Equal(candidate, test.source) {
				t.Errorf("transformRequiredVersionOrder() changed ambiguous input:\n%s\nwant unchanged:\n%s", candidate, test.source)
			}
			if len(transformDiagnostics) != 1 {
				t.Fatalf("transformRequiredVersionOrder() diagnostics = %v, want exactly one", transformDiagnostics)
			}

			diagnostic := transformDiagnostics[0]
			if diagnostic.Reason != "terraform body contains a comment whose ownership is not yet classified" {
				t.Errorf("diagnostic reason = %q, want unclassified comment reason", diagnostic.Reason)
			}
			if diagnostic.Subject != expectedSubject.Range {
				t.Errorf("diagnostic subject = %+v, want comment token range %+v", diagnostic.Subject, expectedSubject.Range)
			}
		})
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

func TestRequiredVersionOrderPreservesSourceBytes(t *testing.T) {
	tests := []struct {
		name   string
		source []byte
		want   []byte
	}{
		{
			name:   "utf8 comments are preserved",
			source: []byte("terraform {\n  backend \"local\" {}\n  # UTF-8: コメント\n  required_version = \">= 1.0\"\n}\n"),
			want:   []byte("terraform {\n  # UTF-8: コメント\n  required_version = \">= 1.0\"\n  backend \"local\" {}\n}\n"),
		},
		{
			name:   "crlf line endings are preserved",
			source: []byte("terraform {\r\n  backend \"local\" {}\r\n  required_version = \">= 1.0\"\r\n}\r\n"),
			want:   []byte("terraform {\r\n  required_version = \">= 1.0\"\r\n  backend \"local\" {}\r\n}\r\n"),
		},
		{
			name:   "final newline is preserved when absent",
			source: []byte("terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}"),
			want:   []byte("terraform {\n  required_version = \">= 1.0\"\n  backend \"local\" {}\n}"),
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
			if !bytes.Equal(candidate, test.want) {
				t.Fatalf("transformRequiredVersionOrder() =\n%s\nwant:\n%s", candidate, test.want)
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
				t.Fatalf("second transform changed candidate:\n%s\nwant unchanged:\n%s", second, candidate)
			}
		})
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

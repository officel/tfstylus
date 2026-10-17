package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestReplaceRequiredVersion(t *testing.T) {
	source := []byte("terraform {\n  # minimum\n  required_version = \">= 1.0\" # keep\n  experiments = [example]\n}\n")
	want := []byte("terraform {\n  # minimum\n  required_version = \">= 1.7.0, < 2.0.0\" # keep\n  experiments = [example]\n}\n")
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ReplaceRequiredVersion: ">= 1.7.0, < 2.0.0"})
	if len(warnings) != 0 || len(changes) != 1 || !bytes.Equal(candidate, want) {
		t.Fatalf("candidate = %s, changes = %v, warnings = %v; want %s", candidate, changes, warnings, want)
	}
}

func TestReplaceRequiredVersionUnsupportedBody(t *testing.T) {
	source := []byte(`{"terraform":{"required_version":"old"}}`)
	parsed, diagnostics := parseJSONSource("main.tf.json", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ReplaceRequiredVersion: ">= 1.7.0"})
	if !bytes.Equal(candidate, source) || len(changes) != 0 || len(warnings) == 0 {
		t.Fatal("unsupported body was not preserved")
	}
}

func TestReplaceRequiredVersionBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		constraint string
		want       string
		warnings   int
	}{
		{name: "disabled", source: `terraform { required_version = var.minimum }`, want: `terraform { required_version = var.minimum }`},
		{name: "missing attribute", source: `terraform {}`, constraint: ">= 1.7.0", want: `terraform {}`},
		{name: "missing block", source: `locals { required_version = "old" }`, constraint: ">= 1.7.0", want: `locals { required_version = "old" }`},
		{name: "shorter value", source: `terraform { required_version = ">= 1.7.0, < 2.0.0" }`, constraint: "1.8", want: `terraform { required_version = "1.8" }`},
		{name: "equal decoded value", source: `terraform { required_version = ">= \u0031.7.0" }`, constraint: ">= 1.7.0", want: `terraform { required_version = ">= \u0031.7.0" }`},
		{name: "reference", source: `terraform { required_version = var.minimum }`, constraint: ">= 1.7.0", want: `terraform { required_version = var.minimum }`, warnings: 1},
		{name: "interpolation", source: `terraform { required_version = "${var.minimum}" }`, constraint: ">= 1.7.0", want: `terraform { required_version = "${var.minimum}" }`, warnings: 1},
		{name: "expression comment", source: `terraform { required_version = (/* keep */ "old") }`, constraint: ">= 1.7.0", want: `terraform { required_version = (/* keep */ "old") }`, warnings: 1},
		{name: "heredoc", source: "terraform {\nrequired_version = <<EOT\n>= 1.0\nEOT\n}\n", constraint: ">= 1.7.0", want: "terraform {\nrequired_version = <<EOT\n>= 1.0\nEOT\n}\n", warnings: 1},
		{name: "CRLF and UTF8", source: "# 日本語\r\nterraform {\r\n  required_version = \"old\" # keep\r\n}\r\n", constraint: ">= 1.7.0", want: "# 日本語\r\nterraform {\r\n  required_version = \">= 1.7.0\" # keep\r\n}\r\n"},
		{name: "multiple blocks", source: "terraform { required_version = \"old\" }\nterraform { required_version = \"older\" }", constraint: ">= 1.7.0", want: "terraform { required_version = \">= 1.7.0\" }\nterraform { required_version = \">= 1.7.0\" }"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, parseDiagnostics := parseHCLSource("main.tf", []byte(test.source))
			if parseDiagnostics.HasErrors() {
				t.Fatal(parseDiagnostics.Error())
			}
			config := Config{ReplaceRequiredVersion: test.constraint}
			candidate, _, warnings := transformTerraformOrderWithChanges(parsed, config)
			if string(candidate) != test.want || len(warnings) != test.warnings {
				t.Fatalf("candidate = %q, warnings = %v; want %q, %d warnings", candidate, warnings, test.want, test.warnings)
			}
			reparsed, parseDiagnostics := parseHCLSource("main.tf", candidate)
			if parseDiagnostics.HasErrors() {
				t.Fatal(parseDiagnostics.Error())
			}
			second, changes, _ := transformTerraformOrderWithChanges(reparsed, config)
			if !bytes.Equal(candidate, second) || len(changes) != 0 {
				t.Fatal("replacement is not idempotent")
			}
		})
	}
}

func TestValidateRequiredVersionReplacementRejectsTampering(t *testing.T) {
	source := []byte("terraform {\n  required_version = \"old\" # keep\n}\nlocals { untouched = \"safe\" }\n")
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, _, warnings := transformRequiredVersionReplacement(parsed, ">= 1.7.0")
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	for _, change := range []struct{ from, to []byte }{
		{[]byte(">= 1.7.0"), []byte(">= 1.8.0")},
		{[]byte("safe"), []byte("evil")},
		{[]byte("# keep"), []byte("# lost")},
		{[]byte(" # keep"), nil},
		{[]byte("required_version"), []byte("unknown_version")},
	} {
		modified := bytes.ReplaceAll(candidate, change.from, change.to)
		if bytes.Equal(modified, candidate) {
			t.Fatal("tampering probe did not change candidate")
		}
		if validateRequiredVersionReplacement(parsed, modified, ">= 1.7.0") == nil {
			t.Fatalf("accepted tampering: %s", modified)
		}
	}
	if validateRequiredVersionReplacement(parsed, source, ">= 1.7.0") == nil {
		t.Fatal("accepted missing replacement")
	}
}

func TestReplaceRequiredVersionComposition(t *testing.T) {
	input, err := os.ReadFile(filepath.Join("..", "..", "testdata", "required-version-replace", "composition.input.tf"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "required-version-replace", "composition.output.tf"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, diagnostics := parseHCLSource("main.tf", input)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	configPath := filepath.Join(t.TempDir(), "config.hcl")
	writeConfig(t, configPath, `replace_required_version = ">= 1.7.0"`)
	config, err := loadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, config)
	if len(warnings) != 0 || !bytes.Equal(candidate, want) {
		t.Fatalf("candidate = %s, warnings = %v; want %s", candidate, warnings, want)
	}
	var replacements int
	for _, change := range changes {
		if change.Rule == "terraform/replace-required-version" {
			replacements++
			if change.Subject.Start.Line != 4 && change.Subject.Start.Line != 15 {
				t.Fatalf("wrong original position: %v", change.Subject)
			}
		}
	}
	if replacements != 2 {
		t.Fatalf("replacement count = %d", replacements)
	}
	reparsed, diagnostics := parseHCLSource("main.tf", candidate)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	second, secondChanges, warnings := transformTerraformOrderWithChanges(reparsed, config)
	if len(warnings) != 0 || len(secondChanges) != 0 || !bytes.Equal(second, candidate) {
		t.Fatal("composed transformation is not idempotent")
	}
}

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

func TestApplySourceEditsRejectsOverlappingRanges(t *testing.T) {
	source := []byte("abcdef")
	edits := []sourceEdit{
		{start: 0, end: 3, text: []byte("first")},
		{start: 2, end: 5, text: []byte("second")},
	}
	if candidate := applySourceEdits(source, edits); !bytes.Equal(candidate, source) {
		t.Fatalf("overlapping edits produced %q, want unchanged source %q", candidate, source)
	}
}

func TestApplySourceEditsPreservesEditOrderAndAppliesDisjointRanges(t *testing.T) {
	source := []byte("abcdef")
	edits := []sourceEdit{
		{start: 0, end: 1, text: []byte("A")},
		{start: 5, end: 6, text: []byte("F")},
	}
	candidate := applySourceEdits(source, edits)
	if !bytes.Equal(candidate, []byte("AbcdeF")) {
		t.Fatalf("disjoint edits produced %q, want %q", candidate, "AbcdeF")
	}
	if edits[0].start != 0 || edits[1].start != 5 {
		t.Fatalf("applySourceEdits() reordered input edits: %v", edits)
	}
}

func TestApplySourceMovesPreservesByteCount(t *testing.T) {
	source := []byte("__abcdef")
	edit := sourceEdit{start: 6, end: 8, insertAt: 2, text: []byte("ef")}
	candidate := applySourceMoves(source, []sourceEdit{edit})
	if !bytes.Equal(candidate, []byte("__efabcd")) || len(candidate) != len(source) {
		t.Fatalf("source move produced %q, want %q with unchanged byte count", candidate, "__efabcd")
	}
}

func TestApplySourceMovesRejectsChangedBytesWhenRelocating(t *testing.T) {
	source := []byte("__abcdef")
	edit := sourceEdit{start: 6, end: 8, insertAt: 2, text: []byte("ZZ")}
	candidate := applySourceMoves(source, []sourceEdit{edit})
	if !bytes.Equal(candidate, source) {
		t.Fatalf("changed source bytes were moved: got %q, want unchanged source %q", candidate, source)
	}
}

func TestApplySourceMovesRejectsOverlappingRanges(t *testing.T) {
	source := []byte("abcdefg")
	edits := []sourceEdit{
		{start: 1, end: 4, insertAt: 6, text: []byte("bcd")},
		{start: 3, end: 5, insertAt: 1, text: []byte("de")},
	}
	if candidate := applySourceMoves(source, edits); !bytes.Equal(candidate, source) {
		t.Fatalf("overlapping source moves produced %q, want unchanged source %q", candidate, source)
	}
}

func TestApplySourceMovesRejectsInsertionInsideRemovedRange(t *testing.T) {
	source := []byte("__abcdef")
	edit := sourceEdit{start: 4, end: 6, insertAt: 5, text: []byte("ef")}
	if candidate := applySourceMoves(source, []sourceEdit{edit}); !bytes.Equal(candidate, source) {
		t.Fatalf("move into its own source range produced %q, want unchanged source %q", candidate, source)
	}
}

func TestValidateSourceCandidatePreservesModuleChunks(t *testing.T) {
	source := []byte(`module "example" {
  # dependency
  depends_on = [module.network]
	# input
	input = <<-EOT
		heredoc text
	EOT
  # source
  source = "./module"
}
locals { untouched = 1 }
`)
	candidate := []byte(`module "example" {
  # source
  source = "./module"
	# input
	input = <<-EOT
		heredoc text
	EOT
  # dependency
  depends_on = [module.network]
}
locals { untouched = 1 }
`)
	original, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parse original: %s", diagnostics.Error())
	}
	policy := map[string]sourceBodyPolicy{
		"module": {owner: "module", rank: moduleElementRank},
	}
	if diagnostic := validateSourceCandidate(original, candidate, policy); diagnostic != nil {
		t.Fatalf("valid source-chunk permutation rejected: %v", diagnostic)
	}

	for name, mutated := range map[string][]byte{
		"expression": bytes.Replace(candidate, []byte(`"./module"`), []byte(`"../other"`), 1),
		"heredoc":    bytes.Replace(candidate, []byte("heredoc text"), []byte("changed text"), 1),
		"comment ownership": []byte(`module "example" {
  # dependency
  source = "./module"
  # source
  depends_on = [module.network]
}
locals { untouched = 1 }
`),
		"non-target bytes":    bytes.Replace(candidate, []byte("untouched = 1"), []byte("untouched = 2"), 1),
		"deleted element":     bytes.Replace(candidate, []byte("\t# input\n\tinput = <<-EOT\n\t\theredoc text\n\tEOT\n"), nil, 1),
		"duplicated elements": append(bytes.Clone(candidate), candidate...),
		"unconfigured block":  bytes.Replace(candidate, []byte(`"./module"`), []byte(`"../other"`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(mutated, candidate) {
				t.Fatal("test mutation did not change candidate")
			}
			candidatePolicy := policy
			if name == "duplicated elements" || name == "unconfigured block" {
				candidatePolicy = map[string]sourceBodyPolicy{}
			}
			if diagnostic := validateSourceCandidate(original, mutated, candidatePolicy); diagnostic == nil {
				t.Fatal("validateSourceCandidate() = nil, want preservation diagnostic")
			}
		})
	}
}

func TestValidateSourceCandidateKeepsRepeatedBlockIdentity(t *testing.T) {
	source := []byte(`module "same" {
  depends_on = [module.first]
  value = "alpha"
  source = "./shared"
}
module "same" {
  depends_on = [module.other]
  value = "bravo"
  source = "./shared"
}
`)
	candidate := []byte(`module "same" {
  source = "./shared"
  value = "alpha"
  depends_on = [module.first]
}
module "same" {
  source = "./shared"
  value = "bravo"
  depends_on = [module.other]
}
`)
	mutated := []byte(`module "same" {
  source = "./shared"
  value = "bravo"
  depends_on = [module.other]
}
module "same" {
  source = "./shared"
  value = "alpha"
  depends_on = [module.first]
}
`)
	original, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parse original: %s", diagnostics.Error())
	}
	policy := map[string]sourceBodyPolicy{
		"module": {owner: "module", rank: moduleElementRank},
	}
	if diagnostic := validateSourceCandidate(original, candidate, policy); diagnostic != nil {
		t.Fatalf("valid repeated blocks rejected: %v", diagnostic)
	}
	if bytes.Equal(mutated, candidate) {
		t.Fatal("test mutation did not change candidate")
	}
	if diagnostic := validateSourceCandidate(original, mutated, policy); diagnostic == nil {
		t.Fatal("validateSourceCandidate() = nil, want repeated block identity diagnostic")
	}
}

func TestRequiredProvidersOrderMovesSourceBeforeVersion(t *testing.T) {
	source := []byte(`terraform {
  required_providers {
    aws = {
      version = "~> 5.0"
      source  = "hashicorp/aws"
    }
  }
}`)
	want := []byte(`terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, changes, transformDiagnostics := transformTerraformOrderWithChanges(parsed, Config{RequiredProvidersOrder: true})
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformTerraformOrderWithChanges() diagnostics = %v, want none", transformDiagnostics)
	}
	if len(changes) != 1 || changes[0].Rule != "required_providers_order" {
		t.Fatalf("transformTerraformOrderWithChanges() changes = %v, want one required_providers_order change", changes)
	}
	if !bytes.Equal(candidate, want) {
		t.Fatalf("transformTerraformOrderWithChanges() =\n%s\nwant:\n%s", candidate, want)
	}
}

func TestRequiredProvidersOrderMovesAttachedCommentsWithKeys(t *testing.T) {
	source := []byte(`terraform {
	required_providers {
		aws = {
			version = "~> 5.0" # version constraint
			# provider source
			source = "hashicorp/aws" # source address
		}
	}
}`)
	want := []byte(`terraform {
	required_providers {
		aws = {
			# provider source
			source = "hashicorp/aws" # source address
			version = "~> 5.0" # version constraint
		}
	}
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, _, transformDiagnostics := transformTerraformOrderWithChanges(parsed, Config{RequiredProvidersOrder: true})
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformTerraformOrderWithChanges() diagnostics = %v, want none", transformDiagnostics)
	}
	if !bytes.Equal(candidate, want) {
		t.Fatalf("transformTerraformOrderWithChanges() =\n%s\nwant:\n%s", candidate, want)
	}
}

func TestRequiredProvidersOrderPreservesCRLFAndIsIdempotent(t *testing.T) {
	source := []byte("terraform {\r\n  required_providers {\r\n    aws = {\r\n      version = \"~> 5.0\" # バージョン\r\n      source = \"hashicorp/aws\"\r\n    }\r\n  }\r\n}")
	want := []byte("terraform {\r\n  required_providers {\r\n    aws = {\r\n      source = \"hashicorp/aws\"\r\n      version = \"~> 5.0\" # バージョン\r\n    }\r\n  }\r\n}")

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}
	candidate, _, transformDiagnostics := transformTerraformOrderWithChanges(parsed, Config{RequiredProvidersOrder: true})
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformTerraformOrderWithChanges() diagnostics = %v, want none", transformDiagnostics)
	}
	if !bytes.Equal(candidate, want) {
		t.Fatalf("transformTerraformOrderWithChanges() =\n%q\nwant:\n%q", candidate, want)
	}

	parsedCandidate, diagnostics := parseHCLSource("candidate.tf", candidate)
	if diagnostics.HasErrors() {
		t.Fatalf("candidate parse diagnostics = %s, want no errors", diagnostics.Error())
	}
	second, changes, transformDiagnostics := transformTerraformOrderWithChanges(parsedCandidate, Config{RequiredProvidersOrder: true})
	if len(transformDiagnostics) != 0 || len(changes) != 0 {
		t.Fatalf("second transform changes = %v, diagnostics = %v, want none", changes, transformDiagnostics)
	}
	if !bytes.Equal(second, candidate) {
		t.Fatalf("second transform changed candidate:\n%q\nwant unchanged:\n%q", second, candidate)
	}
}

func TestRequiredProvidersOrderPreservesAliasesSyntaxAndProviderOrder(t *testing.T) {
	source := []byte(`terraform {
	required_providers {
		zeta = {
			"configuration_aliases": [zeta.west, zeta.east],
			"version": "~> 2.0",
			"source": "example/zeta",
		}
		alpha = {
			version = ">= 1.0"
		}
	}
}`)
	want := []byte(`terraform {
	required_providers {
		zeta = {
			"source": "example/zeta",
			"version": "~> 2.0",
			"configuration_aliases": [zeta.west, zeta.east],
		}
		alpha = {
			version = ">= 1.0"
		}
	}
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, changes, transformDiagnostics := transformTerraformOrderWithChanges(parsed, Config{RequiredProvidersOrder: true})
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformTerraformOrderWithChanges() diagnostics = %v, want none", transformDiagnostics)
	}
	if len(changes) != 1 || changes[0].Rule != "required_providers_order" {
		t.Fatalf("transformTerraformOrderWithChanges() changes = %v, want one required_providers_order change", changes)
	}
	if !bytes.Equal(candidate, want) {
		t.Fatalf("transformTerraformOrderWithChanges() =\n%s\nwant:\n%s", candidate, want)
	}
}

func TestRequiredProvidersOrderSkipsUnsafeProviderObjects(t *testing.T) {
	tests := []struct {
		name         string
		providerBody string
		wantWarning  bool
	}{
		{
			name:         "legacy string is preserved",
			providerBody: `aws = "~> 5.0"`,
		},
		{
			name:         "unknown key",
			providerBody: "aws = {\n      version = \"~> 5.0\"\n      source = \"hashicorp/aws\"\n      other = true\n    }",
			wantWarning:  true,
		},
		{
			name:         "duplicate key",
			providerBody: "aws = {\n      version = \"~> 5.0\"\n      source = \"hashicorp/aws\"\n      source = \"example/aws\"\n    }",
			wantWarning:  true,
		},
		{
			name:         "dynamic key",
			providerBody: "aws = {\n      (local.key) = \"~> 5.0\"\n      source = \"hashicorp/aws\"\n    }",
			wantWarning:  true,
		},
		{
			name:         "single-line object",
			providerBody: `aws = { version = "~> 5.0", source = "hashicorp/aws" }`,
			wantWarning:  true,
		},
		{
			name:         "first item shares the opening line",
			providerBody: "aws = { version = \"~> 5.0\",\n      source = \"hashicorp/aws\"\n    }",
			wantWarning:  true,
		},
		{
			name:         "independent comment",
			providerBody: "aws = {\n      # section comment\n\n      version = \"~> 5.0\"\n      source = \"hashicorp/aws\"\n    }",
			wantWarning:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := []byte("terraform {\n  required_providers {\n    " + test.providerBody + "\n  }\n}")
			parsed, diagnostics := parseHCLSource("main.tf", source)
			if diagnostics.HasErrors() {
				t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
			}

			candidate, _, transformDiagnostics := transformTerraformOrderWithChanges(parsed, Config{RequiredProvidersOrder: true})
			if gotWarning := len(transformDiagnostics) > 0; gotWarning != test.wantWarning {
				t.Fatalf("transformTerraformOrderWithChanges() diagnostics = %v, warning = %t, want %t", transformDiagnostics, gotWarning, test.wantWarning)
			}
			if !bytes.Equal(candidate, source) {
				t.Fatalf("transformTerraformOrderWithChanges() changed skipped input:\n%s\nwant unchanged:\n%s", candidate, source)
			}
		})
	}
}

func TestValidateRequiredProvidersCandidate(t *testing.T) {
	originalSource := []byte(`terraform {
	required_providers {
		zeta = {
			version = "~> 2.0"
			source = "example/zeta"
		}
		alpha = {
			version = ">= 1.0"
		}
	}
}`)
	original, diagnostics := parseHCLSource("main.tf", originalSource)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	tests := []struct {
		name      string
		candidate []byte
		wantError bool
	}{
		{
			name: "allows required key reordering",
			candidate: []byte(`terraform {
	required_providers {
		zeta = {
			source = "example/zeta"
			version = "~> 2.0"
		}
		alpha = {
			version = ">= 1.0"
		}
	}
}`),
		},
		{
			name: "rejects changed value",
			candidate: []byte(`terraform {
	required_providers {
		zeta = {
			source = "evil/zeta"
			version = "~> 2.0"
		}
		alpha = {
			version = ">= 1.0"
		}
	}
}`),
			wantError: true,
		},
		{
			name: "rejects changed provider declaration order",
			candidate: []byte(`terraform {
	required_providers {
		alpha = {
			version = ">= 1.0"
		}
		zeta = {
			source = "example/zeta"
			version = "~> 2.0"
		}
	}
}`),
			wantError: true,
		},
		{
			name: "rejects changed separator bytes",
			candidate: []byte(`terraform {
	required_providers {
		zeta = {
			source: "example/zeta"
			version = "~> 2.0"
		}
		alpha = {
			version = ">= 1.0"
		}
	}
}`),
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostic := validateRequiredProvidersCandidate(original, test.candidate)
			if gotError := diagnostic != nil; gotError != test.wantError {
				t.Fatalf("validateRequiredProvidersCandidate() diagnostic = %v, error = %t, want error %t", diagnostic, gotError, test.wantError)
			}
		})
	}
}

func TestValidateProviderMetaCandidatePreservesOpaqueBlocks(t *testing.T) {
	originalSource := []byte(`terraform {
	provider_meta "example/first" {
		unknown_value = local.metadata # expression comment
		payload = <<-EOT
		  provider-defined data
		EOT
	}
	provider_meta "example/second" {
		unknown_attribute = { nested = ["value"] }
	}
	provider_meta "example/first" {
		unknown_value = "repeated label"
	}
}`)
	original, diagnostics := parseHCLSource("main.tf", originalSource)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	mutatedBody := []byte(`terraform {
	provider_meta "example/first" {
		unknown_value = local.changed # expression comment
		payload = <<-EOT
		  provider-defined data
		EOT
	}
	provider_meta "example/second" {
		unknown_attribute = { nested = ["value"] }
	}
	provider_meta "example/first" {
		unknown_value = "repeated label"
	}
}`)
	reorderedBlocks := []byte(`terraform {
	provider_meta "example/second" {
		unknown_attribute = { nested = ["value"] }
	}
	provider_meta "example/first" {
		unknown_value = local.metadata # expression comment
		payload = <<-EOT
		  provider-defined data
		EOT
	}
	provider_meta "example/first" {
		unknown_value = "repeated label"
	}
}`)

	for _, test := range []struct {
		name      string
		candidate []byte
	}{
		{name: "changed opaque body", candidate: mutatedBody},
		{name: "reordered labels and repeated blocks", candidate: reorderedBlocks},
	} {
		t.Run(test.name, func(t *testing.T) {
			if diagnostic := validateProviderMetaCandidate(original, test.candidate); diagnostic == nil {
				t.Fatal("validateProviderMetaCandidate() = nil, want preservation diagnostic")
			}
		})
	}
	if diagnostic := validateProviderMetaCandidate(original, originalSource); diagnostic != nil {
		t.Fatalf("validateProviderMetaCandidate() = %v, want no diagnostic for unchanged blocks", diagnostic)
	}
}

func TestBackendOrderPreservesBlankLineChunks(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "backend-order", "cloud-blank-lines.input.tf"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "backend-order", "cloud-blank-lines.want.tf"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		source []byte
		want   []byte
	}{
		{name: "reported cloud input", source: source, want: want},
		{
			name:   "CRLF",
			source: bytes.ReplaceAll(source, []byte("\n"), []byte("\r\n")),
			want:   bytes.ReplaceAll(want, []byte("\n"), []byte("\r\n")),
		},
		{
			name:   "no final newline",
			source: bytes.TrimSuffix(source, []byte("\n")),
			want:   bytes.TrimSuffix(want, []byte("\n")),
		},
		{
			name:   "backend with trailing gap and comments",
			source: []byte("terraform {\n  required_version = \">= 1.5.0\" # version\n\n  required_providers {}\n\n  # state\n  backend \"local\" {} # backend\n\n  experiments = []\n}\n"),
			want:   []byte("terraform {\n  required_version = \">= 1.5.0\" # version\n\n  # state\n  backend \"local\" {} # backend\n\n  required_providers {}\n\n  experiments = []\n}\n"),
		},
		{
			name:   "cloud without required version",
			source: []byte("terraform {\n  required_providers {}\n\n  cloud {}\n\n  experiments = []\n}\n"),
			want:   []byte("terraform {\n  cloud {}\n\n  required_providers {}\n\n  experiments = []\n}\n"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, diagnostics := parseHCLSource("main.tf", test.source)
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			config := Config{RequiredVersionOrder: true, BackendOrder: true}
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, config)
			if len(warnings) != 0 {
				t.Fatalf("warnings = %v, want none", warnings)
			}
			if !bytes.Equal(candidate, test.want) {
				t.Fatalf("candidate =\n%s\nwant:\n%s", candidate, test.want)
			}
			if len(changes) != 1 || changes[0].Rule != "backend_order" {
				t.Fatalf("changes = %v, want one backend_order change", changes)
			}
			again, diagnostics := parseHCLSource("main.tf", candidate)
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			second, changes, warnings := transformTerraformOrderWithChanges(again, config)
			if !bytes.Equal(second, candidate) || len(changes) != 0 || len(warnings) != 0 {
				t.Fatalf("second transform = %s, changes = %v, warnings = %v; want unchanged", second, changes, warnings)
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

func TestTerraformOrderPreservesProviderMetaDuringOuterMoves(t *testing.T) {
	source := []byte(`terraform {
	provider_meta "example/first" {
		unknown_value = local.metadata # expression comment
		payload = <<-EOT
		  provider-defined data
		EOT
	}
	backend "local" {}
	provider_meta "example/second" {
		unknown_attribute = { nested = ["value"] }
	}
	provider_meta "example/first" {
		unknown_value = "repeated label"
	}
	required_version = ">= 1.0"
}`)
	want := []byte(`terraform {
	required_version = ">= 1.0"
	backend "local" {}
	provider_meta "example/first" {
		unknown_value = local.metadata # expression comment
		payload = <<-EOT
		  provider-defined data
		EOT
	}
	provider_meta "example/second" {
		unknown_attribute = { nested = ["value"] }
	}
	provider_meta "example/first" {
		unknown_value = "repeated label"
	}
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}
	config := Config{RequiredVersionOrder: true, BackendOrder: true}
	candidate, _, transformDiagnostics := transformTerraformOrderWithChanges(parsed, config)
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformTerraformOrderWithChanges() diagnostics = %v, want none", transformDiagnostics)
	}
	if !bytes.Equal(candidate, want) {
		t.Fatalf("transformTerraformOrderWithChanges() =\n%s\nwant:\n%s", candidate, want)
	}

	parsedCandidate, diagnostics := parseHCLSource("candidate.tf", candidate)
	if diagnostics.HasErrors() {
		t.Fatalf("candidate parse diagnostics = %s, want no errors", diagnostics.Error())
	}
	second, changes, transformDiagnostics := transformTerraformOrderWithChanges(parsedCandidate, config)
	if len(transformDiagnostics) != 0 || len(changes) != 0 || !bytes.Equal(second, candidate) {
		t.Fatalf("second transform = %s, changes = %v, diagnostics = %v; want unchanged candidate", second, changes, transformDiagnostics)
	}
}

func TestTerraformOrderPreservesExperimentsDuringOuterMoves(t *testing.T) {
	source := []byte(`terraform {
	experiments = [
		"future_alpha",
		# keep this expression comment
		"older_experiment",
	]
	backend "local" {}
	required_version = ">= 1.0"
}`)
	want := []byte(`terraform {
	required_version = ">= 1.0"
	backend "local" {}
	experiments = [
		"future_alpha",
		# keep this expression comment
		"older_experiment",
	]
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}
	config := Config{RequiredVersionOrder: true, BackendOrder: true}
	candidate, _, transformDiagnostics := transformTerraformOrderWithChanges(parsed, config)
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformTerraformOrderWithChanges() diagnostics = %v, want none", transformDiagnostics)
	}
	if !bytes.Equal(candidate, want) {
		t.Fatalf("transformTerraformOrderWithChanges() =\n%s\nwant:\n%s", candidate, want)
	}

	parsedCandidate, diagnostics := parseHCLSource("candidate.tf", candidate)
	if diagnostics.HasErrors() {
		t.Fatalf("candidate parse diagnostics = %s, want no errors", diagnostics.Error())
	}
	originalBody := parsed.body.(*hclsyntax.Body).Blocks[0].Body
	candidateBody := parsedCandidate.body.(*hclsyntax.Body).Blocks[0].Body
	originalExpression := originalBody.Attributes["experiments"].Expr.Range()
	candidateExpression := candidateBody.Attributes["experiments"].Expr.Range()
	if !sameRangeBytes(source, originalExpression, candidate, candidateExpression) {
		t.Fatal("experiments expression bytes changed during outer moves")
	}
	if originalExpression.Start.Byte == candidateExpression.Start.Byte {
		t.Fatal("experiments expression offset did not change during outer moves")
	}
}

func TestVariableOrderMovesTypeBeforeDefault(t *testing.T) {
	source := []byte("variable \"example\" {\n  default = 1\n  type = number\n}\n")
	want := []byte("variable \"example\" {\n  type = number\n  default = 1\n}\n")
	configPath := filepath.Join(t.TempDir(), "config.hcl")
	if err := os.WriteFile(configPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, config)
	if !bytes.Equal(candidate, want) || len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("variable order = %s; changes = %v; warnings = %v; want %s", candidate, changes, warnings, want)
	}
}

func TestTerraformOrderComposesOuterAndNestedMoves(t *testing.T) {
	source := []byte(`terraform {
	cloud {
		token = "dummy-token"
		workspaces {
			project = "example-project"
			name = "example"
		}
		organization = "example"
	}
	required_providers {
		aws = {
			version = "~> 5.0"
			source = "hashicorp/aws"
		}
	}
	experiments = [
		"future_alpha",
		# keep this expression comment
		"older_experiment",
	]
	provider_meta "example/first" {
		unknown_value = local.metadata # expression comment
		payload = <<-EOT
		  provider-defined data
		EOT
	}
	required_version = ">= 1.0"
}`)
	want := []byte(`terraform {
	required_version = ">= 1.0"
	cloud {
		organization = "example"
		token = "dummy-token"
		workspaces {
			name = "example"
			project = "example-project"
		}
	}
	required_providers {
		aws = {
			source = "hashicorp/aws"
			version = "~> 5.0"
		}
	}
	experiments = [
		"future_alpha",
		# keep this expression comment
		"older_experiment",
	]
	provider_meta "example/first" {
		unknown_value = local.metadata # expression comment
		payload = <<-EOT
		  provider-defined data
		EOT
	}
}`)
	config := Config{
		RequiredVersionOrder:   true,
		BackendOrder:           true,
		CloudOrder:             true,
		CloudWorkspacesOrder:   true,
		RequiredProvidersOrder: true,
	}

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, changes, transformDiagnostics := transformTerraformOrderWithChanges(parsed, config)
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformTerraformOrderWithChanges() diagnostics = %v, want none", transformDiagnostics)
	}
	wantRules := []string{"required_version_order", "cloud_order", "cloud_workspaces_order", "required_providers_order"}
	if len(changes) != len(wantRules) {
		t.Fatalf("transformTerraformOrderWithChanges() changes = %v, want rules %v", changes, wantRules)
	}
	for index, wantRule := range wantRules {
		if changes[index].Rule != wantRule {
			t.Fatalf("transformTerraformOrderWithChanges() changes[%d].Rule = %q, want %q", index, changes[index].Rule, wantRule)
		}
	}
	if !bytes.Equal(candidate, want) {
		t.Fatalf("transformTerraformOrderWithChanges() =\n%s\nwant:\n%s", candidate, want)
	}

	parsedCandidate, diagnostics := parseHCLSource("candidate.tf", candidate)
	if diagnostics.HasErrors() {
		t.Fatalf("candidate parse diagnostics = %s, want no errors", diagnostics.Error())
	}
	second, secondChanges, secondDiagnostics := transformTerraformOrderWithChanges(parsedCandidate, config)
	if len(secondDiagnostics) != 0 || len(secondChanges) != 0 || !bytes.Equal(second, candidate) {
		t.Fatalf("second transform =\n%s\nchanges = %v, diagnostics = %v; want unchanged candidate", second, secondChanges, secondDiagnostics)
	}

	crlfSource := bytes.ReplaceAll(source, []byte("\n"), []byte("\r\n"))
	crlfWant := bytes.ReplaceAll(want, []byte("\n"), []byte("\r\n"))
	crlfSource = bytes.ReplaceAll(crlfSource, []byte("keep this expression comment"), []byte("keep this expression comment: 日本語保持"))
	crlfWant = bytes.ReplaceAll(crlfWant, []byte("keep this expression comment"), []byte("keep this expression comment: 日本語保持"))
	parsedCRLF, diagnostics := parseHCLSource("main.tf", crlfSource)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() CRLF diagnostics = %s, want no errors", diagnostics.Error())
	}
	crlfCandidate, _, crlfDiagnostics := transformTerraformOrderWithChanges(parsedCRLF, config)
	if len(crlfDiagnostics) != 0 || !bytes.Equal(crlfCandidate, crlfWant) {
		t.Fatalf("CRLF composition =\n%q\ndiagnostics = %v\nwant:\n%q", crlfCandidate, crlfDiagnostics, crlfWant)
	}
	parsedCRLFCandidate, diagnostics := parseHCLSource("candidate.tf", crlfCandidate)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() CRLF candidate diagnostics = %s, want no errors", diagnostics.Error())
	}
	crlfSecond, crlfChanges, crlfSecondDiagnostics := transformTerraformOrderWithChanges(parsedCRLFCandidate, config)
	if len(crlfSecondDiagnostics) != 0 || len(crlfChanges) != 0 || !bytes.Equal(crlfSecond, crlfCandidate) {
		t.Fatalf("second CRLF transform =\n%q\nchanges = %v, diagnostics = %v; want unchanged candidate", crlfSecond, crlfChanges, crlfSecondDiagnostics)
	}
}

func TestBackendOrderMovesCloudBlockAfterRequiredVersion(t *testing.T) {
	source := []byte(`terraform {
  cloud {
    organization = "example"
  }
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
  cloud {
    organization = "example"
  }
}`)
	if !bytes.Equal(candidate, want) {
		t.Errorf("transformBackendOrder() =\n%s\nwant:\n%s", candidate, want)
	}
}

func TestBackendOrderSkipsMutuallyExclusiveBackendAndCloudWhenVersionMissing(t *testing.T) {
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
	if len(transformDiagnostics) != 1 {
		t.Fatalf("transformBackendOrder() diagnostics = %v, want exactly one warning", transformDiagnostics)
	}
	if !bytes.Equal(candidate, source) {
		t.Errorf("transformBackendOrder() changed invalid input:\n%s\nwant unchanged:\n%s", candidate, source)
	}
}

func TestBackendOrderHandlesFixture(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "backend-order", "simple.input.tf"))
	if err != nil {
		t.Fatalf("read input fixture: %v", err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "backend-order", "simple.want.tf"))
	if err != nil {
		t.Fatalf("read expected fixture: %v", err)
	}

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, transformDiagnostics := transformBackendOrder(parsed, Config{RequiredVersionOrder: true, BackendOrder: true})
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformBackendOrder() diagnostics = %v, want none", transformDiagnostics)
	}
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

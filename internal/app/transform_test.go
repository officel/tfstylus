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

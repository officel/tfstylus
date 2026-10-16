package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderOrderMovesAliasRegionAndBlocks(t *testing.T) {
	source := []byte(`provider "example" {
  region = "west"
  endpoint = "https://example.invalid"
  alias = "secondary"
  version = "~> 1.0"
  default_tags {
    tags = { team = "platform" }
  }
  assume_role {
    role_arn = "arn:example:role"
  }
}`)
	want := []byte(`provider "example" {
  endpoint = "https://example.invalid"
  version = "~> 1.0"
  alias = "secondary"
  region = "west"
  default_tags {
    tags = { team = "platform" }
  }
  assume_role {
    role_arn = "arn:example:role"
  }
}`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, changes, transformDiagnostics := transformTerraformOrderWithChanges(parsed, Config{ProviderOrder: true})
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformTerraformOrderWithChanges() diagnostics = %v, want none", transformDiagnostics)
	}
	if len(changes) != 1 || changes[0].Rule != "provider_order" {
		t.Fatalf("transformTerraformOrderWithChanges() changes = %v, want one provider_order change", changes)
	}
	if !bytes.Equal(candidate, want) {
		t.Fatalf("transformTerraformOrderWithChanges() =\n%s\nwant:\n%s", candidate, want)
	}
}

func TestProviderOrderPreservesCommentsCRLFAndIsIdempotent(t *testing.T) {
	source := []byte(`provider "example" {
  region = "west"
  endpoint = "https://example.invalid"
  # version constraint
  version = "~> 1.0"
  alias = "secondary" # alias comment
  default_tags {
    # nested comment
    tags = { team = "platform" }
  }
}`)
	want := []byte(`provider "example" {
  endpoint = "https://example.invalid"
  # version constraint
  version = "~> 1.0"
  alias = "secondary" # alias comment
  region = "west"
  default_tags {
    # nested comment
    tags = { team = "platform" }
  }
}`)
	source = bytes.ReplaceAll(source, []byte("\n"), []byte("\r\n"))
	want = bytes.ReplaceAll(want, []byte("\n"), []byte("\r\n"))

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}
	candidate, changes, transformDiagnostics := transformTerraformOrderWithChanges(parsed, Config{ProviderOrder: true})
	if len(transformDiagnostics) != 0 || len(changes) != 1 {
		t.Fatalf("transformTerraformOrderWithChanges() changes = %v, diagnostics = %v", changes, transformDiagnostics)
	}
	if !bytes.Equal(candidate, want) {
		t.Fatalf("transformTerraformOrderWithChanges() =\n%q\nwant:\n%q", candidate, want)
	}

	parsedCandidate, diagnostics := parseHCLSource("main.tf", candidate)
	if diagnostics.HasErrors() {
		t.Fatalf("candidate parse diagnostics = %s, want no errors", diagnostics.Error())
	}
	second, changes, transformDiagnostics := transformTerraformOrderWithChanges(parsedCandidate, Config{ProviderOrder: true})
	if len(transformDiagnostics) != 0 || len(changes) != 0 || !bytes.Equal(second, candidate) {
		t.Fatalf("second transform changes = %v, diagnostics = %v, candidate = %q", changes, transformDiagnostics, second)
	}
}

func TestProviderOrderSkipsAmbiguousCommentAttachment(t *testing.T) {
	source := []byte(`provider "example" {
  region = "west"
  alias = "secondary"

  # unattached comment
}`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}

	candidate, changes, transformDiagnostics := transformTerraformOrderWithChanges(parsed, Config{ProviderOrder: true})
	if len(changes) != 0 || len(transformDiagnostics) != 1 {
		t.Fatalf("transformTerraformOrderWithChanges() changes = %v, diagnostics = %v, want unchanged input and one diagnostic", changes, transformDiagnostics)
	}
	if !bytes.Equal(candidate, source) {
		t.Fatalf("transformTerraformOrderWithChanges() changed ambiguous input:\n%s", candidate)
	}
}

func TestProviderOrderDisabledByConfig(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := []byte("provider \"example\" {\n  region = \"west\"\n  endpoint = \"https://example.invalid\"\n  alias = \"secondary\"\n}\n")
	if err := os.WriteFile(filename, source, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, ".tfstylus.hcl"), []byte("provider_order = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{"check", "run"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("%s with provider_order disabled = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || !bytes.Equal(got, source) {
			t.Fatalf("%s changed input while provider_order was disabled: %s; %v", command, got, err)
		}
	}
}

func TestProviderOrderCLI(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "main.tf")
	source := []byte("provider \"example\" {\n  region = \"west\"\n  endpoint = \"https://example.invalid\"\n  alias = \"secondary\"\n}\n")
	want := []byte("provider \"example\" {\n  endpoint = \"https://example.invalid\"\n  alias = \"secondary\"\n  region = \"west\"\n}\n")
	if err := os.WriteFile(filename, source, 0o600); err != nil {
		t.Fatal(err)
	}

	var report string
	for _, diff := range []bool{false, true} {
		args := []string{"check", "--chdir", directory}
		if diff {
			args = append(args, "--diff")
		}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "main.tf:1:1: provider_order:") {
			t.Fatalf("check = %v; output = %s", err, output.String())
		}
		if diff {
			if !strings.HasPrefix(output.String(), report) || !strings.Contains(output.String(), "--- a/main.tf") {
				t.Fatalf("missing report/diff: %s", output.String())
			}
		} else {
			report = output.String()
		}
		if got, err := os.ReadFile(filename); err != nil || !bytes.Equal(got, source) {
			t.Fatalf("check modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"run", "run", "check"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
}

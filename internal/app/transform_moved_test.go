package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMovedOrderHandlesFixture(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "moved-order", "basic.input.tf"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "moved-order", "basic.want.tf"))
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.hcl")
	writeConfig(t, configPath, "")
	config, err := loadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, config)
	if len(warnings) != 0 || len(changes) != 1 || changes[0].Rule != "moved_order" || !bytes.Equal(candidate, want) {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestMovedOrderKeepsUnknownBlocksWithoutValidatingLabels(t *testing.T) {
	source := []byte(`moved "extra" {
  future_block {
    value = true
  }
  to = module.new
  from = module.old
}
`)
	want := []byte(`moved "extra" {
  from = module.old
  to = module.new
  future_block {
    value = true
  }
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{MovedOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestMovedOrderPreservesCRLFAndIsIdempotent(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "moved-order", "basic.input.tf"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "moved-order", "basic.want.tf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, newline := range []string{"\n", "\r\n"} {
		input := bytes.ReplaceAll(source, []byte("\n"), []byte(newline))
		expected := bytes.ReplaceAll(want, []byte("\n"), []byte(newline))
		for iteration := 0; iteration < 2; iteration++ {
			parsed, diagnostics := parseHCLSource("main.tf", input)
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{MovedOrder: true})
			if len(warnings) != 0 || len(changes) != 1-iteration || !bytes.Equal(candidate, expected) {
				t.Fatalf("iteration %d: candidate = %q; changes = %v; warnings = %v; want %q", iteration, candidate, changes, warnings, expected)
			}
			input = candidate
		}
	}
}

func TestMovedOrderPreservesUnknownAttributesAndBlockOrder(t *testing.T) {
	source := []byte("moved {\n  zeta = [local.keep] # keep expression\n  to = module.new\n  alpha = { value = local.keep }\n  from = module.old\n}\n# between blocks\nlocals { keep = \"same\" }\nmoved {\n  to = aws_instance.new\n  from = aws_instance.old\n}\n")
	want := []byte("moved {\n  from = module.old\n  to = module.new\n  zeta = [local.keep] # keep expression\n  alpha = { value = local.keep }\n}\n# between blocks\nlocals { keep = \"same\" }\nmoved {\n  from = aws_instance.old\n  to = aws_instance.new\n}\n")
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{MovedOrder: true})
	if len(warnings) != 0 || len(changes) != 2 || !bytes.Equal(candidate, want) {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestMovedOrderSafetyAndDisabledRule(t *testing.T) {
	for _, test := range []struct {
		name    string
		source  string
		warning string
	}{
		{name: "ambiguous comment", source: "moved {\n  to = module.new\n\n  # separate section\n\n  from = module.old\n}\n", warning: "ambiguous comment"},
		{name: "nested block needs no move", source: "moved {\n  from = module.old\n  to = module.new\n  future {}\n}\n"},
		{name: "last trailing comment", source: "moved {\n  to = module.new\n  from = module.old # source\n}\n", warning: "ambiguous boundaries"},
		{name: "single-line needs no move", source: "moved { to = module.new }\n"},
		{name: "missing from is not validated", source: "moved {\n  to = module.new\n}\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := []byte(test.source)
			parsed, diagnostics := parseHCLSource("main.tf", source)
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			for _, enabled := range []bool{true, false} {
				candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{MovedOrder: enabled})
				if !bytes.Equal(candidate, source) || len(changes) != 0 {
					t.Fatalf("enabled = %t: candidate = %s; changes = %v, want unchanged", enabled, candidate, changes)
				}
				if enabled && test.warning != "" {
					if len(warnings) != 1 || !bytes.Contains([]byte(warnings[0].Reason), []byte(test.warning)) {
						t.Fatalf("warnings = %v, want %q", warnings, test.warning)
					}
				} else if len(warnings) != 0 {
					t.Fatalf("warnings = %v, want none", warnings)
				}
			}
		})
	}
}

func TestMovedOrderCandidateValidationRejectsUnrelatedChanges(t *testing.T) {
	source := []byte("locals { keep = \"same\" }\nmoved {\n  to = module.new\n  from = module.old\n}\n")
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformMovedOrderWithChanges(parsed)
	if len(warnings) != 0 || len(changes) != 1 {
		t.Fatalf("changes = %v; warnings = %v", changes, warnings)
	}
	for _, corrupted := range [][]byte{
		bytes.ReplaceAll(candidate, []byte("same"), []byte("diff")),
		bytes.ReplaceAll(candidate, []byte("module.old"), []byte("module.bad")),
	} {
		if validateMovedCandidate(parsed, corrupted) == nil {
			t.Fatalf("accepted changed candidate: %s", corrupted)
		}
	}
}

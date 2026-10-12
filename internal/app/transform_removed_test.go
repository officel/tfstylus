package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestRemovedOrderPreservesCRLFAndIsIdempotent(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "removed-order", "basic.input.tf"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "removed-order", "basic.want.tf"))
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
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{RemovedOrder: true})
			if len(warnings) != 0 || len(changes) != 1-iteration || !bytes.Equal(candidate, expected) {
				t.Fatalf("newline %q, iteration %d: candidate = %q; changes = %v; warnings = %v; want %q", newline, iteration, candidate, changes, warnings, expected)
			}
			input = candidate
		}
	}
}

func TestRemovedOrderSafetyAndDisabledRule(t *testing.T) {
	for _, test := range []struct {
		name    string
		source  string
		warning string
	}{
		{name: "ambiguous comment", source: "removed {\n  lifecycle { destroy = false }\n\n  # separate section\n\n  from = module.old\n}\n", warning: "ambiguous comment"},
		{name: "last trailing comment", source: "removed {\n  lifecycle { destroy = false }\n  from = module.old # source\n}\n", warning: "ambiguous boundaries"},
		{name: "single-line needs no move", source: "removed { from = module.old }\n"},
		{name: "missing from is not validated", source: "removed {\n  lifecycle { destroy = false }\n}\n"},
		{name: "sorted trailing comment", source: "removed {\n  from = module.old\n  lifecycle { destroy = false } # keep\n}\n"},
		{name: "same-rank order unchanged", source: "removed {\n  from = module.old\n  lifecycle { destroy = true }\n  provisioner \"local-exec\" {\n    command = \"echo goodbye\"\n    when = destroy\n  }\n  connection { host = self.public_ip }\n}\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := []byte(test.source)
			parsed, diagnostics := parseHCLSource("main.tf", source)
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			for _, enabled := range []bool{true, false} {
				candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{RemovedOrder: enabled})
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

func TestRemovedOrderPreservesLifecycleAndBlockOrder(t *testing.T) {
	lifecycle := "  lifecycle {\n    future = { z = local.keep, a = true }\n    destroy = false\n  }\n"
	first := "removed {\n" + lifecycle + "  from = module.old\n}\n"
	second := "removed {\n  lifecycle { destroy = true }\n  from = aws_instance.old\n}\n"
	source := []byte(first + "# between blocks\nlocals { keep = \"same\" }\n" + second)
	want := []byte("removed {\n  from = module.old\n" + lifecycle + "}\n# between blocks\nlocals { keep = \"same\" }\nremoved {\n  from = aws_instance.old\n  lifecycle { destroy = true }\n}\n")
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{RemovedOrder: true})
	if len(warnings) != 0 || len(changes) != 2 || !bytes.Equal(candidate, want) {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestRemovedOrderCandidateValidationRejectsUnrelatedChanges(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "removed-order", "basic.input.tf"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformRemovedOrderWithChanges(parsed)
	if len(warnings) != 0 || len(changes) != 1 {
		t.Fatalf("changes = %v; warnings = %v", changes, warnings)
	}
	parsedCandidate, diagnostics := parseHCLSource("main.tf", candidate)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	var reordered []byte
	for _, block := range parsedCandidate.body.(*hclsyntax.Body).Blocks {
		if block.Type != "removed" {
			continue
		}
		for _, nested := range block.Body.Blocks {
			if nested.Type != "provisioner" || len(nested.Labels) != 1 || nested.Labels[0] != "remote-exec" {
				continue
			}
			chunks, diagnostic := bodyElementChunks(parsedCandidate, nested, syntaxElements(nested.Body), "provisioner")
			if diagnostic != nil || len(chunks) != 2 {
				t.Fatalf("expected two provisioner attribute chunks: %v; diagnostic = %v", chunks, diagnostic)
			}
			reordered = bytes.Join([][]byte{
				candidate[:chunks[0].start], chunks[1].text, chunks[0].text, candidate[chunks[1].end:],
			}, nil)
		}
	}
	if reordered == nil {
		t.Fatal("remote-exec provisioner not found in candidate")
	}
	for _, corrupted := range [][]byte{
		bytes.ReplaceAll(candidate, []byte("same"), []byte("diff")),
		bytes.ReplaceAll(candidate, []byte("aws_instance.old"), []byte("aws_instance.bad")),
		bytes.ReplaceAll(candidate, []byte("# first provisioner"), []byte("# wrong provisioner")),
		bytes.ReplaceAll(candidate, []byte("echo goodbye"), []byte("echo changed")),
		bytes.ReplaceAll(candidate, []byte("user = \"root\""), []byte("user = \"user\"")),
		bytes.ReplaceAll(candidate, []byte("destroy = true"), []byte("destroy = null")),
		reordered,
		bytes.ReplaceAll(candidate, []byte("  # source address\n  from = aws_instance.old\n  /* removal policy */\n"), []byte("  /* removal policy */\n  from = aws_instance.old\n  # source address\n")),
	} {
		if bytes.Equal(candidate, corrupted) {
			t.Fatal("corruption did not change candidate")
		}
		if validateRemovedCandidate(parsed, corrupted) == nil {
			t.Fatalf("accepted changed candidate: %s", corrupted)
		}
	}
}

func TestRemovedOrderHandlesFixture(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "removed-order", "basic.input.tf"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "removed-order", "basic.want.tf"))
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
	if len(warnings) != 0 || len(changes) != 1 || changes[0].Rule != "removed_order" || !bytes.Equal(candidate, want) {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestRemovedOrderKeepsUnknownElementsWithoutValidatingLabels(t *testing.T) {
	source := []byte(`removed "extra" {
	future_block {
		value = true
	}
	lifecycle {
		destroy = false
	}
	future_attribute = "kept"
	from = module.old
}
`)
	want := []byte(`removed "extra" {
	from = module.old
	future_attribute = "kept"
	lifecycle {
		destroy = false
	}
	future_block {
		value = true
	}
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{RemovedOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

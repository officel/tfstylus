package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestListOrderGroupsAndPreservesSource(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("../../testdata/list-order", "input.tfquery.hcl"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("../../testdata/list-order", "want.tfquery.hcl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, newline := range []string{"\n", "\r\n"} {
		for _, trailingNewline := range []bool{true, false} {
			input := bytes.ReplaceAll(source, []byte("\n"), []byte(newline))
			expected := bytes.ReplaceAll(want, []byte("\n"), []byte(newline))
			if !trailingNewline {
				input = bytes.TrimSuffix(input, []byte(newline))
				expected = bytes.TrimSuffix(expected, []byte(newline))
			}
			parsed, diagnostics := parseHCLSource("query.tfquery.hcl", input)
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			candidate, changes, warnings := transformListOrderWithChanges(parsed)
			if !bytes.Equal(candidate, expected) || len(changes) != 1 || changes[0].Rule != "list_order" || len(warnings) != 0 {
				t.Fatalf("newline %q, trailing %t: candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", newline, trailingNewline, candidate, changes, warnings, expected)
			}
			corrupted := bytes.Replace(candidate, []byte(`name = "web"`), []byte(`name = "api"`), 1)
			if bytes.Equal(candidate, corrupted) || validateListCandidate(parsed, corrupted) == nil {
				t.Fatalf("accepted candidate with changed config content: %s", corrupted)
			}
			parsed, diagnostics = parseHCLSource("query.tfquery.hcl", candidate)
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			candidate, changes, warnings = transformListOrderWithChanges(parsed)
			if !bytes.Equal(candidate, expected) || len(changes) != 0 || len(warnings) != 0 {
				t.Fatalf("second pass, newline %q, trailing %t: candidate =\n%s\nchanges = %v; warnings = %v", newline, trailingNewline, candidate, changes, warnings)
			}
		}
	}
}

func TestListOrderSafety(t *testing.T) {
	for _, test := range []struct {
		name    string
		source  string
		warning string
	}{
		{name: "ambiguous comment", source: "list \"aws_instance\" \"one\" {\n  provider = aws\n\n  # section\n\n  count = 1\n}\n", warning: "ambiguous comment"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed, diagnostics := parseHCLSource("query.tfquery.hcl", []byte(test.source))
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			candidate, changes, warnings := transformListOrderWithChanges(parsed)
			if !bytes.Equal(candidate, []byte(test.source)) || len(changes) != 0 || len(warnings) != 1 || warnings[0].Reason == "" ||
				!bytes.Contains([]byte(warnings[0].Reason), []byte(test.warning)) {
				t.Fatalf("candidate = %s; changes = %v; warnings = %v, want unchanged and warning %q", candidate, changes, warnings, test.warning)
			}
		})
	}
}

func TestListOrderMutuallyExclusiveArgumentsKeepRelativeOrder(t *testing.T) {
	source := []byte("list \"aws_instance\" \"one\" {\n  provider = aws\n  for_each = toset([\"a\"])\n  count = 1\n}\n")
	want := []byte("list \"aws_instance\" \"one\" {\n  for_each = toset([\"a\"])\n  count = 1\n  provider = aws\n}\n")
	parsed, diagnostics := parseHCLSource("query.tfquery.hcl", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformListOrderWithChanges(parsed)
	if !bytes.Equal(candidate, want) || len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("candidate = %s; changes = %v; warnings = %v; want simple ordered output", candidate, changes, warnings)
	}
}

func TestListOrderKeepsUnknownBlocksWithoutValidatingLabels(t *testing.T) {
	source := []byte(`list {
	config {}
	future_block {
		value = true
	}
	provider = aws
	count = 1
}
`)
	want := []byte(`list {
	count = 1
	provider = aws
	config {}
	future_block {
		value = true
	}
}
`)
	parsed, diagnostics := parseHCLSource("query.tfquery.hcl", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformListOrderWithChanges(parsed)
	if !bytes.Equal(candidate, want) || len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

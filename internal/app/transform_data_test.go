package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestDataOrderGroups(t *testing.T) {
	source := []byte(`data "example" "one" {
  depends_on = [local.first, local.second]
  lifecycle {
    postcondition {
      condition = true
      error_message = "post"
    }
    precondition {
      condition = true
      error_message = "pre"
    }
  }
  dynamic "label" {
    for_each = ["name"]
    content { value = label.value }
  }
  filter {
    name = "kind"
    values = ["example"]
  }
  provider = example.west
  first = var.first
  count = 2
  for_each = { item = "value" }
  second = var.second
}
`)
	want := []byte(`data "example" "one" {
  count = 2
  for_each = { item = "value" }
  provider = example.west
  first = var.first
  second = var.second
  dynamic "label" {
    for_each = ["name"]
    content { value = label.value }
  }
  filter {
    name = "kind"
    values = ["example"]
  }
  lifecycle {
    precondition {
      condition = true
      error_message = "pre"
    }
    postcondition {
      condition = true
      error_message = "post"
    }
  }
  depends_on = [local.first, local.second]
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{DataOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || changes[0].Rule != "data_order" || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestDataOrderMultipleBlocksAndSingleMetaArguments(t *testing.T) {
	source := []byte(`data "example" "first" {
  depends_on = [local.first]
  count = 1
}
data "example" "second" {
  id = "second"
  depends_on = [local.second]
  for_each = local.items
}
`)
	want := []byte(`data "example" "first" {
  count = 1
  depends_on = [local.first]
}
data "example" "second" {
  for_each = local.items
  id = "second"
  depends_on = [local.second]
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{DataOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 2 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestDataOrderWithTopLevelAttributeBeforeBlock(t *testing.T) {
	source := []byte("terraform_version = \"1.0\"\ndata \"example\" \"one\" {\n  depends_on = [local.first]\n  count = 1\n}\n")
	want := []byte("terraform_version = \"1.0\"\ndata \"example\" \"one\" {\n  count = 1\n  depends_on = [local.first]\n}\n")
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{DataOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestSourceElementChunksPreserveCommentOwnershipAndBytes(t *testing.T) {
	source := []byte(`data "example" "one" {
  # count owner
  count = 1 # count trailing
  # id owner
  id = "example"
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}
	root := parsed.body.(*hclsyntax.Body)
	block := root.Blocks[0]
	chunks, diagnostic := sourceElementChunks(parsed, block, syntaxElements(block.Body), "data")
	if diagnostic != nil {
		t.Fatalf("sourceElementChunks() diagnostic = %v, want none", diagnostic)
	}
	want := [][]byte{
		[]byte("  # count owner\n  count = 1 # count trailing\n"),
		[]byte("  # id owner\n  id = \"example\"\n"),
	}
	if len(chunks) != len(want) {
		t.Fatalf("sourceElementChunks() returned %d chunks, want %d", len(chunks), len(want))
	}
	for index, chunk := range chunks {
		if !bytes.Equal(chunk.text, want[index]) || !bytes.Equal(chunk.text, parsed.source[chunk.start:chunk.end]) {
			t.Fatalf("chunk %d = %q (range %d:%d), want %q and exact source bytes", index, chunk.text, chunk.start, chunk.end, want[index])
		}
	}
}

func TestDataOrderPreservesCRLFUTF8AndEOF(t *testing.T) {
	source := []byte(`data "example" "one" {
  depends_on = [local.first, local.second] # dependency
  # 検索 ID
  id = <<-EOT
    # heredoc text
    example-id
  EOT
  count = 1
}`)
	want := []byte(`data "example" "one" {
  count = 1
  # 検索 ID
  id = <<-EOT
    # heredoc text
    example-id
  EOT
  depends_on = [local.first, local.second] # dependency
}`)
	for _, newline := range []string{"\n", "\r\n"} {
		input := bytes.ReplaceAll(source, []byte("\n"), []byte(newline))
		expected := bytes.ReplaceAll(want, []byte("\n"), []byte(newline))
		parsed, diagnostics := parseHCLSource("main.tf", input)
		if diagnostics.HasErrors() {
			t.Fatal(diagnostics.Error())
		}
		candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{DataOrder: true})
		if !bytes.Equal(candidate, expected) || len(changes) != 1 || len(warnings) != 0 {
			t.Fatalf("candidate = %q; changes = %v; warnings = %v; want %q", candidate, changes, warnings, expected)
		}
		parsedCandidate, diagnostics := parseHCLSource("main.tf", candidate)
		if diagnostics.HasErrors() {
			t.Fatal(diagnostics.Error())
		}
		second, secondChanges, secondWarnings := transformTerraformOrderWithChanges(parsedCandidate, Config{DataOrder: true})
		if !bytes.Equal(second, candidate) || len(secondChanges) != 0 || len(secondWarnings) != 0 {
			t.Fatalf("not idempotent: %q; %v; %v", second, secondChanges, secondWarnings)
		}
	}
}

func TestDataLifecycleConditionOrder(t *testing.T) {
	source := []byte(`data "example" "one" {
  lifecycle {
    # first postcondition
    postcondition {
      condition = true
      error_message = <<-EOT
        first postcondition
      EOT
    }
    precondition {
      condition = true
      error_message = "precondition"
    }
    postcondition {
      condition = false
      error_message = "second postcondition"
    }
  }
}
`)
	want := []byte(`data "example" "one" {
  lifecycle {
    precondition {
      condition = true
      error_message = "precondition"
    }
    # first postcondition
    postcondition {
      condition = true
      error_message = <<-EOT
        first postcondition
      EOT
    }
    postcondition {
      condition = false
      error_message = "second postcondition"
    }
  }
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{DataOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || changes[0].Rule != "data_order" || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestDataLifecycleKeepsUnknownBlocksWithoutValidatingLabels(t *testing.T) {
	source := []byte(`data "example" "one" {
  lifecycle {
    postcondition {
      condition = true
      error_message = "post"
    }
    future_block {}
    precondition "extra" {
      condition = true
      error_message = "pre"
    }
  }
}
`)
	want := []byte(`data "example" "one" {
  lifecycle {
    precondition "extra" {
      condition = true
      error_message = "pre"
    }
    postcondition {
      condition = true
      error_message = "post"
    }
    future_block {}
  }
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{DataOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestDataOrderBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		enabled bool
		warning string
	}{
		{name: "empty", source: "data \"example\" \"one\" {}\n", enabled: true},
		{name: "single element", source: "data \"example\" \"one\" { count = 1 }\n", enabled: true},
		{name: "sorted boundary comment", source: "data \"example\" \"one\" {\n  count = 1\n\n  # section\n\n  id = \"example\"\n}\n", enabled: true},
		{name: "ambiguous comment", source: "data \"example\" \"one\" {\n  depends_on = [local.first]\n\n  # section\n\n  count = 1\n}\n", enabled: true, warning: "ambiguous comment"},
		{name: "disabled", source: "data \"example\" \"one\" {\n  depends_on = [local.first]\n  count = 1\n}\n", enabled: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, parseDiagnostics := parseHCLSource("main.tf", []byte(test.source))
			if parseDiagnostics.HasErrors() {
				t.Fatal(parseDiagnostics.Error())
			}
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{DataOrder: test.enabled})
			if string(candidate) != test.source {
				t.Fatalf("changed preserved input:\n%s", candidate)
			}
			if len(changes) != 0 {
				t.Fatalf("changes = %v, want none", changes)
			}
			if test.warning == "" {
				if len(warnings) != 0 {
					t.Fatalf("warnings = %v, want none", warnings)
				}
			} else if len(warnings) != 1 || !strings.Contains(warnings[0].Reason, test.warning) || warnings[0].Subject.Start.Line < 1 {
				t.Fatalf("warnings = %v; want %q at input location", warnings, test.warning)
			}
		})
	}
}

func TestValidateDataCandidateRejectsUnpermittedChanges(t *testing.T) {
	source := []byte(`# header
data "example" "one" {
  # dependency
  depends_on = [local.first]
  lifecycle {
    postcondition {
      condition = true
      error_message = "post"
    }
    precondition {
      condition = true
      error_message = "pre"
    }
  }
  # count
  count = 1
}
locals { untouched = 1 }
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	valid, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{DataOrder: true})
	if len(changes) != 1 || len(warnings) != 0 {
		t.Fatalf("transform changes = %v; warnings = %v", changes, warnings)
	}
	if diagnostic := validateDataCandidate(parsed, valid); diagnostic != nil {
		t.Fatalf("validateDataCandidate() = %v for valid candidate", diagnostic)
	}
	for name, mutated := range map[string][]byte{
		"expression":        []byte(strings.Replace(string(valid), "count = 1", "count = 2", 1)),
		"comment ownership": []byte(strings.Replace(string(valid), "# count", "# changed", 1)),
		"condition order":   []byte(strings.Replace(string(valid), "precondition {", "postcondition {", 1)),
		"outside block":     []byte(strings.Replace(string(valid), "untouched = 1", "untouched = 2", 1)),
		"label":             []byte(strings.Replace(string(valid), `data "example"`, `data "changed"`, 1)),
		"syntax":            append(bytes.Clone(valid), []byte("}\n")...),
	} {
		t.Run(name, func(t *testing.T) {
			if diagnostic := validateDataCandidate(parsed, mutated); diagnostic == nil {
				t.Fatal("validateDataCandidate() = nil, want preservation diagnostic")
			}
		})
	}
}

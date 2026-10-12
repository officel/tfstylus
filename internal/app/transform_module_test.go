package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestModuleOrderGroups(t *testing.T) {
	source, err := os.ReadFile("../../testdata/module-order/input.tf")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/module-order/want.tf")
	if err != nil {
		t.Fatal(err)
	}
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	config, err := loadConfigForModuleTest(t, "")
	if err != nil {
		t.Fatal(err)
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, config)
	if !bytes.Equal(candidate, want) || len(changes) != 1 || changes[0].Rule != "module_order" || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func loadConfigForModuleTest(t *testing.T, content string) (Config, error) {
	t.Helper()
	filename := filepath.Join(t.TempDir(), ".tfstylus.hcl")
	writeConfig(t, filename, content)
	return loadConfig(filename)
}

func TestModuleOrderPreservesBytesAndIsIdempotent(t *testing.T) {
	dependency := "  depends_on = [terraform_data.second, terraform_data.first] # dependency\n"
	providers := "  # provider selection\n  providers = {\n    // destination\n    aws.dst = aws.west\n    aws.src = aws.east # source\n  }\n"
	inputs := "  /* input description */\n  second = {\n    z = var.second # inner comment\n    a = \"日本語\"\n  }\n  first = <<-EOT\n    # heredoc text\n    ${var.first}\n  EOT\n"
	version := "  // registry version\n  version = \"~> 1.0\" # constraint\n"
	sourceAttribute := "  # module source\n  source = \"example/network/aws\"\n"
	meta := "  for_each = {\n    z = var.second\n    a = var.first\n  }\n  count = 1\n"
	deprecated := "  ignore_nested_deprecations = true\n"
	header := "# header\n\nlocals { keep = \"same\" }\n\nmodule \"example\" {\n"
	footer := "}\n\n# footer\n"
	source := []byte(header + dependency + providers + inputs + deprecated + version + meta + sourceAttribute + footer)
	want := []byte(header + meta + sourceAttribute + version + inputs + providers + deprecated + dependency + footer)
	for _, newline := range []string{"\n", "\r\n"} {
		for _, trailingNewline := range []bool{true, false} {
			input := bytes.ReplaceAll(source, []byte("\n"), []byte(newline))
			expected := bytes.ReplaceAll(want, []byte("\n"), []byte(newline))
			if !trailingNewline {
				input = bytes.TrimSuffix(input, []byte(newline))
				expected = bytes.TrimSuffix(expected, []byte(newline))
			}
			for iteration := 0; iteration < 2; iteration++ {
				parsed, diagnostics := parseHCLSource("main.tf", input)
				if diagnostics.HasErrors() {
					t.Fatal(diagnostics.Error())
				}
				candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ModuleOrder: true})
				if !bytes.Equal(candidate, expected) || len(changes) != 1-iteration || len(warnings) != 0 {
					t.Fatalf("newline %q, trailing %t, iteration %d: candidate = %q; changes = %v; warnings = %v; want %q", newline, trailingNewline, iteration, candidate, changes, warnings, expected)
				}
				input = candidate
			}
		}
	}
}

func TestModuleOrderSafetyAndDisabledRule(t *testing.T) {
	for _, test := range []struct {
		name    string
		source  string
		warning string
	}{
		{name: "missing label", source: "module {\n  version = \"1.0\"\n  source = \"example/network/aws\"\n}\n", warning: "must have one label"},
		{name: "extra label", source: "module \"one\" \"two\" {\n  version = \"1.0\"\n  source = \"example/network/aws\"\n}\n", warning: "must have one label"},
		{name: "ambiguous comment", source: "module \"one\" {\n  name = var.name\n\n  # section\n\n  source = \"./child\"\n}\n", warning: "ambiguous comment"},
		{name: "unknown nested block needs move", source: "module \"one\" {\n  name = var.name\n  future {}\n  source = \"./child\"\n}\n", warning: "unsupported nested block"},
		{name: "unknown nested block needs no move", source: "module \"one\" {\n  source = \"./child\"\n  future {}\n  name = var.name\n}\n"},
		{name: "sorted ambiguous boundary", source: "module \"one\" {\n  source = \"./child\"\n\n  # section\n\n  name = var.name\n}\n"},
		{name: "last trailing comment", source: "module \"one\" {\n  name = var.name\n  source = \"./child\" # source\n}\n", warning: "ambiguous boundaries"},
		{name: "empty", source: "module \"one\" {}\n"},
		{name: "single line", source: "module \"one\" { source = \"./child\" }\n"},
		{name: "missing source", source: "module \"one\" {\n  version = \"1.0\"\n  name = var.name\n}\n"},
		{name: "same-rank inputs", source: "module \"one\" {\n  z = var.z\n  a = var.a\n}\n"},
		{name: "mutually exclusive meta arguments", source: "module \"one\" {\n  for_each = toset([\"b\", \"a\"])\n  count = 1\n}\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := []byte(test.source)
			parsed, diagnostics := parseHCLSource("main.tf", input)
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			for _, enabled := range []bool{true, false} {
				candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ModuleOrder: enabled})
				if !bytes.Equal(candidate, input) || len(changes) != 0 {
					t.Fatalf("enabled %t: candidate = %s; changes = %v, want unchanged", enabled, candidate, changes)
				}
				if enabled && test.warning != "" {
					if len(warnings) != 1 || !strings.Contains(warnings[0].Reason, test.warning) {
						t.Fatalf("warnings = %v, want %q", warnings, test.warning)
					}
				} else if len(warnings) != 0 {
					t.Fatalf("warnings = %v, want none", warnings)
				}
			}
		})
	}
}

func TestModuleOrderOptionalAttributes(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{name: "local source without version", source: "module \"one\" {\n  name = var.name\n  source = \"./child\"\n}\n", want: "module \"one\" {\n  source = \"./child\"\n  name = var.name\n}\n"},
		{name: "missing source is not repaired", source: "module \"one\" {\n  name = var.name\n  version = local.constraint\n}\n", want: "module \"one\" {\n  version = local.constraint\n  name = var.name\n}\n"},
		{name: "set expression", source: "module \"one\" {\n  source = var.module_source\n  for_each = toset([\"z\", \"a\"])\n}\n", want: "module \"one\" {\n  for_each = toset([\"z\", \"a\"])\n  source = var.module_source\n}\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed, diagnostics := parseHCLSource("main.tf", []byte(test.source))
			if diagnostics.HasErrors() {
				t.Fatal(diagnostics.Error())
			}
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ModuleOrder: true})
			if string(candidate) != test.want || len(changes) != 1 || len(warnings) != 0 {
				t.Fatalf("candidate = %s; changes = %v; warnings = %v; want %s", candidate, changes, warnings, test.want)
			}
			candidate, changes, warnings = transformTerraformOrderWithChanges(parsed, Config{})
			if string(candidate) != test.source || len(changes) != 0 || len(warnings) != 0 {
				t.Fatalf("disabled rule: candidate = %s; changes = %v; warnings = %v", candidate, changes, warnings)
			}
		})
	}
}

func TestModuleOrderSkipsUnsafeBlockIndependently(t *testing.T) {
	valid := "module \"valid\" {\n  name = var.name\n  source = \"./child\"\n}\n"
	unsafe := "module \"unsafe\" {\n  name = var.name\n  future {}\n  source = \"./child\"\n}\n"
	separator := "# between modules\nlocals { keep = \"same\" }\n"
	source := []byte(valid + separator + unsafe + valid)
	wantModule := "module \"valid\" {\n  source = \"./child\"\n  name = var.name\n}\n"
	want := []byte(wantModule + separator + unsafe + wantModule)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ModuleOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 2 || len(warnings) != 1 {
		t.Fatalf("candidate = %s; changes = %v; warnings = %v; want %s", candidate, changes, warnings, want)
	}
}

func TestModuleOrderComposesWithOtherRules(t *testing.T) {
	source := []byte("module \"one\" {\n  name = var.name\n  source = \"./child\"\n}\ndata \"example\" \"one\" {\n  depends_on = [local.data]\n  count = 1\n}\nresource \"example\" \"one\" {\n  depends_on = [local.resource]\n  count = 1\n}\n")
	want := []byte("module \"one\" {\n  source = \"./child\"\n  name = var.name\n}\ndata \"example\" \"one\" {\n  count = 1\n  depends_on = [local.data]\n}\nresource \"example\" \"one\" {\n  count = 1\n  depends_on = [local.resource]\n}\n")
	for iteration := 0; iteration < 2; iteration++ {
		parsed, diagnostics := parseHCLSource("main.tf", source)
		if diagnostics.HasErrors() {
			t.Fatal(diagnostics.Error())
		}
		candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{DataOrder: true, ResourceOrder: true, ModuleOrder: true})
		if !bytes.Equal(candidate, want) || len(changes) != 3*(1-iteration) || len(warnings) != 0 {
			t.Fatalf("candidate = %s; changes = %v; warnings = %v; want %s", candidate, changes, warnings, want)
		}
		source = candidate
	}
}

func TestModuleOrderCandidateValidationRejectsUnrelatedChanges(t *testing.T) {
	source := []byte("locals { keep = \"same\" }\nmodule \"one\" {\n  # input\n  name = var.name\n  # provider\n  providers = { aws = aws.west, other = other.east }\n  source = \"./child\"\n}\n")
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	candidate, changes, warnings := transformModuleOrderWithChanges(parsed)
	if len(changes) != 1 || len(warnings) != 0 || validateModuleCandidate(parsed, candidate) != nil {
		t.Fatalf("candidate = %s; changes = %v; warnings = %v", candidate, changes, warnings)
	}
	parsedCandidate, diagnostics := parseHCLSource("main.tf", candidate)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	block := parsedCandidate.body.(*hclsyntax.Body).Blocks[1]
	chunks, diagnostic := bodyElementChunks(parsedCandidate, block, syntaxElements(block.Body), "module")
	if diagnostic != nil || len(chunks) != 3 {
		t.Fatalf("chunks = %v; diagnostic = %v", chunks, diagnostic)
	}
	reordered := bytes.Join([][]byte{candidate[:chunks[0].start], chunks[1].text, chunks[0].text, chunks[2].text, candidate[chunks[2].end:]}, nil)
	for _, corrupted := range [][]byte{
		bytes.ReplaceAll(candidate, []byte("same"), []byte("diff")),
		bytes.ReplaceAll(candidate, []byte("var.name"), []byte("var.fake")),
		bytes.ReplaceAll(candidate, []byte("./child"), []byte("./other")),
		bytes.ReplaceAll(candidate, []byte("aws.west"), []byte("aws.east")),
		bytes.ReplaceAll(candidate, []byte("aws = aws.west, other = other.east"), []byte("other = other.east, aws = aws.west")),
		bytes.ReplaceAll(candidate, []byte("# input"), []byte("# wrong")),
		bytes.ReplaceAll(candidate, []byte("\"one\""), []byte("\"two\"")),
		bytes.ReplaceAll(candidate, []byte("  # input\n  name = var.name\n  # provider\n"), []byte("  # provider\n  name = var.name\n  # input\n")),
		reordered,
		append(bytes.Clone(candidate), '\n'),
		bytes.ReplaceAll(candidate, []byte("./child\""), []byte("./child")),
	} {
		if bytes.Equal(candidate, corrupted) {
			t.Fatal("corruption did not change candidate")
		}
		if validateModuleCandidate(parsed, corrupted) == nil {
			t.Fatalf("accepted changed candidate: %s", corrupted)
		}
	}
}

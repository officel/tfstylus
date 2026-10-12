package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestImportOrderHandlesFixture(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "import-order", "basic.input.tf"))
	if err != nil {
		t.Fatalf("read input fixture: %v", err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "import-order", "basic.want.tf"))
	if err != nil {
		t.Fatalf("read expected fixture: %v", err)
	}

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, transformDiagnostics := transformTerraformOrderWithChanges(parsed, Config{ImportOrder: true})
	if len(transformDiagnostics) != 0 {
		t.Fatalf("transformTerraformOrderWithChanges() diagnostics = %v, want none", transformDiagnostics)
	}
	if len(changes) != 1 || changes[0].Rule != "import_order" {
		t.Fatalf("transformTerraformOrderWithChanges() changes = %v, want one import_order change", changes)
	}
	if !bytes.Equal(candidate, want) {
		t.Errorf("transformTerraformOrderWithChanges() =\n%s\nwant:\n%s", candidate, want)
	}
}

func TestImportOrderSupportsIdentity(t *testing.T) {
	source := []byte(`import {
  identity = { bucket = local.bucket }
  to = aws_s3_bucket.this
  provider = aws.west
  for_each = local.buckets
}
`)
	want := []byte("import {\n  for_each = local.buckets\n  provider = aws.west\n  identity = { bucket = local.bucket }\n  to = aws_s3_bucket.this\n}\n")
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, transformDiagnostics := transformTerraformOrderWithChanges(parsed, Config{ImportOrder: true})
	if len(transformDiagnostics) != 0 || len(changes) != 1 || !bytes.Equal(candidate, want) {
		t.Fatalf("candidate =\n%s\nchanges = %v; diagnostics = %v; want:\n%s", candidate, changes, transformDiagnostics, want)
	}
}

func TestImportOrderPreservesCRLFAndIsIdempotent(t *testing.T) {
	source := []byte("locals {\n  name = \"keep\"\n}\nimport {\n  id = local.bucket # bucket id\n  # target resource\n  to = aws_s3_bucket.this\n  provider = aws.west\n  for_each = local.buckets\n}\nimport {\n  id = \"other\"\n  to = aws_s3_bucket.other\n}\n")
	want := []byte("locals {\n  name = \"keep\"\n}\nimport {\n  for_each = local.buckets\n  provider = aws.west\n  id = local.bucket # bucket id\n  # target resource\n  to = aws_s3_bucket.this\n}\nimport {\n  id = \"other\"\n  to = aws_s3_bucket.other\n}\n")

	for _, newline := range []string{"\n", "\r\n"} {
		input := bytes.ReplaceAll(source, []byte("\n"), []byte(newline))
		expected := bytes.ReplaceAll(want, []byte("\n"), []byte(newline))
		parsed, diagnostics := parseHCLSource("main.tf", input)
		if diagnostics.HasErrors() {
			t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
		}

		candidate, changes, transformDiagnostics := transformTerraformOrderWithChanges(parsed, Config{ImportOrder: true})
		if len(transformDiagnostics) != 0 || len(changes) != 1 || changes[0].Rule != "import_order" {
			t.Fatalf("first transform changes = %v; diagnostics = %v", changes, transformDiagnostics)
		}
		if !bytes.Equal(candidate, expected) {
			t.Fatalf("first transform =\n%q\nwant:\n%q", candidate, expected)
		}

		parsedCandidate, diagnostics := parseHCLSource("main.tf", candidate)
		if diagnostics.HasErrors() {
			t.Fatalf("candidate parse diagnostics = %s, want none", diagnostics.Error())
		}
		second, secondChanges, secondDiagnostics := transformTerraformOrderWithChanges(parsedCandidate, Config{ImportOrder: true})
		if len(secondDiagnostics) != 0 || len(secondChanges) != 0 || !bytes.Equal(second, candidate) {
			t.Fatalf("second transform = %q; changes = %v; diagnostics = %v, want unchanged", second, secondChanges, secondDiagnostics)
		}
	}
}

func TestImportOrderSkipsAmbiguousCommentsAndHonorsDisabledRule(t *testing.T) {
	source := []byte(`import {
	to = aws_s3_bucket.this

  # separate section

	id = "bucket-123"
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	for _, test := range []struct {
		name        string
		enabled     bool
		wantWarning bool
	}{
		{name: "enabled warns and preserves input", enabled: true, wantWarning: true},
		{name: "disabled suppresses warning", enabled: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ImportOrder: test.enabled})
			if !bytes.Equal(candidate, source) || len(changes) != 0 {
				t.Fatalf("candidate = %s; changes = %v, want unchanged", candidate, changes)
			}
			if test.wantWarning {
				if len(warnings) != 1 || !bytes.Contains([]byte(warnings[0].Reason), []byte("ambiguous comment")) {
					t.Fatalf("warnings = %v, want ambiguous comment warning", warnings)
				}
			} else if len(warnings) != 0 {
				t.Fatalf("warnings = %v, want none when rule is disabled", warnings)
			}
		})
	}
}

func TestImportOrderDoesNotWarnWhenUnsupportedElementNeedsNoMove(t *testing.T) {
	source := []byte(`import {
  id = "bucket-123"
	to = aws_s3_bucket.this
  future {
    value = true
  }
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ImportOrder: true})
	if !bytes.Equal(candidate, source) || len(changes) != 0 || len(warnings) != 0 {
		t.Fatalf("candidate = %s; changes = %v; warnings = %v, want unchanged without diagnostics", candidate, changes, warnings)
	}
}

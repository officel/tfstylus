package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestRunBlockTransformCollectsChangesAndLocalDiagnostics(t *testing.T) {
	input := []byte("target \"skip\" {\n  value = 1\n}\n\ntarget \"change\" {\n  value = 2\n}\n")
	parsed, parseDiagnostics := parseHCLSource("pipeline.tf", input)
	if parseDiagnostics.HasErrors() {
		t.Fatalf("parse input: %s", parseDiagnostics.Error())
	}

	candidate, changes, diagnostics := runBlockTransform(parsed, blockTransformPolicy{
		blockType: "target",
		rule:      "target_order",
		reason:    "target blocks should be normalized",
		candidate: func(parsed *parsedFile, block *hclsyntax.Block, _ int) ([]byte, *transformDiagnostic) {
			if block.Labels[0] == "skip" {
				return nil, &transformDiagnostic{Subject: block.Range(), Reason: "ambiguous target body"}
			}
			return []byte(`target "change" { value = 2 }`), nil
		},
		validate: func(_ *parsedFile, _ []byte) *transformDiagnostic { return nil },
	})

	want := []byte("target \"skip\" {\n  value = 1\n}\n\ntarget \"change\" { value = 2 }\n")
	if !bytes.Equal(candidate, want) {
		t.Fatalf("candidate = %q, want %q", candidate, want)
	}
	if len(changes) != 1 || changes[0].Rule != "target_order" || changes[0].Reason != "target blocks should be normalized" {
		t.Fatalf("changes = %+v, want one target_order change", changes)
	}
	if len(diagnostics) != 1 || diagnostics[0].Reason != "ambiguous target body" || diagnostics[0].Subject.Start.Line != 1 {
		t.Fatalf("diagnostics = %+v, want the skipped first block at its input location", diagnostics)
	}
}

func TestRunBlockTransformPreservesInputWhenCandidateIsRejected(t *testing.T) {
	input := []byte("target \"first\" { value = 1 }\n")
	parsed, parseDiagnostics := parseHCLSource("pipeline.tf", input)
	if parseDiagnostics.HasErrors() {
		t.Fatalf("parse input: %s", parseDiagnostics.Error())
	}

	tests := []struct {
		name           string
		candidate      []byte
		validate       func(*parsedFile, []byte) *transformDiagnostic
		wantDiagnostic string
	}{
		{
			name:           "reparse failure",
			candidate:      []byte(`target "first" { value = }`),
			wantDiagnostic: "candidate failed HCL reparse",
		},
		{
			name:      "preservation failure",
			candidate: []byte(`target "first" { value = 2 }`),
			validate: func(*parsedFile, []byte) *transformDiagnostic {
				return &transformDiagnostic{Reason: "candidate changed protected source"}
			},
			wantDiagnostic: "candidate changed protected source",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate, changes, diagnostics := runBlockTransform(parsed, blockTransformPolicy{
				blockType: "target",
				rule:      "target_order",
				reason:    "normalize target",
				candidate: func(*parsedFile, *hclsyntax.Block, int) ([]byte, *transformDiagnostic) {
					return test.candidate, nil
				},
				validate: test.validate,
			})

			if !bytes.Equal(candidate, input) {
				t.Fatalf("candidate after rejection = %q, want original %q", candidate, input)
			}
			if len(changes) != 0 {
				t.Fatalf("changes after rejection = %+v, want none", changes)
			}
			if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Reason, test.wantDiagnostic) {
				t.Fatalf("diagnostics = %+v, want reason containing %q", diagnostics, test.wantDiagnostic)
			}
		})
	}
}

func TestRunTransformPipelineRunsEnabledStepsInOrder(t *testing.T) {
	input := []byte("terraform {}\n")
	parsed, parseDiagnostics := parseHCLSource("pipeline.tf", input)
	if parseDiagnostics.HasErrors() {
		t.Fatalf("parse input: %s", parseDiagnostics.Error())
	}

	var order []string
	candidate, changes, diagnostics := runTransformPipeline(parsed, []transformPipelineStep{
		{
			enabled: true,
			transform: func(*parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
				order = append(order, "first")
				return []byte("# first\nterraform {}\n"), []transformChange{{Rule: "first"}}, nil
			},
		},
		{
			enabled: false,
			transform: func(*parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
				t.Fatal("disabled pipeline step ran")
				return nil, nil, nil
			},
		},
		{
			enabled: true,
			transform: func(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
				order = append(order, "second")
				if !bytes.Contains(parsed.source, []byte("# first")) {
					t.Error("second step did not receive the first candidate")
				}
				return []byte("# second\nterraform {}\n"), []transformChange{{Rule: "second"}}, nil
			},
		},
	})

	if strings.Join(order, ",") != "first,second" {
		t.Fatalf("step order = %v, want [first second]", order)
	}
	if string(candidate) != "# second\nterraform {}\n" {
		t.Fatalf("candidate = %q, want second-stage candidate", candidate)
	}
	if len(changes) != 2 || changes[0].Rule != "first" || changes[1].Rule != "second" {
		t.Fatalf("changes = %+v, want ordered changes from both steps", changes)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want none", diagnostics)
	}
}

func TestRunTransformPipelinePreservesOriginalOnReparseFailure(t *testing.T) {
	input := []byte("terraform {}\n")
	parsed, parseDiagnostics := parseHCLSource("pipeline.tf", input)
	if parseDiagnostics.HasErrors() {
		t.Fatalf("parse input: %s", parseDiagnostics.Error())
	}

	candidate, changes, diagnostics := runTransformPipeline(parsed, []transformPipelineStep{
		{
			enabled: true,
			transform: func(*parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
				return []byte("terraform { value = }\n"), []transformChange{{Rule: "rejected"}}, nil
			},
		},
		{
			enabled: true,
			transform: func(*parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
				t.Fatal("pipeline continued after candidate reparse failure")
				return nil, nil, nil
			},
		},
	})

	if !bytes.Equal(candidate, input) {
		t.Fatalf("candidate after reparse failure = %q, want original %q", candidate, input)
	}
	if len(changes) != 0 {
		t.Fatalf("changes after reparse failure = %+v, want none", changes)
	}
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Reason, "candidate failed HCL reparse") {
		t.Fatalf("diagnostics = %+v, want candidate reparse diagnostic", diagnostics)
	}
}

func TestRunTransformPipelinePreservesOriginalWhenFinalValidationFails(t *testing.T) {
	input := []byte("terraform {}\n")
	parsed, parseDiagnostics := parseHCLSource("pipeline.tf", input)
	if parseDiagnostics.HasErrors() {
		t.Fatalf("parse input: %s", parseDiagnostics.Error())
	}

	candidate, changes, diagnostics := runTransformPipelineWithFinalValidation(parsed, []transformPipelineStep{
		{
			enabled: true,
			transform: func(*parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
				return []byte("# changed\nterraform {}\n"), []transformChange{{Rule: "first"}}, nil
			},
		},
	}, func(*parsedFile, []byte) *transformDiagnostic {
		return &transformDiagnostic{Reason: "final candidate rejected"}
	})

	if !bytes.Equal(candidate, input) {
		t.Fatalf("candidate after final validation failure = %q, want original %q", candidate, input)
	}
	if len(changes) != 0 {
		t.Fatalf("changes after final validation failure = %+v, want none", changes)
	}
	if len(diagnostics) != 1 || diagnostics[0].Reason != "final candidate rejected" {
		t.Fatalf("diagnostics = %+v, want final validation diagnostic", diagnostics)
	}
}

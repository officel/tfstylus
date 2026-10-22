package app

import (
	"bytes"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type blockTransformPolicy struct {
	blockType string
	rule      string
	reason    string
	candidate func(*parsedFile, *hclsyntax.Block, int) ([]byte, *transformDiagnostic)
	validate  func(*parsedFile, []byte) *transformDiagnostic
}

type transformPipelineStep struct {
	enabled   bool
	transform func(*parsedFile) ([]byte, []transformChange, []transformDiagnostic)
}

func runTransformPipeline(parsed *parsedFile, steps []transformPipelineStep) ([]byte, []transformChange, []transformDiagnostic) {
	return runTransformPipelineWithFinalValidation(parsed, steps, nil)
}

func runTransformPipelineWithFinalValidation(parsed *parsedFile, steps []transformPipelineStep, validate func(*parsedFile, []byte) *transformDiagnostic) ([]byte, []transformChange, []transformDiagnostic) {
	originalRoot, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return bytes.Clone(parsed.source), nil, []transformDiagnostic{{Reason: "unsupported root body syntax"}}
	}

	candidate := bytes.Clone(parsed.source)
	var changes []transformChange
	var diagnostics []transformDiagnostic
	runStep := false
	for _, step := range steps {
		if !step.enabled {
			continue
		}
		if step.transform == nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, transformDiagnostic{Reason: "transform pipeline step is missing a handler"})
		}

		stepParsed := parsed
		if runStep {
			validated, errors := parseHCLSource(originalRoot.Range().Filename, candidate)
			if errors.HasErrors() {
				diagnostics = append(diagnostics, transformDiagnostic{
					Subject: originalRoot.Range(),
					Reason:  "candidate failed HCL reparse: " + errors.Error(),
				})
				return bytes.Clone(parsed.source), nil, diagnostics
			}
			if _, ok := validated.body.(*hclsyntax.Body); !ok {
				diagnostics = append(diagnostics, transformDiagnostic{Subject: originalRoot.Range(), Reason: "unsupported candidate root body syntax"})
				return bytes.Clone(parsed.source), nil, diagnostics
			}
			stepParsed = validated
		}

		var stepChanges []transformChange
		var stepDiagnostics []transformDiagnostic
		candidate, stepChanges, stepDiagnostics = step.transform(stepParsed)
		changes = append(changes, stepChanges...)
		diagnostics = append(diagnostics, stepDiagnostics...)
		runStep = true
	}
	if validate != nil {
		if diagnostic := validate(parsed, candidate); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func runBlockTransform(parsed *parsedFile, policy blockTransformPolicy) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return candidate, nil, []transformDiagnostic{{Reason: "unsupported root body syntax"}}
	}

	var edits []sourceEdit
	var changes []transformChange
	var diagnostics []transformDiagnostic
	for blockIndex, block := range root.Blocks {
		if block.Type != policy.blockType {
			continue
		}
		if parsed.ruleIgnored(policy.rule, block) {
			continue
		}
		blockText, diagnostic := policy.candidate(parsed, block, blockIndex)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
			continue
		}
		if blockText == nil {
			continue
		}
		blockRange := block.Range()
		if bytes.Equal(blockText, parsed.source[blockRange.Start.Byte:blockRange.End.Byte]) {
			continue
		}
		edits = append(edits, sourceEdit{
			start: blockRange.Start.Byte,
			end:   blockRange.End.Byte,
			text:  blockText,
		})
		changes = append(changes, transformChange{
			Subject: blockRange,
			Rule:    policy.rule,
			Reason:  policy.reason,
		})
	}
	if len(edits) == 0 {
		return candidate, changes, diagnostics
	}

	candidate = applySourceEdits(candidate, edits)
	var diagnostic *transformDiagnostic
	if policy.validate != nil {
		diagnostic = policy.validate(parsed, candidate)
	} else {
		validated, parseDiagnostics := parseHCLSource(root.Range().Filename, candidate)
		if parseDiagnostics.HasErrors() {
			diagnostic = &transformDiagnostic{
				Subject: root.Range(),
				Reason:  "candidate failed HCL reparse: " + parseDiagnostics.Error(),
			}
		} else if _, ok := validated.body.(*hclsyntax.Body); !ok {
			diagnostic = &transformDiagnostic{Subject: root.Range(), Reason: "unsupported candidate root body syntax"}
		}
	}
	if diagnostic != nil {
		return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
	}
	return candidate, changes, diagnostics
}

func blockCandidateFromEdit(parsed *parsedFile, block *hclsyntax.Block, edit *sourceEdit) []byte {
	blockRange := block.Range()
	if edit == nil {
		return bytes.Clone(parsed.source[blockRange.Start.Byte:blockRange.End.Byte])
	}
	candidate := applySourceEdits(parsed.source, []sourceEdit{*edit})
	return bytes.Clone(candidate[blockRange.Start.Byte:blockRange.End.Byte])
}

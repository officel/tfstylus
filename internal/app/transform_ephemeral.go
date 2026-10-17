package app

import (
	"bytes"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func ephemeralElementRank(element syntaxElement) (int, bool) {
	if isTagElement(element) {
		return 4, true
	}
	if element.attribute != nil {
		switch element.attribute.Name {
		case "count", "for_each":
			return 0, true
		case "provider":
			return 1, true
		case "depends_on":
			return 6, true
		default:
			return 2, true
		}
	}
	if element.block != nil {
		if element.block.Type == "lifecycle" {
			return 5, true
		}
		return 3, true
	}
	return 0, false
}

func ephemeralBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	elements := syntaxElements(block.Body)
	if len(elements) < 2 {
		return nil, nil
	}

	indices, needsSort, unsupportedIndex := rankedElementOrder(elements, ephemeralElementRank)
	if unsupportedIndex != -1 {
		return nil, &transformDiagnostic{Subject: elements[unsupportedIndex].rangeValue, Reason: "ephemeral body contains an unsupported element"}
	}
	if !needsSort {
		return nil, nil
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line ephemeral body is not sorted"}
	}

	chunks, diagnostic := bodyElementChunks(parsed, block, elements, "ephemeral")
	if diagnostic != nil {
		return nil, diagnostic
	}
	var replacement []byte
	for _, index := range indices {
		replacement = append(replacement, chunks[index].text...)
	}
	if bytes.Equal(replacement, parsed.source[chunks[0].start:chunks[len(chunks)-1].end]) {
		return nil, nil
	}
	return &sourceEdit{start: chunks[0].start, end: chunks[len(chunks)-1].end, text: replacement}, nil
}

func ephemeralConditionRank(element syntaxElement) (int, bool) {
	if element.attribute == nil {
		if element.block != nil {
			return 3, true
		}
		return 0, false
	}
	switch element.attribute.Name {
	case "condition":
		return 0, true
	case "error_message":
		return 1, true
	default:
		return 2, true
	}
}

func ephemeralConditionEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	elements := syntaxElements(block.Body)
	if len(elements) < 2 {
		return nil, nil
	}

	indices, needsSort, _ := rankedElementOrder(elements, ephemeralConditionRank)
	if !needsSort {
		return nil, nil
	}
	chunks, diagnostic := bodyElementChunks(parsed, block, elements, "ephemeral condition")
	if diagnostic != nil {
		return nil, diagnostic
	}
	var replacement []byte
	for _, index := range indices {
		replacement = append(replacement, chunks[index].text...)
	}
	return &sourceEdit{start: chunks[0].start, end: chunks[len(chunks)-1].end, text: replacement}, nil
}

func ephemeralBlockCandidate(parsed *parsedFile, block *hclsyntax.Block, blockIndex int) ([]byte, *transformDiagnostic) {
	conditions := syntaxElements(block.Body)
	lifecycleCount := 0
	var conditionEdits []sourceEdit
	for _, element := range conditions {
		if element.block == nil || element.block.Type != "lifecycle" {
			continue
		}
		lifecycleCount++
		for _, condition := range element.block.Body.Blocks {
			if condition.Type != "precondition" && condition.Type != "postcondition" {
				continue
			}
			edit, diagnostic := ephemeralConditionEdit(parsed, condition)
			if diagnostic != nil {
				return nil, diagnostic
			}
			if edit != nil {
				conditionEdits = append(conditionEdits, *edit)
			}
		}
	}

	if len(conditionEdits) > 0 && lifecycleCount > 1 {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "multiple lifecycle blocks make ephemeral condition reordering ambiguous"}
	}
	blockCandidate := applySourceEdits(bytes.Clone(parsed.source), conditionEdits)
	if len(conditionEdits) > 0 {
		parsedCandidate, diagnostics := parseHCLSource(block.Range().Filename, blockCandidate)
		if diagnostics.HasErrors() {
			return nil, &transformDiagnostic{Subject: block.Range(), Reason: "ephemeral condition candidate failed HCL reparse: " + diagnostics.Error()}
		}
		root, ok := parsedCandidate.body.(*hclsyntax.Body)
		if !ok || blockIndex >= len(root.Blocks) {
			return nil, &transformDiagnostic{Subject: block.Range(), Reason: "ephemeral condition candidate changed root structure"}
		}
		parsed = parsedCandidate
		block = root.Blocks[blockIndex]
	}

	var lifecycleEdits []sourceEdit
	for _, nested := range block.Body.Blocks {
		if nested.Type != "lifecycle" {
			continue
		}
		edit, diagnostic := lifecycleBlockEdit(parsed, nested)
		if diagnostic != nil {
			return nil, diagnostic
		}
		if edit != nil {
			lifecycleEdits = append(lifecycleEdits, *edit)
		}
	}
	if len(lifecycleEdits) > 0 && lifecycleCount > 1 {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "multiple lifecycle blocks make ephemeral condition reordering ambiguous"}
	}
	blockCandidate = applySourceEdits(blockCandidate, lifecycleEdits)
	if len(lifecycleEdits) > 0 {
		parsedCandidate, diagnostics := parseHCLSource(block.Range().Filename, blockCandidate)
		if diagnostics.HasErrors() {
			return nil, &transformDiagnostic{Subject: block.Range(), Reason: "ephemeral lifecycle candidate failed HCL reparse: " + diagnostics.Error()}
		}
		root, ok := parsedCandidate.body.(*hclsyntax.Body)
		if !ok || blockIndex >= len(root.Blocks) {
			return nil, &transformDiagnostic{Subject: block.Range(), Reason: "ephemeral lifecycle candidate changed root structure"}
		}
		parsed = parsedCandidate
		block = root.Blocks[blockIndex]
	}

	outerEdit, diagnostic := ephemeralBlockEdit(parsed, block)
	if diagnostic != nil {
		return nil, diagnostic
	}
	if outerEdit != nil && lifecycleCount > 1 {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "multiple lifecycle blocks make ephemeral reordering ambiguous"}
	}
	if outerEdit != nil {
		blockCandidate = applySourceEdits(blockCandidate, []sourceEdit{*outerEdit})
	}
	blockRange := block.Range()
	return bytes.Clone(blockCandidate[blockRange.Start.Byte:blockRange.End.Byte]), nil
}

func transformEphemeralOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	return runBlockTransform(parsed, blockTransformPolicy{
		blockType: "ephemeral",
		rule:      "ephemeral_order",
		reason:    "ephemeral elements should be ordered count/for_each, provider-specific attributes, provider, nested blocks, lifecycle, then depends_on",
		candidate: func(parsed *parsedFile, block *hclsyntax.Block, blockIndex int) ([]byte, *transformDiagnostic) {
			return ephemeralBlockCandidate(parsed, block, blockIndex)
		},
		validate: validateEphemeralCandidate,
	})
}

func validateEphemeralCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	conditionPolicy := sourceBodyPolicy{owner: "ephemeral condition", rank: ephemeralConditionRank}
	return validateSourceCandidate(original, candidate, map[string]sourceBodyPolicy{
		"ephemeral": {
			owner: "ephemeral", rank: ephemeralElementRank,
			children: map[string]sourceBodyPolicy{
				"lifecycle": {
					owner: "lifecycle", rank: lifecycleConditionRank,
					children: map[string]sourceBodyPolicy{
						"precondition":  conditionPolicy,
						"postcondition": conditionPolicy,
					},
				},
			},
		},
	})
}

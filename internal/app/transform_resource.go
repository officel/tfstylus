package app

import "github.com/hashicorp/hcl/v2/hclsyntax"

func resourceElementRank(element syntaxElement) (int, bool) {
	return commonElementRank(element)
}

func resourceBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	elements := syntaxElements(block.Body)
	if len(elements) < 2 {
		return nil, nil
	}

	indices, needsSort, unsupportedIndex := rankedElementOrder(elements, resourceElementRank)
	if unsupportedIndex != -1 {
		return nil, &transformDiagnostic{Subject: elements[unsupportedIndex].rangeValue, Reason: "resource body contains an unsupported element"}
	}
	if !needsSort {
		return nil, nil
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line resource body is not sorted"}
	}

	return rankedBodyEdit(parsed, block, elements, indices, "resource")
}

func resourceBlockCandidate(parsed *parsedFile, block *hclsyntax.Block) ([]byte, *transformDiagnostic) {
	edit, diagnostic := resourceBlockEdit(parsed, block)
	if diagnostic != nil {
		return nil, diagnostic
	}
	return blockCandidateFromEdit(parsed, block, edit), nil
}

func transformResourceOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	return runBlockTransform(parsed, blockTransformPolicy{
		blockType: "resource",
		rule:      "resource_order",
		reason:    "resource elements should be ordered count/for_each, id, name, display_name, provider, provider-specific attributes, provider-specific blocks, tag/tags, lifecycle, then depends_on",
		candidate: func(parsed *parsedFile, block *hclsyntax.Block, _ int) ([]byte, *transformDiagnostic) {
			return resourceBlockCandidate(parsed, block)
		},
		validate: validateResourceCandidate,
	})
}

func validateResourceCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	return validateSourceCandidate(original, candidate, map[string]sourceBodyPolicy{
		"resource": {
			owner: "resource", rank: resourceElementRank,
			children: map[string]sourceBodyPolicy{
				"lifecycle": {owner: "lifecycle", rank: lifecycleConditionRank},
			},
		},
	})
}

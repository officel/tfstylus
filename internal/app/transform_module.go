package app

import "github.com/hashicorp/hcl/v2/hclsyntax"

func moduleElementRank(element syntaxElement) (int, bool) {
	if element.attribute == nil {
		if element.block != nil {
			return 7, true
		}
		return 0, false
	}
	switch element.attribute.Name {
	case "source":
		return 0, true
	case "version":
		return 1, true
	case "count", "for_each":
		return 2, true
	case "id":
		return 3, true
	case "name":
		return 4, true
	case "providers":
		return 6, true
	case "ignore_nested_deprecations":
		return 8, true
	case "depends_on":
		return 9, true
	default:
		return 5, true
	}
}

func moduleBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	elements := syntaxElements(block.Body)
	indices, needsSort, unsupportedIndex := rankedElementOrder(elements, moduleElementRank)
	if !needsSort {
		return nil, nil
	}
	if unsupportedIndex != -1 {
		return nil, &transformDiagnostic{Subject: elements[unsupportedIndex].rangeValue, Reason: "module body contains an unsupported nested block"}
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line module body is not sorted"}
	}
	return rankedBodyEdit(parsed, block, elements, indices, "module")
}

func moduleBlockCandidate(parsed *parsedFile, block *hclsyntax.Block) ([]byte, *transformDiagnostic) {
	edit, diagnostic := moduleBlockEdit(parsed, block)
	if diagnostic != nil {
		return nil, diagnostic
	}
	return blockCandidateFromEdit(parsed, block, edit), nil
}

func transformModuleOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	return runBlockTransform(parsed, blockTransformPolicy{
		blockType: "module",
		rule:      "module_order",
		reason:    "module attributes should be ordered source, version, count/for_each, id, name, module-specific inputs, providers, nested blocks, ignore_nested_deprecations, then depends_on",
		candidate: func(parsed *parsedFile, block *hclsyntax.Block, _ int) ([]byte, *transformDiagnostic) {
			return moduleBlockCandidate(parsed, block)
		},
		validate: validateModuleCandidate,
	})
}

func validateModuleCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	return validateSourceCandidate(original, candidate, map[string]sourceBodyPolicy{
		"module": {owner: "module", rank: moduleElementRank},
	})
}

package app

import (
	"bytes"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func resourceElementRank(element syntaxElement) (int, bool) {
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

	chunks, diagnostic := bodyElementChunks(parsed, block, elements, "resource")
	if diagnostic != nil {
		return nil, diagnostic
	}
	var replacement []byte
	for _, index := range indices {
		replacement = append(replacement, chunks[index].text...)
	}
	start := chunks[0].start
	end := chunks[len(chunks)-1].end
	if bytes.Equal(replacement, parsed.source[start:end]) {
		return nil, nil
	}
	return &sourceEdit{start: start, end: end, text: replacement}, nil
}

func resourceBlockCandidate(parsed *parsedFile, block *hclsyntax.Block) ([]byte, *transformDiagnostic) {
	edit, diagnostic := resourceBlockEdit(parsed, block)
	if diagnostic != nil {
		return nil, diagnostic
	}
	blockRange := block.Range()
	if edit == nil {
		return bytes.Clone(parsed.source[blockRange.Start.Byte:blockRange.End.Byte]), nil
	}
	candidate := append(bytes.Clone(parsed.source[:edit.start]), append(bytes.Clone(edit.text), parsed.source[edit.end:]...)...)
	return bytes.Clone(candidate[blockRange.Start.Byte:blockRange.End.Byte]), nil
}

func transformResourceOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	return runBlockTransform(parsed, blockTransformPolicy{
		blockType: "resource",
		rule:      "resource_order",
		reason:    "resource elements should be ordered count/for_each, provider, provider-specific attributes, provider-specific blocks, lifecycle, then depends_on",
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

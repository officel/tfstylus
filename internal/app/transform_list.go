package app

import (
	"bytes"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func listElementRank(element syntaxElement) (int, bool) {
	if rank, identity := identityAttributeRank(element); identity {
		return rank, true
	}
	if element.attribute != nil {
		switch element.attribute.Name {
		case "count", "for_each":
			return 0, true
		case "provider":
			return 4, true
		case "include_resource":
			return 5, true
		case "limit":
			return 6, true
		default:
			return 7, true
		}
	}
	if element.block != nil {
		return 8, true
	}
	return 0, false
}

func listBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	elements := syntaxElements(block.Body)
	if len(elements) < 2 {
		return nil, nil
	}
	indices, needsSort, _ := rankedElementOrder(elements, listElementRank)
	if !needsSort {
		return nil, nil
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line list body is not sorted"}
	}
	chunks, diagnostic := bodyElementChunks(parsed, block, elements, "list")
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

func listBlockCandidate(parsed *parsedFile, block *hclsyntax.Block) ([]byte, *transformDiagnostic) {
	edit, diagnostic := listBlockEdit(parsed, block)
	if diagnostic != nil {
		return nil, diagnostic
	}
	return blockCandidateFromEdit(parsed, block, edit), nil
}

func transformListOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	return runBlockTransform(parsed, blockTransformPolicy{
		blockType: "list",
		rule:      "list_order",
		reason:    "list arguments should be ordered count/for_each, id, name, display_name, provider, include_resource, limit, provider-specific attributes, then config",
		candidate: func(parsed *parsedFile, block *hclsyntax.Block, _ int) ([]byte, *transformDiagnostic) {
			return listBlockCandidate(parsed, block)
		},
		validate: validateListCandidate,
	})
}

func validateListCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	return validateSourceCandidate(original, candidate, map[string]sourceBodyPolicy{
		"list": {owner: "list", rank: listElementRank},
	})
}

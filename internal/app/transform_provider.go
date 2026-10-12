package app

import (
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func providerElementRank(element syntaxElement) (int, bool) {
	if element.attribute != nil {
		switch element.attribute.Name {
		case "alias":
			return 1, true
		case "region":
			return 2, true
		default:
			return 0, true
		}
	}
	if element.block != nil {
		return 3, true
	}
	return 0, false
}

func providerBodyChunks(parsed *parsedFile, block *hclsyntax.Block, elements []syntaxElement) ([]sourceChunk, *transformDiagnostic) {
	leadingStarts := make([]int, len(elements))
	trailingEnds := make([]int, len(elements))
	for index, element := range elements {
		leadingStarts[index] = lineStart(parsed.source, element.rangeValue.Start.Byte)
		trailingEnds[index] = lineEnd(parsed.source, cloudElementEnd(element))
	}

	if diagnostic := assignSourceComments(parsed, block, elements, leadingStarts, trailingEnds, sourceCommentPolicy{
		owner:                  "provider",
		elementStart:           func(element syntaxElement) int { return element.rangeValue.Start.Byte },
		elementEnd:             cloudElementEnd,
		trailingCommentLineEnd: cloudCommentLineEnd,
		preserveEarliestStart:  true,
	}); diagnostic != nil {
		return nil, diagnostic
	}

	previousElementLine := -1
	for index, element := range elements {
		start := leadingStarts[index]
		elementLine := lineStart(parsed.source, element.rangeValue.Start.Byte)
		if elementLine == previousElementLine || !horizontalWhitespace(parsed.source[elementLine:element.rangeValue.Start.Byte]) {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "provider elements do not occupy unambiguous standalone lines"}
		}
		previousElementLine = elementLine
		end := trailingEnds[index]
		if index+1 < len(elements) {
			end = leadingStarts[index+1]
		}
		if end < start {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "provider element comment chunks overlap"}
		}
	}
	if lineStart(parsed.source, block.CloseBraceRange.Start.Byte) == previousElementLine {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "provider elements do not occupy unambiguous standalone lines"}
	}
	return sourceChunksFromBounds(parsed.source, leadingStarts, trailingEnds), nil
}

func providerBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	elements := syntaxElements(block.Body)
	if len(elements) < 2 {
		return nil, nil
	}

	indices, needsSort, unsupportedIndex := rankedElementOrder(elements, providerElementRank)
	if unsupportedIndex != -1 {
		return nil, &transformDiagnostic{Subject: elements[unsupportedIndex].rangeValue, Reason: "provider body contains an unsupported element"}
	}
	if !needsSort {
		return nil, nil
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line provider body is not sorted"}
	}

	chunks, diagnostic := providerBodyChunks(parsed, block, elements)
	if diagnostic != nil {
		return nil, diagnostic
	}
	var replacement []byte
	for _, index := range indices {
		replacement = append(replacement, chunks[index].text...)
	}
	return &sourceEdit{start: chunks[0].start, end: chunks[len(chunks)-1].end, text: replacement}, nil
}

func transformProviderOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	return runBlockTransform(parsed, blockTransformPolicy{
		blockType: "provider",
		rule:      "provider_order",
		reason:    "provider elements should be ordered provider-specific attributes, alias, region, then blocks",
		candidate: func(parsed *parsedFile, block *hclsyntax.Block, _ int) ([]byte, *transformDiagnostic) {
			edit, diagnostic := providerBlockEdit(parsed, block)
			if diagnostic != nil {
				return nil, diagnostic
			}
			return blockCandidateFromEdit(parsed, block, edit), nil
		},
		validate: validateProviderCandidate,
	})
}

func validateProviderCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	return validateSourceCandidate(original, candidate, map[string]sourceBodyPolicy{
		"provider": {owner: "provider", rank: providerElementRank},
	})
}

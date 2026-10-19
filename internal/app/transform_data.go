package app

import (
	"bytes"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func dataElementRank(element syntaxElement) (int, bool) {
	return commonElementRank(element)
}

func dataElementEnd(element syntaxElement) int {
	if element.block != nil {
		return element.block.CloseBraceRange.End.Byte
	}
	return element.rangeValue.End.Byte
}

func sourceElementChunks(parsed *parsedFile, block *hclsyntax.Block, elements []syntaxElement, owner string) ([]sourceChunk, *transformDiagnostic) {
	starts := make([]int, len(elements))
	ends := make([]int, len(elements))
	for index, element := range elements {
		starts[index] = lineStart(parsed.source, element.rangeValue.Start.Byte)
		ends[index] = lineEnd(parsed.source, dataElementEnd(element))
		if !horizontalWhitespace(parsed.source[starts[index]:element.rangeValue.Start.Byte]) {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: owner + " elements must occupy standalone lines"}
		}
	}

	if diagnostic := assignSourceComments(parsed, block, elements, starts, ends, sourceCommentPolicy{
		owner:                  owner,
		elementStart:           func(element syntaxElement) int { return element.rangeValue.Start.Byte },
		elementEnd:             dataElementEnd,
		trailingCommentLineEnd: lineEnd,
	}); diagnostic != nil {
		return nil, diagnostic
	}

	previousElementLine := -1
	for index, element := range elements {
		start := starts[index]
		elementLine := lineStart(parsed.source, element.rangeValue.Start.Byte)
		if elementLine == previousElementLine || !horizontalWhitespace(parsed.source[elementLine:element.rangeValue.Start.Byte]) {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: owner + " elements do not occupy unambiguous standalone lines"}
		}
		previousElementLine = elementLine
		end := ends[index]
		if index+1 < len(elements) {
			end = starts[index+1]
		}
		if start < block.OpenBraceRange.End.Byte || end > block.CloseBraceRange.Start.Byte || end < dataElementEnd(element) || end < start {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: owner + " element comment chunks have ambiguous boundaries"}
		}
	}
	if lineStart(parsed.source, block.CloseBraceRange.Start.Byte) == previousElementLine {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: owner + " elements do not occupy unambiguous standalone lines"}
	}
	return sourceChunksFromBounds(parsed.source, starts, ends), nil
}

func bodyElementChunks(parsed *parsedFile, block *hclsyntax.Block, elements []syntaxElement, owner string) ([]sourceChunk, *transformDiagnostic) {
	return sourceElementChunks(parsed, block, elements, owner)
}

func dataBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	elements := syntaxElements(block.Body)
	if len(elements) < 2 {
		return nil, nil
	}

	indices, needsSort, unsupportedIndex := rankedElementOrder(elements, dataElementRank)
	if unsupportedIndex != -1 {
		return nil, &transformDiagnostic{Subject: elements[unsupportedIndex].rangeValue, Reason: "data body contains an unsupported element"}
	}
	if !needsSort {
		return nil, nil
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line data body is not sorted"}
	}

	chunks, diagnostic := bodyElementChunks(parsed, block, elements, "data")
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

func lifecycleConditionRank(element syntaxElement) (int, bool) {
	if element.block == nil {
		if element.attribute != nil {
			return 2, true
		}
		return 0, false
	}
	switch element.block.Type {
	case "precondition":
		return 0, true
	case "postcondition":
		return 1, true
	default:
		return 2, true
	}
}

func lifecycleBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	elements := syntaxElements(block.Body)
	if len(elements) < 2 {
		return nil, nil
	}

	indices, needsSort, _ := rankedElementOrder(elements, lifecycleConditionRank)
	if !needsSort {
		return nil, nil
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line lifecycle body is not sorted"}
	}

	chunks, diagnostic := bodyElementChunks(parsed, block, elements, "lifecycle")
	if diagnostic != nil {
		return nil, diagnostic
	}
	var replacement []byte
	for _, index := range indices {
		replacement = append(replacement, chunks[index].text...)
	}
	return &sourceEdit{start: chunks[0].start, end: chunks[len(chunks)-1].end, text: replacement}, nil
}

func dataBlockCandidate(parsed *parsedFile, block *hclsyntax.Block, blockIndex int) ([]byte, *transformDiagnostic) {
	blockCandidate := bytes.Clone(parsed.source)
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
	blockCandidate = applySourceEdits(blockCandidate, lifecycleEdits)

	parsedCandidate, parseDiagnostics := parseHCLSource(block.Range().Filename, blockCandidate)
	if parseDiagnostics.HasErrors() {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "lifecycle condition candidate failed HCL reparse: " + parseDiagnostics.Error()}
	}
	candidateRoot, ok := parsedCandidate.body.(*hclsyntax.Body)
	if !ok || blockIndex >= len(candidateRoot.Blocks) {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "lifecycle condition candidate changed root structure"}
	}
	candidateBlock := candidateRoot.Blocks[blockIndex]
	outerEdit, diagnostic := dataBlockEdit(parsedCandidate, candidateBlock)
	if diagnostic != nil {
		return nil, diagnostic
	}
	if outerEdit != nil {
		blockCandidate = applySourceEdits(blockCandidate, []sourceEdit{*outerEdit})
	}

	blockRange := block.Range()
	blockText := blockCandidate[blockRange.Start.Byte:blockRange.End.Byte]
	if len(blockText) != blockRange.End.Byte-blockRange.Start.Byte {
		return nil, &transformDiagnostic{Subject: blockRange, Reason: "data order edits changed the data block byte length"}
	}
	return bytes.Clone(blockText), nil
}

func transformDataOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	return runBlockTransform(parsed, blockTransformPolicy{
		blockType: "data",
		rule:      "data_order",
		reason:    "data elements should be ordered count/for_each, id, name, display_name, provider, provider-specific attributes, nested blocks, tag/tags, lifecycle, then depends_on",
		candidate: func(parsed *parsedFile, block *hclsyntax.Block, blockIndex int) ([]byte, *transformDiagnostic) {
			return dataBlockCandidate(parsed, block, blockIndex)
		},
		validate: validateDataCandidate,
	})
}

func validateDataCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	return validateSourceCandidate(original, candidate, map[string]sourceBodyPolicy{
		"data": {
			owner: "data", rank: dataElementRank,
			children: map[string]sourceBodyPolicy{
				"lifecycle": {owner: "lifecycle", rank: lifecycleConditionRank},
			},
		},
	})
}

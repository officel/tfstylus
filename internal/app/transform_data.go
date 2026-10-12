package app

import (
	"bytes"
	"sort"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type dataElementChunk struct {
	start int
	end   int
	text  []byte
}

func dataElementRank(element syntaxElement) (int, bool) {
	if element.attribute != nil {
		switch element.attribute.Name {
		case "count", "for_each":
			return 0, true
		case "provider":
			return 1, true
		case "depends_on":
			return 5, true
		default:
			return 2, true
		}
	}
	if element.block != nil {
		if element.block.Type == "lifecycle" {
			return 4, true
		}
		return 3, true
	}
	return 0, false
}

func dataElementEnd(element syntaxElement) int {
	if element.block != nil {
		return element.block.CloseBraceRange.End.Byte
	}
	return element.rangeValue.End.Byte
}

func bodyElementChunks(parsed *parsedFile, block *hclsyntax.Block, elements []syntaxElement, owner string) ([]dataElementChunk, *transformDiagnostic) {
	starts := make([]int, len(elements))
	ends := make([]int, len(elements))
	for index, element := range elements {
		starts[index] = lineStart(parsed.source, element.rangeValue.Start.Byte)
		ends[index] = lineEnd(parsed.source, dataElementEnd(element))
		if !horizontalWhitespace(parsed.source[starts[index]:element.rangeValue.Start.Byte]) {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: owner + " elements must occupy standalone lines"}
		}
	}

	for _, token := range parsed.tokens {
		if token.Type != hclsyntax.TokenComment || token.Range.Start.Byte < block.OpenBraceRange.End.Byte || token.Range.End.Byte > block.CloseBraceRange.Start.Byte {
			continue
		}
		owned := false
		for index, element := range elements {
			end := dataElementEnd(element)
			if token.Range.Start.Byte >= element.rangeValue.Start.Byte && token.Range.End.Byte <= end {
				owned = true
				break
			}
			if token.Range.Start.Byte >= end && lineStart(parsed.source, token.Range.Start.Byte) == lineStart(parsed.source, end) && horizontalWhitespace(parsed.source[end:token.Range.Start.Byte]) {
				ends[index] = lineEnd(parsed.source, token.Range.End.Byte)
				owned = true
				break
			}
		}
		if owned {
			continue
		}

		for index, element := range elements {
			if element.rangeValue.Start.Byte <= token.Range.End.Byte {
				continue
			}
			commentStart := lineStart(parsed.source, token.Range.Start.Byte)
			endsWithNewline := token.Range.End.Byte > token.Range.Start.Byte && parsed.source[token.Range.End.Byte-1] == '\n'
			if token.Range.End.Byte <= starts[index] && horizontalWhitespace(parsed.source[commentStart:token.Range.Start.Byte]) &&
				commentGapWhitespace(parsed.source[token.Range.End.Byte:starts[index]], endsWithNewline) {
				starts[index] = commentStart
				owned = true
			}
			break
		}
		if !owned {
			return nil, &transformDiagnostic{Subject: token.Range, Reason: owner + " body contains an ambiguous comment attachment"}
		}
	}

	chunks := make([]dataElementChunk, len(elements))
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
		chunks[index] = dataElementChunk{start: start, end: end, text: bytes.Clone(parsed.source[start:end])}
	}
	if lineStart(parsed.source, block.CloseBraceRange.Start.Byte) == previousElementLine {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: owner + " elements do not occupy unambiguous standalone lines"}
	}
	chunks[len(chunks)-1].end = ends[len(ends)-1]
	chunks[len(chunks)-1].text = bytes.Clone(parsed.source[chunks[len(chunks)-1].start:chunks[len(chunks)-1].end])
	return chunks, nil
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
	sort.Slice(lifecycleEdits, func(i, j int) bool { return lifecycleEdits[i].start > lifecycleEdits[j].start })
	for _, edit := range lifecycleEdits {
		blockCandidate = append(bytes.Clone(blockCandidate[:edit.start]), append(bytes.Clone(edit.text), blockCandidate[edit.end:]...)...)
	}

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
		blockCandidate = append(bytes.Clone(blockCandidate[:outerEdit.start]), append(bytes.Clone(outerEdit.text), blockCandidate[outerEdit.end:]...)...)
	}

	blockRange := block.Range()
	blockText := blockCandidate[blockRange.Start.Byte:blockRange.End.Byte]
	if len(blockText) != blockRange.End.Byte-blockRange.Start.Byte {
		return nil, &transformDiagnostic{Subject: blockRange, Reason: "data order edits changed the data block byte length"}
	}
	return bytes.Clone(blockText), nil
}

func transformDataOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return candidate, nil, []transformDiagnostic{{Reason: "unsupported root body syntax"}}
	}

	var edits []sourceEdit
	var changes []transformChange
	var diagnostics []transformDiagnostic
	for blockIndex, block := range root.Blocks {
		if block.Type != "data" {
			continue
		}
		blockText, diagnostic := dataBlockCandidate(parsed, block, blockIndex)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
			continue
		}

		blockRange := block.Range()
		if !bytes.Equal(blockText, parsed.source[blockRange.Start.Byte:blockRange.End.Byte]) {
			edits = append(edits, sourceEdit{start: blockRange.Start.Byte, end: blockRange.End.Byte, text: bytes.Clone(blockText)})
			changes = append(changes, transformChange{
				Subject: blockRange, Rule: "data_order",
				Reason: "data elements should be ordered count/for_each, provider-specific attributes, provider, nested blocks, lifecycle, then depends_on",
			})
		}
	}

	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for _, edit := range edits {
		candidate = append(bytes.Clone(candidate[:edit.start]), append(bytes.Clone(edit.text), candidate[edit.end:]...)...)
	}
	if len(edits) > 0 {
		if diagnostic := validateDataCandidate(parsed, candidate); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func validateDataCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	originalRoot := original.body.(*hclsyntax.Body)
	failure := &transformDiagnostic{Subject: originalRoot.Range(), Reason: "candidate changed data elements, comment ownership, or non-target bytes"}
	validated, diagnostics := parseHCLSource(originalRoot.Range().Filename, candidate)
	if diagnostics.HasErrors() {
		return failure
	}
	candidateRoot, ok := validated.body.(*hclsyntax.Body)
	if !ok || len(original.source) != len(candidate) {
		return failure
	}
	originalElements := syntaxElements(originalRoot)
	candidateElements := syntaxElements(candidateRoot)
	if len(originalElements) != len(candidateElements) {
		return failure
	}

	cursor := 0
	blockIndex := 0
	for index, element := range originalElements {
		other := candidateElements[index]
		if element.start != other.start || element.kind != other.kind || element.name != other.name || !sameLabels(element.labels, other.labels) ||
			!bytes.Equal(original.source[cursor:element.start], candidate[cursor:other.start]) {
			return failure
		}
		if element.block != nil && element.block.Type == "data" {
			expected, diagnostic := dataBlockCandidate(original, element.block, blockIndex)
			if diagnostic != nil || other.block == nil || len(expected) != other.rangeValue.End.Byte-other.rangeValue.Start.Byte ||
				!bytes.Equal(expected, candidate[other.rangeValue.Start.Byte:other.rangeValue.End.Byte]) {
				return failure
			}
		} else if !sameRangeBytes(original.source, element.rangeValue, candidate, other.rangeValue) {
			return failure
		}
		if element.block != nil {
			blockIndex++
		}
		cursor = element.rangeValue.End.Byte
	}
	if !bytes.Equal(original.source[cursor:], candidate[cursor:]) {
		return failure
	}
	return nil
}

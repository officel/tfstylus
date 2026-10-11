package app

import (
	"bytes"
	"sort"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func outputElementRank(element syntaxElement) (int, bool) {
	if element.attribute != nil {
		switch element.name {
		case "description":
			return 0, true
		case "deprecated":
			return 1, true
		case "sensitive":
			return 2, true
		case "ephemeral":
			return 3, true
		case "type":
			return 4, true
		case "value":
			return 5, true
		case "depends_on":
			return 7, true
		}
	}
	if element.block != nil && element.name == "precondition" && len(element.labels) == 0 {
		return 6, true
	}
	return 0, false
}

func outputBodyChunks(parsed *parsedFile, block *hclsyntax.Block, elements []syntaxElement) ([]cloudElementChunk, *transformDiagnostic) {
	starts := make([]int, len(elements))
	ends := make([]int, len(elements))
	for index, element := range elements {
		starts[index] = lineStart(parsed.source, element.start)
		ends[index] = lineEnd(parsed.source, cloudElementEnd(element))
		if !horizontalWhitespace(parsed.source[starts[index]:element.start]) {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "output elements must occupy standalone lines"}
		}
	}
	for tokenIndex := len(parsed.tokens) - 1; tokenIndex >= 0; tokenIndex-- {
		token := parsed.tokens[tokenIndex]
		if token.Type != hclsyntax.TokenComment || token.Range.Start.Byte < block.OpenBraceRange.End.Byte || token.Range.End.Byte > block.CloseBraceRange.Start.Byte {
			continue
		}
		owned := false
		for index, element := range elements {
			end := cloudElementEnd(element)
			if token.Range.Start.Byte >= element.start && token.Range.End.Byte <= end {
				owned = true
				break
			}
			if token.Range.Start.Byte >= end && lineStart(parsed.source, token.Range.Start.Byte) == lineStart(parsed.source, end) && horizontalWhitespace(parsed.source[end:token.Range.Start.Byte]) {
				ends[index] = cloudCommentLineEnd(parsed.source, token.Range.End.Byte)
				owned = true
				break
			}
		}
		if owned {
			continue
		}
		for index, element := range elements {
			if element.start <= token.Range.End.Byte {
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
			return nil, &transformDiagnostic{Subject: token.Range, Reason: "output body contains an ambiguous comment attachment"}
		}
	}
	chunks := make([]cloudElementChunk, len(elements))
	for index, element := range elements {
		end := ends[index]
		if index+1 < len(elements) {
			if end > starts[index+1] {
				return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "output comment chunks overlap"}
			}
			end = starts[index+1]
		}
		if starts[index] < block.OpenBraceRange.End.Byte || end > block.CloseBraceRange.Start.Byte || end < cloudElementEnd(element) {
			return nil, &transformDiagnostic{Subject: block.Range(), Reason: "output element boundaries are ambiguous"}
		}
		chunks[index] = cloudElementChunk{start: starts[index], end: end, text: bytes.Clone(parsed.source[starts[index]:end])}
	}
	return chunks, nil
}

func outputBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	if len(block.Labels) != 1 || !hclsyntax.ValidIdentifier(block.Labels[0]) {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "output must have one identifier label"}
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line output body is not sorted"}
	}
	elements := syntaxElements(block.Body)
	needsSort := false
	previousRank := -1
	for _, element := range elements {
		rank, known := outputElementRank(element)
		if !known {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "output body contains an unknown element or labeled precondition"}
		}
		needsSort = needsSort || rank < previousRank
		previousRank = rank
	}
	if !needsSort {
		return nil, nil
	}
	chunks, diagnostic := outputBodyChunks(parsed, block, elements)
	if diagnostic != nil {
		return nil, diagnostic
	}
	indices := make([]int, len(elements))
	for index := range indices {
		indices[index] = index
	}
	sort.SliceStable(indices, func(left, right int) bool {
		leftRank, _ := outputElementRank(elements[indices[left]])
		rightRank, _ := outputElementRank(elements[indices[right]])
		return leftRank < rightRank
	})
	var replacement []byte
	for _, index := range indices {
		replacement = append(replacement, chunks[index].text...)
	}
	return &sourceEdit{start: chunks[0].start, end: chunks[len(chunks)-1].end, text: replacement}, nil
}

func transformOutputOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return candidate, nil, []transformDiagnostic{{Reason: "unsupported root body syntax"}}
	}
	var edits []sourceEdit
	var changes []transformChange
	var diagnostics []transformDiagnostic
	for _, block := range root.Blocks {
		if block.Type != "output" {
			continue
		}
		edit, diagnostic := outputBlockEdit(parsed, block)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
		} else if edit != nil {
			edits = append(edits, *edit)
			changes = append(changes, transformChange{
				Subject: block.Range(), Rule: "output_order",
				Reason: "output elements should be ordered description, deprecated, sensitive, ephemeral, type, value, precondition, depends_on",
			})
		}
	}
	for index := len(edits) - 1; index >= 0; index-- {
		edit := edits[index]
		candidate = append(bytes.Clone(candidate[:edit.start]), append(bytes.Clone(edit.text), candidate[edit.end:]...)...)
	}
	if len(edits) > 0 {
		if diagnostic := validateOutputCandidate(parsed, candidate); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func validateOutputCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	root := original.body.(*hclsyntax.Body)
	failure := &transformDiagnostic{Subject: root.Range(), Reason: "candidate changed output elements, comment ownership, or non-target bytes"}
	validated, diagnostics := parseHCLSource(root.Range().Filename, candidate)
	if diagnostics.HasErrors() {
		return failure
	}
	candidateRoot, ok := validated.body.(*hclsyntax.Body)
	if !ok || len(original.source) != len(candidate) {
		return failure
	}
	originalElements := syntaxElements(root)
	candidateElements := syntaxElements(candidateRoot)
	if len(originalElements) != len(candidateElements) {
		return failure
	}
	cursor := 0
	for index, element := range originalElements {
		other := candidateElements[index]
		if element.start != other.start || element.kind != other.kind || element.name != other.name || !sameLabels(element.labels, other.labels) ||
			!bytes.Equal(original.source[cursor:element.start], candidate[cursor:other.start]) {
			return failure
		}
		cursor = element.rangeValue.End.Byte
		if sameRangeBytes(original.source, element.rangeValue, candidate, other.rangeValue) {
			continue
		}
		if element.block == nil || element.name != "output" || other.block == nil {
			return failure
		}
		edit, diagnostic := outputBlockEdit(original, element.block)
		if diagnostic != nil || edit == nil || len(edit.text) != edit.end-edit.start {
			return failure
		}
		expected := bytes.Clone(original.source[element.start:element.rangeValue.End.Byte])
		copy(expected[edit.start-element.start:edit.end-element.start], edit.text)
		if !bytes.Equal(expected, candidate[other.start:other.rangeValue.End.Byte]) {
			return failure
		}
	}
	if !bytes.Equal(original.source[cursor:], candidate[cursor:]) {
		return failure
	}
	return nil
}

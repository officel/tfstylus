package app

import (
	"bytes"
	"sort"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func variableElementRank(element syntaxElement) int {
	if element.block != nil {
		if element.name == "validation" && len(element.labels) == 0 {
			return 8
		}
		return -1
	}
	for rank, name := range []string{"description", "deprecated", "sensitive", "nullable", "ephemeral", "const", "type", "default"} {
		if element.name == name {
			return rank
		}
	}
	return -1
}

func variableBodyChunks(parsed *parsedFile, block *hclsyntax.Block, elements []syntaxElement) ([]cloudElementChunk, *transformDiagnostic) {
	starts := make([]int, len(elements))
	ends := make([]int, len(elements))
	for index, element := range elements {
		starts[index] = lineStart(parsed.source, element.start)
		ends[index] = lineEnd(parsed.source, cloudElementEnd(element))
		if !horizontalWhitespace(parsed.source[starts[index]:element.start]) {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "variable elements must occupy standalone lines"}
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
			return nil, &transformDiagnostic{Subject: token.Range, Reason: "variable body contains an ambiguous comment attachment"}
		}
	}
	chunks := make([]cloudElementChunk, len(elements))
	for index, element := range elements {
		end := ends[index]
		if index+1 < len(elements) {
			if end > starts[index+1] {
				return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "variable comment chunks overlap"}
			}
			end = starts[index+1]
		}
		if starts[index] < block.OpenBraceRange.End.Byte || end > block.CloseBraceRange.Start.Byte || end < cloudElementEnd(element) {
			return nil, &transformDiagnostic{Subject: block.Range(), Reason: "variable element boundaries are ambiguous"}
		}
		chunks[index] = cloudElementChunk{start: starts[index], end: end, text: bytes.Clone(parsed.source[starts[index]:end])}
	}
	return chunks, nil
}

func variableBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	if len(block.Labels) != 1 || !hclsyntax.ValidIdentifier(block.Labels[0]) {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "variable must have one identifier label"}
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line variable body is not sorted"}
	}
	elements := syntaxElements(block.Body)
	needsSort := false
	previousRank := -1
	for _, element := range elements {
		rank := variableElementRank(element)
		if rank < 0 {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "variable body contains an unknown element or labeled validation"}
		}
		needsSort = needsSort || rank < previousRank
		previousRank = rank
	}
	if !needsSort {
		return nil, nil
	}
	chunks, diagnostic := variableBodyChunks(parsed, block, elements)
	if diagnostic != nil {
		return nil, diagnostic
	}
	indices := make([]int, len(elements))
	for index := range indices {
		indices[index] = index
	}
	sort.SliceStable(indices, func(left, right int) bool {
		return variableElementRank(elements[indices[left]]) < variableElementRank(elements[indices[right]])
	})
	var replacement []byte
	for _, index := range indices {
		replacement = append(replacement, chunks[index].text...)
	}
	return &sourceEdit{start: chunks[0].start, end: chunks[len(chunks)-1].end, text: replacement}, nil
}

func transformVariableOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return candidate, nil, []transformDiagnostic{{Reason: "unsupported root body syntax"}}
	}
	var edits []sourceEdit
	var changes []transformChange
	var diagnostics []transformDiagnostic
	for _, block := range root.Blocks {
		if block.Type != "variable" {
			continue
		}
		edit, diagnostic := variableBlockEdit(parsed, block)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
		} else if edit != nil {
			edits = append(edits, *edit)
			changes = append(changes, transformChange{
				Subject: block.Range(), Rule: "variable_order",
				Reason: "variable elements should be ordered description, deprecated, sensitive, nullable, ephemeral, const, type, default, validation",
			})
		}
	}
	for index := len(edits) - 1; index >= 0; index-- {
		edit := edits[index]
		candidate = append(bytes.Clone(candidate[:edit.start]), append(bytes.Clone(edit.text), candidate[edit.end:]...)...)
	}
	if len(edits) > 0 {
		if diagnostic := validateVariableCandidate(parsed, candidate); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func validateVariableCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	root := original.body.(*hclsyntax.Body)
	failure := &transformDiagnostic{Subject: root.Range(), Reason: "candidate changed variable elements, comment ownership, or non-target bytes"}
	validated, diagnostics := parseHCLSource(root.Range().Filename, candidate)
	if diagnostics.HasErrors() {
		return failure
	}
	originalElements := syntaxElements(root)
	candidateElements := syntaxElements(validated.body.(*hclsyntax.Body))
	if len(original.source) != len(candidate) || len(originalElements) != len(candidateElements) {
		return failure
	}
	cursor := 0
	for index, element := range originalElements {
		other := candidateElements[index]
		if element.start != other.start || element.rangeValue.End.Byte != other.rangeValue.End.Byte ||
			!bytes.Equal(original.source[cursor:element.start], candidate[cursor:other.start]) {
			return failure
		}
		cursor = element.rangeValue.End.Byte
		if sameRangeBytes(original.source, element.rangeValue, candidate, other.rangeValue) {
			continue
		}
		if element.block == nil || other.block == nil || element.name != "variable" || other.name != "variable" || !sameLabels(element.labels, other.labels) {
			return failure
		}
		edit, diagnostic := variableBlockEdit(original, element.block)
		if diagnostic != nil || edit == nil ||
			!bytes.Equal(original.source[element.start:edit.start], candidate[element.start:edit.start]) ||
			!bytes.Equal(edit.text, candidate[edit.start:edit.end]) ||
			!bytes.Equal(original.source[edit.end:cursor], candidate[edit.end:cursor]) {
			return failure
		}
		before := syntaxElements(element.block.Body)
		after := syntaxElements(other.block.Body)
		if len(before) != len(after) {
			return failure
		}
		sort.SliceStable(before, func(left, right int) bool {
			return variableElementRank(before[left]) < variableElementRank(before[right])
		})
		for elementIndex, previous := range before {
			if !sameRangeBytes(original.source, previous.rangeValue, candidate, after[elementIndex].rangeValue) {
				return failure
			}
		}
	}
	if !bytes.Equal(original.source[cursor:], candidate[cursor:]) {
		return failure
	}
	return nil
}

package app

import (
	"bytes"
	"sort"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func movedElementRank(element syntaxElement) (int, bool) {
	if element.attribute != nil {
		switch element.attribute.Name {
		case "from":
			return 0, true
		case "to":
			return 1, true
		default:
			return 2, true
		}
	}
	return 0, false
}

func movedBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	elements := syntaxElements(block.Body)
	if len(elements) < 2 {
		return nil, nil
	}

	needsSort := false
	previousRank := -1
	unsupported := false
	for _, element := range elements {
		rank, supported := movedElementRank(element)
		if !supported {
			unsupported = true
			continue
		}
		if rank < previousRank {
			needsSort = true
		}
		previousRank = rank
	}
	if !needsSort {
		return nil, nil
	}
	if unsupported {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "moved body contains an unsupported element"}
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line moved body is not sorted"}
	}

	chunks, diagnostic := bodyElementChunks(parsed, block, elements, "moved")
	if diagnostic != nil {
		return nil, diagnostic
	}
	indices := make([]int, len(elements))
	for index := range indices {
		indices[index] = index
	}
	sort.SliceStable(indices, func(left, right int) bool {
		leftRank, _ := movedElementRank(elements[indices[left]])
		rightRank, _ := movedElementRank(elements[indices[right]])
		return leftRank < rightRank
	})
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

func movedBlockCandidate(parsed *parsedFile, block *hclsyntax.Block) ([]byte, *transformDiagnostic) {
	if len(block.Labels) != 0 {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "moved block must not have labels"}
	}
	edit, diagnostic := movedBlockEdit(parsed, block)
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

func transformMovedOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return candidate, nil, []transformDiagnostic{{Reason: "unsupported root body syntax"}}
	}

	var edits []sourceEdit
	var changes []transformChange
	var diagnostics []transformDiagnostic
	for _, block := range root.Blocks {
		if block.Type != "moved" {
			continue
		}
		blockText, diagnostic := movedBlockCandidate(parsed, block)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
			continue
		}
		blockRange := block.Range()
		if !bytes.Equal(blockText, parsed.source[blockRange.Start.Byte:blockRange.End.Byte]) {
			edits = append(edits, sourceEdit{start: blockRange.Start.Byte, end: blockRange.End.Byte, text: bytes.Clone(blockText)})
			changes = append(changes, transformChange{
				Subject: blockRange, Rule: "moved_order",
				Reason: "moved elements should be ordered from, to, then unknown attributes",
			})
		}
	}

	sort.Slice(edits, func(left, right int) bool { return edits[left].start > edits[right].start })
	for _, edit := range edits {
		candidate = append(bytes.Clone(candidate[:edit.start]), append(bytes.Clone(edit.text), candidate[edit.end:]...)...)
	}
	if len(edits) > 0 {
		if diagnostic := validateMovedCandidate(parsed, candidate); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func validateMovedCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	originalRoot := original.body.(*hclsyntax.Body)
	failure := &transformDiagnostic{Subject: originalRoot.Range(), Reason: "candidate changed moved elements, comment ownership, or non-target bytes"}
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
	for index, element := range originalElements {
		other := candidateElements[index]
		if element.start != other.start || element.kind != other.kind || element.name != other.name || !sameLabels(element.labels, other.labels) ||
			!bytes.Equal(original.source[cursor:element.start], candidate[cursor:other.start]) {
			return failure
		}
		if element.block != nil && element.block.Type == "moved" {
			expected, diagnostic := movedBlockCandidate(original, element.block)
			if diagnostic != nil {
				expected = bytes.Clone(original.source[element.rangeValue.Start.Byte:element.rangeValue.End.Byte])
			}
			if other.block == nil || len(expected) != other.rangeValue.End.Byte-other.rangeValue.Start.Byte ||
				!bytes.Equal(expected, candidate[other.rangeValue.Start.Byte:other.rangeValue.End.Byte]) {
				return failure
			}
		} else if !sameRangeBytes(original.source, element.rangeValue, candidate, other.rangeValue) {
			return failure
		}
		cursor = element.rangeValue.End.Byte
	}
	if !bytes.Equal(original.source[cursor:], candidate[cursor:]) {
		return failure
	}
	return nil
}

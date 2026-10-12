package app

import (
	"bytes"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func removedElementRank(element syntaxElement) (int, bool) {
	if element.attribute != nil {
		if element.attribute.Name == "from" {
			return 0, true
		}
		return 1, true
	}
	if element.block != nil {
		if element.block.Type == "lifecycle" {
			return 2, true
		}
		return 3, true
	}
	return 0, false
}

func removedBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	elements := syntaxElements(block.Body)
	if len(elements) < 2 {
		return nil, nil
	}

	indices, needsSort, _ := rankedElementOrder(elements, removedElementRank)
	if !needsSort {
		return nil, nil
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line removed body is not sorted"}
	}

	chunks, diagnostic := bodyElementChunks(parsed, block, elements, "removed")
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

func removedBlockCandidate(parsed *parsedFile, block *hclsyntax.Block) ([]byte, *transformDiagnostic) {
	edit, diagnostic := removedBlockEdit(parsed, block)
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

func transformRemovedOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return candidate, nil, []transformDiagnostic{{Reason: "unsupported root body syntax"}}
	}

	var edits []sourceEdit
	var changes []transformChange
	var diagnostics []transformDiagnostic
	for _, block := range root.Blocks {
		if block.Type != "removed" {
			continue
		}
		blockText, diagnostic := removedBlockCandidate(parsed, block)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
			continue
		}
		blockRange := block.Range()
		if !bytes.Equal(blockText, parsed.source[blockRange.Start.Byte:blockRange.End.Byte]) {
			edits = append(edits, sourceEdit{start: blockRange.Start.Byte, end: blockRange.End.Byte, text: bytes.Clone(blockText)})
			changes = append(changes, transformChange{
				Subject: blockRange, Rule: "removed_order",
				Reason: "removed elements should be ordered from, lifecycle, then connection/provisioner in their original relative order",
			})
		}
	}

	candidate = applySourceEdits(candidate, edits)
	if len(edits) > 0 {
		if diagnostic := validateRemovedCandidate(parsed, candidate); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func validateRemovedCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	originalRoot := original.body.(*hclsyntax.Body)
	failure := &transformDiagnostic{Subject: originalRoot.Range(), Reason: "candidate changed removed elements, comment ownership, or non-target bytes"}
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
		if element.block != nil && element.block.Type == "removed" {
			expected, diagnostic := removedBlockCandidate(original, element.block)
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

package app

import (
	"bytes"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func actionElementRank(element syntaxElement) (int, bool) {
	if element.attribute != nil {
		switch element.name {
		case "count", "for_each":
			return 0, true
		case "provider":
			return 1, true
		default:
			return 2, true
		}
	}
	if element.block != nil {
		return 3, true
	}
	return 0, false
}

func actionBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line action body is not sorted"}
	}
	elements := syntaxElements(block.Body)
	indices, needsSort, _ := rankedElementOrder(elements, actionElementRank)
	if !needsSort {
		return nil, nil
	}
	chunks, diagnostic := bodyElementChunks(parsed, block, elements, "action")
	if diagnostic != nil {
		return nil, diagnostic
	}
	var replacement []byte
	for _, index := range indices {
		replacement = append(replacement, chunks[index].text...)
	}
	return &sourceEdit{start: chunks[0].start, end: chunks[len(chunks)-1].end, text: replacement}, nil
}

func transformActionOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return candidate, nil, []transformDiagnostic{{Reason: "unsupported root body syntax"}}
	}
	var edits []sourceEdit
	var changes []transformChange
	var diagnostics []transformDiagnostic
	for _, block := range root.Blocks {
		if block.Type != "action" {
			continue
		}
		edit, diagnostic := actionBlockEdit(parsed, block)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
		} else if edit != nil {
			edits = append(edits, *edit)
			changes = append(changes, transformChange{
				Subject: block.Range(), Rule: "action_order",
				Reason: "action elements should be ordered count/for_each, provider, then config",
			})
		}
	}
	candidate = applySourceEdits(candidate, edits)
	if len(edits) > 0 {
		if diagnostic := validateActionCandidate(parsed, candidate); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func validateActionCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	root := original.body.(*hclsyntax.Body)
	failure := &transformDiagnostic{Subject: root.Range(), Reason: "candidate changed action elements, comment ownership, or non-target bytes"}
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
		if element.block == nil || element.name != "action" || other.block == nil {
			return failure
		}
		edit, diagnostic := actionBlockEdit(original, element.block)
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

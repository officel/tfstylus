package app

import (
	"bytes"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func checkElementRank(element syntaxElement) (int, bool) {
	if element.attribute != nil {
		return 2, true
	}
	if element.block == nil {
		return 0, false
	}
	switch element.block.Type {
	case "data":
		return 0, true
	case "assert":
		return 1, true
	default:
		return 3, true
	}
}

func checkDataElementRank(element syntaxElement) (int, bool) {
	if element.attribute != nil {
		switch element.attribute.Name {
		case "depends_on":
			return 1, true
		case "provider":
			return 2, true
		default:
			return 0, true
		}
	}
	if element.block != nil {
		return 0, true
	}
	return 0, false
}

func checkAssertElementRank(element syntaxElement) (int, bool) {
	if element.attribute == nil {
		return 0, false
	}
	switch element.attribute.Name {
	case "condition":
		return 0, true
	case "error_message":
		return 1, true
	default:
		if element.attribute != nil {
			return 2, true
		}
		return 3, true
	}
}

func checkBodyEdit(parsed *parsedFile, block *hclsyntax.Block, owner string, rank func(syntaxElement) (int, bool)) (*sourceEdit, *transformDiagnostic) {
	elements := syntaxElements(block.Body)
	if len(elements) < 2 {
		for _, element := range elements {
			if _, supported := rank(element); !supported {
				return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: owner + " body contains an unsupported element"}
			}
		}
		return nil, nil
	}

	indices, needsSort, unsupportedIndex := rankedElementOrder(elements, rank)
	if unsupportedIndex != -1 {
		return nil, &transformDiagnostic{Subject: elements[unsupportedIndex].rangeValue, Reason: owner + " body contains an unsupported element"}
	}
	if !needsSort {
		return nil, nil
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line " + owner + " body is not sorted"}
	}

	chunks, diagnostic := bodyElementChunks(parsed, block, elements, owner)
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

func checkBlockCandidate(parsed *parsedFile, block *hclsyntax.Block, blockIndex int) ([]byte, *transformDiagnostic) {
	var nestedEdits []sourceEdit
	for _, element := range syntaxElements(block.Body) {
		if element.block == nil {
			continue
		}
		var edit *sourceEdit
		var diagnostic *transformDiagnostic
		switch element.block.Type {
		case "data":
			edit, diagnostic = checkBodyEdit(parsed, element.block, "check data", checkDataElementRank)
		case "assert":
			edit, diagnostic = checkBodyEdit(parsed, element.block, "check assert", checkAssertElementRank)
		default:
			continue
		}
		if diagnostic != nil {
			return nil, diagnostic
		}
		if edit != nil {
			nestedEdits = append(nestedEdits, *edit)
		}
	}

	blockRange := block.Range()
	candidate := applySourceEdits(parsed.source, nestedEdits)
	parsedCandidate := parsed
	if len(nestedEdits) > 0 {
		validated, diagnostics := parseHCLSource(blockRange.Filename, candidate)
		if diagnostics.HasErrors() {
			return nil, &transformDiagnostic{Subject: blockRange, Reason: "nested check candidate failed HCL reparse: " + diagnostics.Error()}
		}
		root, ok := validated.body.(*hclsyntax.Body)
		if !ok || blockIndex >= len(root.Blocks) {
			return nil, &transformDiagnostic{Subject: blockRange, Reason: "nested check candidate changed root structure"}
		}
		parsedCandidate = validated
		block = root.Blocks[blockIndex]
	}
	outerEdit, diagnostic := checkBodyEdit(parsedCandidate, block, "check", checkElementRank)
	if diagnostic != nil {
		return nil, diagnostic
	}
	if outerEdit != nil {
		candidate = applySourceEdits(candidate, []sourceEdit{*outerEdit})
	}
	return bytes.Clone(candidate[blockRange.Start.Byte:blockRange.End.Byte]), nil
}
func transformCheckOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return candidate, nil, []transformDiagnostic{{Reason: "unsupported root body syntax"}}
	}

	var edits []sourceEdit
	var changes []transformChange
	var diagnostics []transformDiagnostic
	for blockIndex, block := range root.Blocks {
		if block.Type != "check" {
			continue
		}
		blockText, diagnostic := checkBlockCandidate(parsed, block, blockIndex)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
			continue
		}
		blockRange := block.Range()
		if !bytes.Equal(blockText, parsed.source[blockRange.Start.Byte:blockRange.End.Byte]) {
			edits = append(edits, sourceEdit{start: blockRange.Start.Byte, end: blockRange.End.Byte, text: blockText})
			changes = append(changes, transformChange{
				Subject: blockRange, Rule: "check_order",
				Reason: "check elements should be ordered data then assert; nested data and assert arguments follow their configured order",
			})
		}
	}
	candidate = applySourceEdits(candidate, edits)
	if len(edits) > 0 {
		if diagnostic := validateCheckCandidate(parsed, candidate); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func validateCheckCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	root := original.body.(*hclsyntax.Body)
	failure := &transformDiagnostic{Subject: root.Range(), Reason: "candidate changed check elements, comment ownership, or non-target bytes"}
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
	blockIndex := 0
	for index, element := range originalElements {
		other := candidateElements[index]
		if element.start != other.start || element.kind != other.kind || element.name != other.name || !sameLabels(element.labels, other.labels) ||
			!bytes.Equal(original.source[cursor:element.start], candidate[cursor:other.start]) {
			return failure
		}
		if element.block != nil && element.block.Type == "check" {
			expected, diagnostic := checkBlockCandidate(original, element.block, blockIndex)
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

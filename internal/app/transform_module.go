package app

import (
	"bytes"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func moduleElementRank(element syntaxElement) (int, bool) {
	if element.attribute == nil {
		if element.block != nil {
			return 5, true
		}
		return 0, false
	}
	switch element.attribute.Name {
	case "count", "for_each":
		return 0, true
	case "source":
		return 1, true
	case "version":
		return 2, true
	case "providers":
		return 4, true
	case "ignore_nested_deprecations":
		return 6, true
	case "depends_on":
		return 7, true
	default:
		return 3, true
	}
}

func moduleBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	elements := syntaxElements(block.Body)
	indices, needsSort, unsupportedIndex := rankedElementOrder(elements, moduleElementRank)
	if !needsSort {
		return nil, nil
	}
	if unsupportedIndex != -1 {
		return nil, &transformDiagnostic{Subject: elements[unsupportedIndex].rangeValue, Reason: "module body contains an unsupported nested block"}
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line module body is not sorted"}
	}
	chunks, diagnostic := bodyElementChunks(parsed, block, elements, "module")
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

func moduleBlockCandidate(parsed *parsedFile, block *hclsyntax.Block) ([]byte, *transformDiagnostic) {
	edit, diagnostic := moduleBlockEdit(parsed, block)
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

func transformModuleOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return candidate, nil, []transformDiagnostic{{Reason: "unsupported root body syntax"}}
	}
	var edits []sourceEdit
	var changes []transformChange
	var diagnostics []transformDiagnostic
	for _, block := range root.Blocks {
		if block.Type != "module" {
			continue
		}
		blockText, diagnostic := moduleBlockCandidate(parsed, block)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
			continue
		}
		blockRange := block.Range()
		if !bytes.Equal(blockText, parsed.source[blockRange.Start.Byte:blockRange.End.Byte]) {
			edits = append(edits, sourceEdit{start: blockRange.Start.Byte, end: blockRange.End.Byte, text: bytes.Clone(blockText)})
			changes = append(changes, transformChange{
				Subject: blockRange, Rule: "module_order",
				Reason: "module attributes should be ordered count/for_each, source, version, module-specific inputs, providers, ignore_nested_deprecations, then depends_on",
			})
		}
	}
	candidate = applySourceEdits(candidate, edits)
	if len(edits) > 0 {
		if diagnostic := validateModuleCandidate(parsed, candidate); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func validateModuleCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	return validateSourceCandidate(original, candidate, map[string]sourceBodyPolicy{
		"module": {owner: "module", rank: moduleElementRank},
	})
}

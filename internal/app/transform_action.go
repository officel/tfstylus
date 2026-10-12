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
	return validateSourceCandidate(original, candidate, map[string]sourceBodyPolicy{
		"action": {owner: "action", rank: actionElementRank},
	})
}

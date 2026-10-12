package app

import (
	"bytes"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func variableElementRank(element syntaxElement) int {
	if element.block != nil {
		if element.name == "validation" {
			return 9
		}
		return 10
	}
	for rank, name := range []string{"description", "deprecated", "sensitive", "nullable", "ephemeral", "const", "type", "default"} {
		if element.name == name {
			return rank
		}
	}
	return 8
}

func variableBodyChunks(parsed *parsedFile, block *hclsyntax.Block, elements []syntaxElement) ([]sourceChunk, *transformDiagnostic) {
	starts := make([]int, len(elements))
	ends := make([]int, len(elements))
	for index, element := range elements {
		starts[index] = lineStart(parsed.source, element.start)
		ends[index] = lineEnd(parsed.source, cloudElementEnd(element))
		if !horizontalWhitespace(parsed.source[starts[index]:element.start]) {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "variable elements must occupy standalone lines"}
		}
	}
	if diagnostic := assignSourceComments(parsed, block, elements, starts, ends, sourceCommentPolicy{
		owner:                  "variable",
		elementStart:           func(element syntaxElement) int { return element.start },
		elementEnd:             cloudElementEnd,
		trailingCommentLineEnd: cloudCommentLineEnd,
		reverseTokens:          true,
	}); diagnostic != nil {
		return nil, diagnostic
	}
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
	}
	return sourceChunksFromBounds(parsed.source, starts, ends), nil
}

func variableBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line variable body is not sorted"}
	}
	elements := syntaxElements(block.Body)
	indices, needsSort, _ := rankedElementOrder(elements, func(element syntaxElement) (int, bool) {
		rank := variableElementRank(element)
		return rank, true
	})
	if !needsSort {
		return nil, nil
	}
	chunks, diagnostic := variableBodyChunks(parsed, block, elements)
	if diagnostic != nil {
		return nil, diagnostic
	}
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
	candidate = applySourceEdits(candidate, edits)
	if len(edits) > 0 {
		if diagnostic := validateVariableCandidate(parsed, candidate); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func validateVariableCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	return validateSourceCandidate(original, candidate, map[string]sourceBodyPolicy{
		"variable": {
			owner: "variable",
			rank:  func(element syntaxElement) (int, bool) { return variableElementRank(element), true },
		},
	})
}

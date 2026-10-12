package app

import (
	"bytes"

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
			return 8, true
		default:
			return 6, true
		}
	}
	if element.block != nil {
		return 7, true
	}
	return 0, false
}

func outputBodyChunks(parsed *parsedFile, block *hclsyntax.Block, elements []syntaxElement) ([]sourceChunk, *transformDiagnostic) {
	starts := make([]int, len(elements))
	ends := make([]int, len(elements))
	for index, element := range elements {
		starts[index] = lineStart(parsed.source, element.start)
		ends[index] = lineEnd(parsed.source, cloudElementEnd(element))
		if !horizontalWhitespace(parsed.source[starts[index]:element.start]) {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "output elements must occupy standalone lines"}
		}
	}
	if diagnostic := assignSourceComments(parsed, block, elements, starts, ends, sourceCommentPolicy{
		owner:                  "output",
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
				return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "output comment chunks overlap"}
			}
			end = starts[index+1]
		}
		if starts[index] < block.OpenBraceRange.End.Byte || end > block.CloseBraceRange.Start.Byte || end < cloudElementEnd(element) {
			return nil, &transformDiagnostic{Subject: block.Range(), Reason: "output element boundaries are ambiguous"}
		}
	}
	return sourceChunksFromBounds(parsed.source, starts, ends), nil
}

func outputBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line output body is not sorted"}
	}
	elements := syntaxElements(block.Body)
	indices, needsSort, unsupportedIndex := rankedElementOrder(elements, outputElementRank)
	if unsupportedIndex != -1 {
		return nil, &transformDiagnostic{Subject: elements[unsupportedIndex].rangeValue, Reason: "output body contains an unknown element or labeled precondition"}
	}
	if !needsSort {
		return nil, nil
	}
	chunks, diagnostic := outputBodyChunks(parsed, block, elements)
	if diagnostic != nil {
		return nil, diagnostic
	}
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
	candidate = applySourceEdits(candidate, edits)
	if len(edits) > 0 {
		if diagnostic := validateOutputCandidate(parsed, candidate); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func validateOutputCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	return validateSourceCandidate(original, candidate, map[string]sourceBodyPolicy{
		"output": {owner: "output", rank: outputElementRank},
	})
}

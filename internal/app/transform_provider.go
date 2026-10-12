package app

import (
	"bytes"
	"sort"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func providerElementRank(element syntaxElement) (int, bool) {
	if element.attribute != nil {
		switch element.attribute.Name {
		case "alias":
			return 1, true
		case "region":
			return 2, true
		default:
			return 0, true
		}
	}
	if element.block != nil {
		return 3, true
	}
	return 0, false
}

func providerBodyChunks(parsed *parsedFile, block *hclsyntax.Block, elements []syntaxElement) ([]cloudElementChunk, *transformDiagnostic) {
	leadingStarts := make([]int, len(elements))
	trailingEnds := make([]int, len(elements))
	for index, element := range elements {
		leadingStarts[index] = lineStart(parsed.source, element.rangeValue.Start.Byte)
		trailingEnds[index] = lineEnd(parsed.source, cloudElementEnd(element))
	}

	for _, token := range parsed.tokens {
		if token.Type != hclsyntax.TokenComment || token.Range.Start.Byte < block.OpenBraceRange.End.Byte || token.Range.End.Byte > block.CloseBraceRange.Start.Byte {
			continue
		}

		owned := false
		for _, element := range elements {
			if token.Range.Start.Byte >= element.rangeValue.Start.Byte && token.Range.End.Byte <= cloudElementEnd(element) {
				owned = true
				break
			}
		}
		if owned {
			continue
		}

		for index, element := range elements {
			end := cloudElementEnd(element)
			if lineStart(parsed.source, token.Range.Start.Byte) == lineStart(parsed.source, end) && token.Range.Start.Byte >= end && horizontalWhitespace(parsed.source[end:token.Range.Start.Byte]) {
				trailingEnds[index] = cloudCommentLineEnd(parsed.source, token.Range.End.Byte)
				owned = true
				break
			}
		}
		if owned {
			continue
		}

		nextElement := -1
		for index, element := range elements {
			if element.rangeValue.Start.Byte > token.Range.End.Byte {
				nextElement = index
				break
			}
		}
		if nextElement >= 0 {
			commentStart := lineStart(parsed.source, token.Range.Start.Byte)
			elementStart := elements[nextElement].rangeValue.Start.Byte
			endsWithNewline := token.Range.End.Byte > token.Range.Start.Byte && parsed.source[token.Range.End.Byte-1] == '\n'
			if horizontalWhitespace(parsed.source[commentStart:token.Range.Start.Byte]) && commentGapWhitespace(parsed.source[token.Range.End.Byte:elementStart], endsWithNewline) {
				if commentStart < leadingStarts[nextElement] {
					leadingStarts[nextElement] = commentStart
				}
				continue
			}
		}

		return nil, &transformDiagnostic{Subject: token.Range, Reason: "provider body contains an ambiguous comment attachment"}
	}

	chunks := make([]cloudElementChunk, len(elements))
	previousElementLine := -1
	for index, element := range elements {
		start := leadingStarts[index]
		elementLine := lineStart(parsed.source, element.rangeValue.Start.Byte)
		if elementLine == previousElementLine || !horizontalWhitespace(parsed.source[elementLine:element.rangeValue.Start.Byte]) {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "provider elements do not occupy unambiguous standalone lines"}
		}
		previousElementLine = elementLine
		end := trailingEnds[index]
		if index+1 < len(elements) {
			end = leadingStarts[index+1]
		}
		if end < start {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "provider element comment chunks overlap"}
		}
		chunks[index] = cloudElementChunk{start: start, end: end, text: bytes.Clone(parsed.source[start:end])}
	}
	if lineStart(parsed.source, block.CloseBraceRange.Start.Byte) == previousElementLine {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "provider elements do not occupy unambiguous standalone lines"}
	}
	chunks[len(chunks)-1].end = trailingEnds[len(trailingEnds)-1]
	chunks[len(chunks)-1].text = bytes.Clone(parsed.source[chunks[len(chunks)-1].start:chunks[len(chunks)-1].end])
	return chunks, nil
}

func providerBlockEdit(parsed *parsedFile, block *hclsyntax.Block) (*sourceEdit, *transformDiagnostic) {
	elements := syntaxElements(block.Body)
	if len(elements) < 2 {
		return nil, nil
	}

	indices, needsSort, unsupportedIndex := rankedElementOrder(elements, providerElementRank)
	if unsupportedIndex != -1 {
		return nil, &transformDiagnostic{Subject: elements[unsupportedIndex].rangeValue, Reason: "provider body contains an unsupported element"}
	}
	if !needsSort {
		return nil, nil
	}
	if block.OpenBraceRange.Start.Line == block.CloseBraceRange.Start.Line {
		return nil, &transformDiagnostic{Subject: block.Range(), Reason: "single-line provider body is not sorted"}
	}

	chunks, diagnostic := providerBodyChunks(parsed, block, elements)
	if diagnostic != nil {
		return nil, diagnostic
	}
	var replacement []byte
	for _, index := range indices {
		replacement = append(replacement, chunks[index].text...)
	}
	return &sourceEdit{start: chunks[0].start, end: chunks[len(chunks)-1].end, text: replacement}, nil
}

func transformProviderOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return candidate, nil, []transformDiagnostic{{Reason: "unsupported root body syntax"}}
	}

	var edits []sourceEdit
	var changes []transformChange
	var diagnostics []transformDiagnostic
	for _, block := range root.Blocks {
		if block.Type != "provider" {
			continue
		}
		edit, diagnostic := providerBlockEdit(parsed, block)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
		} else if edit != nil {
			edits = append(edits, *edit)
			changes = append(changes, transformChange{
				Subject: block.Range(), Rule: "provider_order",
				Reason: "provider elements should be ordered provider-specific attributes, alias, region, then blocks",
			})
		}
	}

	sort.Slice(edits, func(i, j int) bool {
		return edits[i].start > edits[j].start
	})
	for _, edit := range edits {
		candidate = append(bytes.Clone(candidate[:edit.start]), append(bytes.Clone(edit.text), candidate[edit.end:]...)...)
	}
	if len(edits) > 0 {
		if _, parseDiagnostics := parseHCLSource(root.Range().Filename, candidate); parseDiagnostics.HasErrors() {
			return bytes.Clone(parsed.source), nil, append(diagnostics, transformDiagnostic{
				Subject: root.Range(), Reason: "provider order candidate failed HCL reparse: " + parseDiagnostics.Error(),
			})
		}
	}
	return candidate, changes, diagnostics
}

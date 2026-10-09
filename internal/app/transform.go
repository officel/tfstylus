package app

import (
	"bytes"
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type transformDiagnostic struct {
	Subject hcl.Range
	Reason  string
}

type transformChange struct {
	Subject hcl.Range
	Rule    string
	Reason  string
}

type sourceEdit struct {
	start int
	end   int
	text  []byte
}

func transformRequiredVersionOrder(parsed *parsedFile, config Config) ([]byte, []transformDiagnostic) {
	candidate, _, diagnostics := transformRequiredVersionOrderWithChanges(parsed, config)
	return candidate, diagnostics
}

func transformRequiredVersionOrderWithChanges(parsed *parsedFile, config Config) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	if !config.RequiredVersionOrder {
		return candidate, nil, nil
	}

	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return candidate, nil, []transformDiagnostic{{Reason: "unsupported root body syntax"}}
	}

	var edits []sourceEdit
	var changes []transformChange
	var diagnostics []transformDiagnostic
	for _, block := range root.Blocks {
		if block.Type != "terraform" {
			continue
		}
		attribute, exists := block.Body.Attributes["required_version"]
		if !exists {
			continue
		}

		edit, diagnostic := requiredVersionEdit(parsed, block, attribute)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
			continue
		}
		if edit != nil {
			edits = append(edits, *edit)
			changes = append(changes, transformChange{
				Subject: attribute.SrcRange,
				Rule:    "required_version_order",
				Reason:  "required_version should be the first element in the terraform block",
			})
		}
	}

	sort.Slice(edits, func(i, j int) bool {
		return edits[i].start > edits[j].start
	})
	for _, edit := range edits {
		candidate = append(candidate[:edit.start], append(edit.text, candidate[edit.end:]...)...)
	}
	if len(edits) > 0 {
		if diagnostic := validateRequiredVersionCandidate(parsed, candidate); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func validateRequiredVersionCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	originalBody, originalOK := original.body.(*hclsyntax.Body)
	if !originalOK {
		return &transformDiagnostic{Reason: "unsupported original body syntax"}
	}
	filename := originalBody.Range().Filename
	if filename == "" {
		filename = "candidate.tf"
	}
	validated, parseDiagnostics := parseHCLSource(filename, candidate)
	if parseDiagnostics.HasErrors() {
		var subject hcl.Range
		for _, diagnostic := range parseDiagnostics {
			if diagnostic.Severity == hcl.DiagError && diagnostic.Subject != nil {
				subject = *diagnostic.Subject
				break
			}
		}
		return &transformDiagnostic{
			Subject: subject,
			Reason:  "candidate failed HCL reparse: " + parseDiagnostics.Error(),
		}
	}

	validatedBody, validatedOK := validated.body.(*hclsyntax.Body)
	if !validatedOK ||
		!sameRootStructure(originalBody, validatedBody, original.source, validated.source) ||
		!sameCommentTokens(original.tokens, validated.tokens, original.source, validated.source) {
		return &transformDiagnostic{
			Subject: originalBody.Range(),
			Reason:  "candidate changed HCL structure or comments outside required_version ordering",
		}
	}
	return nil
}

type syntaxElement struct {
	start      int
	kind       string
	name       string
	labels     []string
	rangeValue hcl.Range
	attribute  *hclsyntax.Attribute
	block      *hclsyntax.Block
}

func sameRootStructure(original, candidate *hclsyntax.Body, originalSource, candidateSource []byte) bool {
	originalElements := syntaxElements(original)
	candidateElements := syntaxElements(candidate)
	if len(originalElements) != len(candidateElements) {
		return false
	}
	for index, originalElement := range originalElements {
		candidateElement := candidateElements[index]
		if originalElement.kind != candidateElement.kind || originalElement.name != candidateElement.name || !sameLabels(originalElement.labels, candidateElement.labels) {
			return false
		}
		if originalElement.attribute != nil {
			if candidateElement.attribute == nil || !sameRangeBytes(originalSource, originalElement.rangeValue, candidateSource, candidateElement.rangeValue) {
				return false
			}
			continue
		}
		if originalElement.block == nil || candidateElement.block == nil {
			return false
		}
		if originalElement.block.Type == "terraform" {
			if !sameTerraformBody(originalElement.block.Body, candidateElement.block.Body, originalSource, candidateSource) {
				return false
			}
		} else if !sameRangeBytes(originalSource, originalElement.rangeValue, candidateSource, candidateElement.rangeValue) {
			return false
		}
	}
	return true
}

func sameTerraformBody(original, candidate *hclsyntax.Body, originalSource, candidateSource []byte) bool {
	originalRequired, originalHasRequired := original.Attributes["required_version"]
	candidateRequired, candidateHasRequired := candidate.Attributes["required_version"]
	if originalHasRequired != candidateHasRequired {
		return false
	}
	if originalHasRequired && !sameRangeBytes(originalSource, originalRequired.Expr.Range(), candidateSource, candidateRequired.Expr.Range()) {
		return false
	}

	originalElements := syntaxElements(original)
	candidateElements := syntaxElements(candidate)
	filteredOriginal := make([]syntaxElement, 0, len(originalElements))
	filteredCandidate := make([]syntaxElement, 0, len(candidateElements))
	for _, element := range originalElements {
		if element.attribute == nil || element.attribute.Name != "required_version" {
			filteredOriginal = append(filteredOriginal, element)
		}
	}
	for _, element := range candidateElements {
		if element.attribute == nil || element.attribute.Name != "required_version" {
			filteredCandidate = append(filteredCandidate, element)
		}
	}
	if len(filteredOriginal) != len(filteredCandidate) {
		return false
	}
	for index, originalElement := range filteredOriginal {
		candidateElement := filteredCandidate[index]
		if originalElement.kind != candidateElement.kind || originalElement.name != candidateElement.name || !sameLabels(originalElement.labels, candidateElement.labels) ||
			!sameRangeBytes(originalSource, originalElement.rangeValue, candidateSource, candidateElement.rangeValue) {
			return false
		}
	}
	return true
}

func syntaxElements(body *hclsyntax.Body) []syntaxElement {
	elements := make([]syntaxElement, 0, len(body.Attributes)+len(body.Blocks))
	for _, attribute := range body.Attributes {
		elements = append(elements, syntaxElement{
			start:      attribute.SrcRange.Start.Byte,
			kind:       "attribute",
			name:       attribute.Name,
			rangeValue: attribute.SrcRange,
			attribute:  attribute,
		})
	}
	for _, block := range body.Blocks {
		rangeValue := block.Range()
		elements = append(elements, syntaxElement{
			start:      rangeValue.Start.Byte,
			kind:       "block",
			name:       block.Type,
			labels:     block.Labels,
			rangeValue: rangeValue,
			block:      block,
		})
	}
	sort.Slice(elements, func(i, j int) bool {
		return elements[i].start < elements[j].start
	})
	return elements
}

func sameLabels(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func sameRangeBytes(leftSource []byte, leftRange hcl.Range, rightSource []byte, rightRange hcl.Range) bool {
	leftStart, leftEnd := leftRange.Start.Byte, leftRange.End.Byte
	rightStart, rightEnd := rightRange.Start.Byte, rightRange.End.Byte
	if leftStart < 0 || leftEnd < leftStart || leftEnd > len(leftSource) || rightStart < 0 || rightEnd < rightStart || rightEnd > len(rightSource) {
		return false
	}
	return bytes.Equal(leftSource[leftStart:leftEnd], rightSource[rightStart:rightEnd])
}

func sameCommentTokens(left, right hclsyntax.Tokens, leftSource, rightSource []byte) bool {
	counts := make(map[string]int)
	for _, token := range left {
		if token.Type == hclsyntax.TokenComment {
			counts[string(leftSource[token.Range.Start.Byte:token.Range.End.Byte])]++
		}
	}
	for _, token := range right {
		if token.Type == hclsyntax.TokenComment {
			text := string(rightSource[token.Range.Start.Byte:token.Range.End.Byte])
			if counts[text] == 0 {
				return false
			}
			counts[text]--
		}
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

func requiredVersionEdit(parsed *parsedFile, block *hclsyntax.Block, attribute *hclsyntax.Attribute) (*sourceEdit, *transformDiagnostic) {
	attributeLineStart := lineStart(parsed.source, attribute.SrcRange.Start.Byte)
	attributeChunkStart := attributeLineStart
	leadingComments := make(map[int]struct{})
	cursor := attributeLineStart
	for index := len(parsed.tokens) - 1; index >= 0; index-- {
		token := parsed.tokens[index]
		if token.Type != hclsyntax.TokenComment || token.Range.Start.Byte < block.OpenBraceRange.End.Byte || token.Range.End.Byte > block.CloseBraceRange.Start.Byte || token.Range.End.Byte > cursor {
			continue
		}
		endsWithNewline := token.Range.End.Byte > token.Range.Start.Byte && parsed.source[token.Range.End.Byte-1] == '\n'
		if !commentGapWhitespace(parsed.source[token.Range.End.Byte:cursor], endsWithNewline) {
			break
		}
		tokenLineStart := lineStart(parsed.source, token.Range.Start.Byte)
		if !horizontalWhitespace(parsed.source[tokenLineStart:token.Range.Start.Byte]) {
			break
		}
		leadingComments[token.Range.Start.Byte] = struct{}{}
		attributeChunkStart = tokenLineStart
		cursor = tokenLineStart
	}

	attributeLineEnd := lineEnd(parsed.source, attribute.SrcRange.End.Byte)
	attributeTrailingStart := attribute.SrcRange.End.Byte
	for _, token := range parsed.tokens {
		if token.Type == hclsyntax.TokenComment && token.Range.Start.Byte >= block.OpenBraceRange.End.Byte && token.Range.End.Byte <= block.CloseBraceRange.Start.Byte {
			if _, exists := leadingComments[token.Range.Start.Byte]; exists {
				continue
			}
			if token.Range.Start.Byte >= attribute.SrcRange.End.Byte &&
				lineStart(parsed.source, token.Range.Start.Byte) == lineStart(parsed.source, attribute.SrcRange.End.Byte) &&
				horizontalWhitespace(parsed.source[attribute.SrcRange.End.Byte:token.Range.Start.Byte]) {
				attributeTrailingStart = token.Range.Start.Byte
				attributeLineEnd = lineEnd(parsed.source, token.Range.Start.Byte)
				continue
			}
			return nil, &transformDiagnostic{
				Subject: token.Range,
				Reason:  "terraform body contains a comment whose ownership is not yet classified",
			}
		}
	}

	type elementRange struct {
		start int
		end   int
	}
	elements := make([]elementRange, 0, len(block.Body.Attributes)+len(block.Body.Blocks))
	for _, item := range block.Body.Attributes {
		elements = append(elements, elementRange{start: item.SrcRange.Start.Byte, end: item.SrcRange.End.Byte})
	}
	for _, item := range block.Body.Blocks {
		rangeValue := item.Range()
		elements = append(elements, elementRange{start: rangeValue.Start.Byte, end: rangeValue.End.Byte})
	}
	sort.Slice(elements, func(i, j int) bool {
		return elements[i].start < elements[j].start
	})

	attributeStart := attribute.SrcRange.Start.Byte
	attributeIndex := -1
	firstStart := -1
	for index, element := range elements {
		if element.start == attributeStart {
			attributeIndex = index
		}
		if firstStart == -1 {
			firstStart = element.start
		}
	}
	if attributeIndex == -1 || attributeIndex == 0 || firstStart == -1 {
		return nil, nil
	}

	firstLineStart := lineStart(parsed.source, firstStart)
	if !horizontalWhitespace(parsed.source[firstLineStart:firstStart]) || !horizontalWhitespace(parsed.source[attributeLineStart:attributeStart]) || !horizontalWhitespace(parsed.source[attribute.SrcRange.End.Byte:attributeTrailingStart]) {
		return nil, &transformDiagnostic{
			Subject: attribute.SrcRange,
			Reason:  "terraform elements do not occupy unambiguous standalone lines",
		}
	}

	return &sourceEdit{
		start: firstLineStart,
		end:   attributeLineEnd,
		text: append(
			bytes.Clone(parsed.source[attributeChunkStart:attributeLineEnd]),
			parsed.source[firstLineStart:attributeChunkStart]...,
		),
	}, nil
}

func commentGapWhitespace(source []byte, endsWithNewline bool) bool {
	newlines := 0
	maximumNewlines := 1
	if endsWithNewline {
		maximumNewlines = 0
	}
	for _, character := range source {
		switch character {
		case '\n':
			newlines++
			if newlines > maximumNewlines {
				return false
			}
		case ' ', '\t', '\r':
		default:
			return false
		}
	}
	return true
}

func lineStart(source []byte, offset int) int {
	if offset < 0 || offset > len(source) {
		return 0
	}
	return bytes.LastIndexByte(source[:offset], '\n') + 1
}

func lineEnd(source []byte, offset int) int {
	if offset < 0 || offset > len(source) {
		return len(source)
	}
	if newline := bytes.IndexByte(source[offset:], '\n'); newline >= 0 {
		return offset + newline + 1
	}
	return len(source)
}

func horizontalWhitespace(source []byte) bool {
	for _, character := range source {
		if character != ' ' && character != '\t' && character != '\r' {
			return false
		}
	}
	return true
}

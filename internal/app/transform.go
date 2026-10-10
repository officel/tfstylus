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
	start    int
	end      int
	insertAt int
	text     []byte
}

func transformRequiredVersionOrder(parsed *parsedFile, config Config) ([]byte, []transformDiagnostic) {
	candidate, _, diagnostics := transformRequiredVersionOrderWithChanges(parsed, config)
	return candidate, diagnostics
}

func transformTerraformOrderWithChanges(parsed *parsedFile, config Config) ([]byte, []transformChange, []transformDiagnostic) {
	candidate, changes, diagnostics := transformRequiredVersionOrderWithChanges(parsed, config)
	if !config.BackendOrder {
		return candidate, changes, diagnostics
	}

	parsedCandidate, parseDiagnostics := parseHCLSource("candidate.tf", candidate)
	if parseDiagnostics.HasErrors() {
		return candidate, changes, append(diagnostics, transformDiagnostic{
			Subject: parsed.body.(*hclsyntax.Body).Range(),
			Reason:  "candidate failed HCL reparse: " + parseDiagnostics.Error(),
		})
	}

	backendCandidate, backendChanges, backendDiagnostics := transformBackendOrderWithChanges(parsedCandidate, config)
	return backendCandidate, append(changes, backendChanges...), append(diagnostics, backendDiagnostics...)
}

func transformBackendOrder(parsed *parsedFile, config Config) ([]byte, []transformDiagnostic) {
	candidate, _, diagnostics := transformBackendOrderWithChanges(parsed, config)
	return candidate, diagnostics
}

func transformBackendOrderWithChanges(parsed *parsedFile, config Config) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	if !config.BackendOrder {
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
		remoteBlocks := make([]*hclsyntax.Block, 0, 2)
		for _, nested := range block.Body.Blocks {
			if nested.Type == "backend" || nested.Type == "cloud" {
				remoteBlocks = append(remoteBlocks, nested)
			}
		}
		if len(remoteBlocks) == 0 {
			continue
		}
		if len(remoteBlocks) > 1 {
			diagnostics = append(diagnostics, transformDiagnostic{
				Subject: block.Range(),
				Reason:  "terraform block contains multiple backend/cloud blocks",
			})
			continue
		}

		remote := remoteBlocks[0]
		chunkStart, chunkEnd, ok := backendChunkRange(parsed.source, parsed.tokens, block, remote)
		if !ok {
			diagnostics = append(diagnostics, transformDiagnostic{Subject: remote.Range(), Reason: "terraform body contains an ambiguous backend/cloud comment attachment"})
			continue
		}
		backendIndex, requiredIndex, foundRequired := backendOrderIndices(block.Body, remote)
		if !foundRequired {
			if backendIndex == 0 {
				continue
			}
			insertAt := firstBodyElementLineStart(parsed.source, block.Body)
			if insertAt < 0 {
				continue
			}
			edits = append(edits, sourceEdit{start: chunkStart, end: chunkEnd, insertAt: insertAt, text: parsed.source[chunkStart:chunkEnd]})
			changes = append(changes, transformChange{
				Subject: remote.Range(),
				Rule:    "backend_order",
				Reason:  "backend or cloud should be the first terraform body element when required_version is not present",
			})
			continue
		}
		if backendIndex == requiredIndex+1 {
			continue
		}
		_, requiredChunkEnd, ok := requiredVersionChunkRange(parsed.source, parsed.tokens, block, block.Body.Attributes["required_version"])
		if !ok {
			diagnostics = append(diagnostics, transformDiagnostic{Subject: block.Body.Attributes["required_version"].SrcRange, Reason: "terraform body contains an ambiguous required_version comment attachment"})
			continue
		}
		edits = append(edits, sourceEdit{start: chunkStart, end: chunkEnd, insertAt: requiredChunkEnd, text: parsed.source[chunkStart:chunkEnd]})
		changes = append(changes, transformChange{
			Subject: remote.Range(),
			Rule:    "backend_order",
			Reason:  "backend or cloud should immediately follow required_version in the terraform block",
		})
	}

	sort.Slice(edits, func(i, j int) bool {
		return edits[i].start > edits[j].start
	})
	for _, edit := range edits {
		candidate = applySourceMove(candidate, edit.start, edit.end, edit.insertAt, edit.text)
	}
	if len(diagnostics) == 0 {
		if _, parseDiagnostics := parseHCLSource("candidate.tf", candidate); parseDiagnostics.HasErrors() {
			return bytes.Clone(parsed.source), changes, []transformDiagnostic{{Subject: root.Range(), Reason: "candidate failed HCL reparse: " + parseDiagnostics.Error()}}
		}
	}
	return candidate, changes, diagnostics
}

func backendOrderIndices(body *hclsyntax.Body, backend *hclsyntax.Block) (int, int, bool) {
	elements := syntaxElements(body)
	backendIndex := -1
	requiredIndex := -1
	foundRequired := false
	for index, element := range elements {
		if element.attribute != nil && element.attribute.Name == "required_version" {
			requiredIndex = index
			foundRequired = true
		}
		if element.block != nil && element.block == backend {
			backendIndex = index
		}
	}
	return backendIndex, requiredIndex, foundRequired
}

func firstBodyElementLineStart(source []byte, body *hclsyntax.Body) int {
	elements := syntaxElements(body)
	for _, element := range elements {
		return lineStart(source, element.start)
	}
	return -1
}

func applySourceMove(source []byte, removeStart, removeEnd, insertAt int, text []byte) []byte {
	if removeStart < 0 || removeEnd < removeStart || removeEnd > len(source) {
		return bytes.Clone(source)
	}
	withoutRemoved := append([]byte{}, source[:removeStart]...)
	withoutRemoved = append(withoutRemoved, source[removeEnd:]...)
	if insertAt > removeStart {
		insertAt -= removeEnd - removeStart
	}
	if insertAt < 0 {
		insertAt = 0
	}
	if insertAt > len(withoutRemoved) {
		insertAt = len(withoutRemoved)
	}
	result := append([]byte{}, withoutRemoved[:insertAt]...)
	result = append(result, text...)
	result = append(result, withoutRemoved[insertAt:]...)
	return result
}

func backendChunkRange(source []byte, tokens hclsyntax.Tokens, block *hclsyntax.Block, backend *hclsyntax.Block) (int, int, bool) {
	start := lineStart(source, backend.Range().Start.Byte)
	end := lineEnd(source, backend.Range().End.Byte)
	cursor := start
	for index := len(tokens) - 1; index >= 0; index-- {
		token := tokens[index]
		if token.Type != hclsyntax.TokenComment || token.Range.End.Byte >= cursor {
			continue
		}
		if token.Range.Start.Byte < block.OpenBraceRange.End.Byte || token.Range.End.Byte > block.CloseBraceRange.Start.Byte {
			continue
		}
		if !commentGapWhitespace(source[token.Range.End.Byte:cursor], token.Range.End.Byte > token.Range.Start.Byte && source[token.Range.End.Byte-1] == '\n') {
			continue
		}
		tokenLineStart := lineStart(source, token.Range.Start.Byte)
		if !horizontalWhitespace(source[tokenLineStart:token.Range.Start.Byte]) {
			continue
		}
		start = tokenLineStart
		cursor = tokenLineStart
	}
	for _, token := range tokens {
		if token.Type != hclsyntax.TokenComment || token.Range.Start.Byte < backend.Range().End.Byte {
			continue
		}
		if token.Range.Start.Byte > block.CloseBraceRange.Start.Byte {
			continue
		}
		if lineStart(source, token.Range.Start.Byte) != lineStart(source, backend.Range().End.Byte) {
			continue
		}
		if !horizontalWhitespace(source[backend.Range().End.Byte:token.Range.Start.Byte]) {
			continue
		}
		end = token.Range.End.Byte
		if end > token.Range.Start.Byte && source[end-1] != '\n' && source[end-1] != '\r' {
			end = lineEnd(source, end)
		}
	}
	return start, end, start >= block.OpenBraceRange.End.Byte && end <= block.CloseBraceRange.Start.Byte
}

func requiredVersionChunkRange(source []byte, tokens hclsyntax.Tokens, block *hclsyntax.Block, attribute *hclsyntax.Attribute) (int, int, bool) {
	if attribute == nil {
		return 0, 0, false
	}
	start := lineStart(source, attribute.SrcRange.Start.Byte)
	end := lineEnd(source, attribute.SrcRange.End.Byte)
	cursor := start
	for index := len(tokens) - 1; index >= 0; index-- {
		token := tokens[index]
		if token.Type != hclsyntax.TokenComment || token.Range.End.Byte >= cursor {
			continue
		}
		if token.Range.Start.Byte < block.OpenBraceRange.End.Byte || token.Range.End.Byte > block.CloseBraceRange.Start.Byte {
			continue
		}
		if !commentGapWhitespace(source[token.Range.End.Byte:cursor], token.Range.End.Byte > token.Range.Start.Byte && source[token.Range.End.Byte-1] == '\n') {
			continue
		}
		tokenLineStart := lineStart(source, token.Range.Start.Byte)
		if !horizontalWhitespace(source[tokenLineStart:token.Range.Start.Byte]) {
			continue
		}
		start = tokenLineStart
		cursor = tokenLineStart
	}
	for _, token := range tokens {
		if token.Type != hclsyntax.TokenComment || token.Range.Start.Byte < attribute.SrcRange.End.Byte {
			continue
		}
		if token.Range.Start.Byte > block.CloseBraceRange.Start.Byte {
			continue
		}
		if lineStart(source, token.Range.Start.Byte) != lineStart(source, attribute.SrcRange.End.Byte) {
			continue
		}
		if !horizontalWhitespace(source[attribute.SrcRange.End.Byte:token.Range.Start.Byte]) {
			continue
		}
		end = token.Range.End.Byte
		if end > token.Range.Start.Byte && source[end-1] != '\n' && source[end-1] != '\r' {
			end = lineEnd(source, end)
		}
	}
	return start, end, true
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
		!sameCommentTokens(originalBody, validatedBody, original.tokens, validated.tokens, original.source, validated.source) {
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

func sameCommentTokens(leftBody, rightBody *hclsyntax.Body, leftTokens, rightTokens hclsyntax.Tokens, leftSource, rightSource []byte) bool {
	leftTerraformBodies := terraformBodies(leftBody)
	rightTerraformBodies := terraformBodies(rightBody)
	if len(leftTerraformBodies) != len(rightTerraformBodies) {
		return false
	}
	for index, leftTerraformBody := range leftTerraformBodies {
		if !sameBodyCommentOwnership(leftTerraformBody, rightTerraformBodies[index], leftTokens, rightTokens, leftSource, rightSource) {
			return false
		}
	}
	return true
}

func terraformBodies(body *hclsyntax.Body) []*hclsyntax.Body {
	bodies := make([]*hclsyntax.Body, 0, len(body.Blocks))
	for _, block := range body.Blocks {
		if block.Type == "terraform" {
			bodies = append(bodies, block.Body)
		}
	}
	return bodies
}

func sameBodyCommentOwnership(leftBody, rightBody *hclsyntax.Body, leftTokens, rightTokens hclsyntax.Tokens, leftSource, rightSource []byte) bool {
	leftOwnership := elementCommentOwnership(leftBody, leftTokens, leftSource)
	rightOwnership := elementCommentOwnership(rightBody, rightTokens, rightSource)

	leftRequired := make([]string, 0, 2)
	rightRequired := make([]string, 0, 2)
	leftFiltered := make([]elementCommentOwnershipEntry, 0, len(leftOwnership))
	rightFiltered := make([]elementCommentOwnershipEntry, 0, len(rightOwnership))
	for _, element := range leftOwnership {
		if element.kind == "attribute" && element.name == "required_version" {
			leftRequired = append(leftRequired, element.comments...)
			continue
		}
		leftFiltered = append(leftFiltered, element)
	}
	for _, element := range rightOwnership {
		if element.kind == "attribute" && element.name == "required_version" {
			rightRequired = append(rightRequired, element.comments...)
			continue
		}
		rightFiltered = append(rightFiltered, element)
	}
	if !sameStringSlices(leftRequired, rightRequired) || len(leftFiltered) != len(rightFiltered) {
		return false
	}
	for index, leftElement := range leftFiltered {
		rightElement := rightFiltered[index]
		if leftElement.kind != rightElement.kind || leftElement.name != rightElement.name || !sameLabels(leftElement.labels, rightElement.labels) || !sameStringSlices(leftElement.comments, rightElement.comments) {
			return false
		}
	}
	return true
}

type elementCommentOwnershipEntry struct {
	kind     string
	name     string
	labels   []string
	comments []string
}

func elementCommentOwnership(body *hclsyntax.Body, tokens hclsyntax.Tokens, source []byte) []elementCommentOwnershipEntry {
	elements := syntaxElements(body)
	ownership := make([]elementCommentOwnershipEntry, 0, len(elements))
	for _, element := range elements {
		comments := make([]string, 0, 2)
		comments = append(comments, leadingCommentsForElement(source, tokens, element.rangeValue.Start.Byte)...)
		comments = append(comments, trailingCommentsForElement(source, tokens, element.rangeValue)...)
		ownership = append(ownership, elementCommentOwnershipEntry{
			kind:     element.kind,
			name:     element.name,
			labels:   append([]string(nil), element.labels...),
			comments: comments,
		})
	}
	return ownership
}

func leadingCommentsForElement(source []byte, tokens hclsyntax.Tokens, elementStart int) []string {
	comments := make([]string, 0, 2)
	cursor := elementStart
	for index := len(tokens) - 1; index >= 0; index-- {
		token := tokens[index]
		if token.Type != hclsyntax.TokenComment || token.Range.End.Byte >= cursor {
			continue
		}
		if !commentGapWhitespace(source[token.Range.End.Byte:cursor], token.Range.End.Byte > token.Range.Start.Byte && source[token.Range.End.Byte-1] == '\n') {
			continue
		}
		tokenLineStart := lineStart(source, token.Range.Start.Byte)
		if !horizontalWhitespace(source[tokenLineStart:token.Range.Start.Byte]) {
			continue
		}
		comments = append([]string{string(source[token.Range.Start.Byte:token.Range.End.Byte])}, comments...)
		cursor = tokenLineStart
	}
	return comments
}

func trailingCommentsForElement(source []byte, tokens hclsyntax.Tokens, elementRange hcl.Range) []string {
	comments := make([]string, 0, 1)
	for _, token := range tokens {
		if token.Type != hclsyntax.TokenComment || token.Range.Start.Byte < elementRange.End.Byte {
			continue
		}
		if lineStart(source, token.Range.Start.Byte) != lineStart(source, elementRange.End.Byte) {
			continue
		}
		if !horizontalWhitespace(source[elementRange.End.Byte:token.Range.Start.Byte]) {
			continue
		}
		comments = append(comments, string(source[token.Range.Start.Byte:token.Range.End.Byte]))
	}
	return comments
}

func sameStringSlices(left, right []string) bool {
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

	type elementRange struct {
		start  int
		end    int
		target bool
	}
	elements := make([]elementRange, 0, len(block.Body.Attributes)+len(block.Body.Blocks))
	for _, item := range block.Body.Attributes {
		elements = append(elements, elementRange{
			start:  item.SrcRange.Start.Byte,
			end:    item.SrcRange.End.Byte,
			target: item == attribute,
		})
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
	if attributeIndex == 0 {
		return nil, nil
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
				commentEnd := token.Range.End.Byte
				if commentEnd > token.Range.Start.Byte && parsed.source[commentEnd-1] != '\n' && parsed.source[commentEnd-1] != '\r' {
					commentEnd = lineEnd(parsed.source, commentEnd)
				}
				attributeLineEnd = commentEnd
				continue
			}
			containedByNonTargetElement := false
			for _, element := range elements {
				if !element.target && token.Range.Start.Byte >= element.start && token.Range.End.Byte <= element.end {
					containedByNonTargetElement = true
					break
				}
			}
			if containedByNonTargetElement {
				continue
			}
			return nil, &transformDiagnostic{
				Subject: token.Range,
				Reason:  "terraform body contains a comment whose ownership is not yet classified",
			}
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

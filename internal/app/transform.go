package app

import (
	"bytes"
	"sort"
	"strconv"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
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

type sourceChunk struct {
	start int
	end   int
	text  []byte
}

type sourceBodyPolicy struct {
	owner             string
	rank              func(syntaxElement) (int, bool)
	children          map[string]sourceBodyPolicy
	preserveAttribute func(*parsedFile, *parsedFile, syntaxElement, syntaxElement) bool
}

func validateSourceCandidate(original *parsedFile, candidate []byte, policies map[string]sourceBodyPolicy) *transformDiagnostic {
	originalRoot, ok := original.body.(*hclsyntax.Body)
	if !ok {
		return &transformDiagnostic{Reason: "unsupported original body syntax"}
	}
	failure := &transformDiagnostic{Subject: originalRoot.Range(), Reason: "candidate changed source chunks, comment ownership, or non-target bytes"}
	if len(original.source) != len(candidate) {
		return failure
	}
	validated, diagnostics := parseHCLSource(originalRoot.Range().Filename, candidate)
	if diagnostics.HasErrors() {
		return failure
	}
	candidateRoot, ok := validated.body.(*hclsyntax.Body)
	if !ok {
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
		if !sameSyntaxElement(element, other) || element.start != other.start || element.rangeValue.End.Byte != other.rangeValue.End.Byte ||
			!bytes.Equal(original.source[cursor:element.start], candidate[cursor:other.start]) {
			return failure
		}
		if element.block != nil {
			if sameRangeBytes(original.source, element.rangeValue, candidate, other.rangeValue) {
				cursor = element.rangeValue.End.Byte
				continue
			}
			if policy, targeted := policies[element.block.Type]; targeted {
				if other.block == nil {
					return failure
				}
				expected, reason := expectedSourceBlock(original, validated, element.block, other.block, policy)
				actual := candidate[other.rangeValue.Start.Byte:other.rangeValue.End.Byte]
				if reason == "" && !bytes.Equal(expected, actual) {
					reason = sourceByteMismatch(expected, actual)
				}
				if reason != "" {
					failure.Reason = "candidate changed " + element.block.Type + " source chunks: " + reason
					return failure
				}
			} else if !sameRangeBytes(original.source, element.rangeValue, candidate, other.rangeValue) {
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

func sourceByteMismatch(expected, actual []byte) string {
	limit := min(len(expected), len(actual))
	for index := 0; index < limit; index++ {
		if expected[index] != actual[index] {
			return "candidate bytes differ from input chunks at byte " + strconv.Itoa(index) + " (expected " + strconv.Itoa(int(expected[index])) + ", actual " + strconv.Itoa(int(actual[index])) + ")"
		}
	}
	return "candidate byte length differs from input chunks"
}

func sameSyntaxElement(left, right syntaxElement) bool {
	return left.kind == right.kind && left.name == right.name && sameLabels(left.labels, right.labels)
}

func expectedSourceBlock(original, candidate *parsedFile, originalBlock, candidateBlock *hclsyntax.Block, policy sourceBodyPolicy) ([]byte, string) {
	if originalBlock.Type != candidateBlock.Type || !sameLabels(originalBlock.Labels, candidateBlock.Labels) ||
		originalBlock.Range().End.Byte-originalBlock.Range().Start.Byte != candidateBlock.Range().End.Byte-candidateBlock.Range().Start.Byte {
		return nil, "block identity or range differs"
	}
	originalElements := syntaxElements(originalBlock.Body)
	candidateElements := syntaxElements(candidateBlock.Body)
	if len(originalElements) != len(candidateElements) {
		return nil, "body element count differs"
	}
	originalChunks, diagnostic := sourceChunksForPolicy(original, originalBlock, originalElements, policy.owner)
	if diagnostic != nil {
		return nil, diagnostic.Reason
	}
	if len(originalChunks) != len(candidateElements) {
		return nil, "body chunk count differs"
	}
	var candidateChunks []sourceChunk
	if policy.preserveAttribute != nil {
		candidateChunks, diagnostic = sourceChunksForPolicy(candidate, candidateBlock, candidateElements, policy.owner)
		if diagnostic != nil {
			return nil, diagnostic.Reason
		}
		if len(candidateChunks) != len(candidateElements) {
			return nil, "candidate body chunk count differs"
		}
	}
	order, _, unsupportedIndex := rankedElementOrder(originalElements, policy.rank)
	if unsupportedIndex != -1 {
		return nil, "body contains an unsupported element"
	}
	var body []byte
	if len(originalChunks) == 0 {
		body = bytes.Clone(original.source[originalBlock.OpenBraceRange.End.Byte:originalBlock.CloseBraceRange.Start.Byte])
	} else {
		body = bytes.Clone(original.source[originalBlock.OpenBraceRange.End.Byte:originalChunks[0].start])
	}
	for candidateIndex, originalIndex := range order {
		originalElement := originalElements[originalIndex]
		candidateElement := candidateElements[candidateIndex]
		originalChunk := originalChunks[originalIndex]
		if !sameSyntaxElement(originalElement, candidateElement) {
			return nil, "candidate element order differs from the allowed rank order"
		}
		expectedChunk := bytes.Clone(originalChunk.text)
		if originalElement.attribute != nil && candidateElement.attribute != nil && !sameRangeBytes(original.source, originalElement.rangeValue, candidate.source, candidateElement.rangeValue) {
			if policy.preserveAttribute == nil || !policy.preserveAttribute(original, candidate, originalElement, candidateElement) {
				return nil, "attribute bytes are not permitted by the body policy"
			}
			expectedChunk = bytes.Clone(candidateChunks[candidateIndex].text)
		} else if originalElement.block != nil && candidateElement.block != nil && !sameRangeBytes(original.source, originalElement.rangeValue, candidate.source, candidateElement.rangeValue) {
			childPolicy, allowed := policy.children[originalElement.block.Type]
			if !allowed {
				return nil, "nested " + originalElement.block.Type + " block is not editable by policy"
			}
			nested, reason := expectedSourceBlock(original, candidate, originalElement.block, candidateElement.block, childPolicy)
			if reason != "" {
				return nil, "nested " + originalElement.block.Type + ": " + reason
			}
			start := originalElement.rangeValue.Start.Byte - originalChunk.start
			end := originalElement.rangeValue.End.Byte - originalChunk.start
			if start < 0 || end < start || end > len(expectedChunk) {
				return nil, "nested block is outside its source chunk"
			}
			expectedChunk = append(bytes.Clone(expectedChunk[:start]), append(nested, expectedChunk[end:]...)...)
		}
		body = append(body, expectedChunk...)
	}
	if len(originalChunks) > 0 {
		body = append(body, original.source[originalChunks[len(originalChunks)-1].end:originalBlock.CloseBraceRange.Start.Byte]...)
	}
	expected := append(bytes.Clone(original.source[originalBlock.Range().Start.Byte:originalBlock.OpenBraceRange.End.Byte]), body...)
	expected = append(expected, original.source[originalBlock.CloseBraceRange.Start.Byte:originalBlock.Range().End.Byte]...)
	return expected, ""
}

func sourceChunksForPolicy(parsed *parsedFile, block *hclsyntax.Block, elements []syntaxElement, owner string) ([]sourceChunk, *transformDiagnostic) {
	switch owner {
	case "terraform":
		return requiredVersionBodyChunks(parsed, block, elements)
	case "terraform_backend":
		return terraformBackendBodyChunks(parsed, block, elements)
	case "terraform_required_version":
		return requiredVersionBodyChunks(parsed, block, elements)
	case "cloud", "workspaces":
		return cloudBodyChunks(parsed, block, elements)
	case "provider":
		return providerBodyChunks(parsed, block, elements)
	case "output":
		return outputBodyChunks(parsed, block, elements)
	case "variable":
		return variableBodyChunks(parsed, block, elements)
	default:
		return bodyElementChunks(parsed, block, elements, owner)
	}
}

func terraformBackendBodyChunks(parsed *parsedFile, block *hclsyntax.Block, elements []syntaxElement) ([]sourceChunk, *transformDiagnostic) {
	starts := make([]int, len(elements))
	ends := make([]int, len(elements))
	for index, element := range elements {
		starts[index] = lineStart(parsed.source, element.rangeValue.Start.Byte)
		ends[index] = lineEnd(parsed.source, cloudElementEnd(element))
		if element.attribute != nil && element.attribute.Name == "required_version" {
			start, end, ok := requiredVersionChunkRange(parsed.source, parsed.tokens, block, element.attribute)
			if !ok {
				return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "terraform required_version comment boundary is ambiguous"}
			}
			starts[index], ends[index] = start, end
		}
		if element.block != nil && (element.block.Type == "backend" || element.block.Type == "cloud") {
			start, end, ok := backendChunkRange(parsed.source, parsed.tokens, block, element.block)
			if !ok {
				return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "terraform backend/cloud comment boundary is ambiguous"}
			}
			starts[index], ends[index] = start, end
		}
	}
	for index, element := range elements {
		end := ends[index]
		if index+1 < len(elements) {
			if end > starts[index+1] {
				return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "terraform element comment chunks overlap"}
			}
			end = starts[index+1]
		}
		if starts[index] < block.OpenBraceRange.End.Byte || end > block.CloseBraceRange.Start.Byte || end < cloudElementEnd(element) {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "terraform element comment boundaries are ambiguous"}
		}
	}
	return sourceChunksFromBounds(parsed.source, starts, ends), nil
}

func requiredVersionBodyChunks(parsed *parsedFile, block *hclsyntax.Block, elements []syntaxElement) ([]sourceChunk, *transformDiagnostic) {
	starts := make([]int, len(elements))
	ends := make([]int, len(elements))
	for index, element := range elements {
		starts[index] = lineStart(parsed.source, element.rangeValue.Start.Byte)
		ends[index] = lineEnd(parsed.source, cloudElementEnd(element))
		if element.attribute != nil && element.attribute.Name == "required_version" {
			start, end, ok := requiredVersionChunkRange(parsed.source, parsed.tokens, block, element.attribute)
			if !ok {
				return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "terraform required_version comment boundary is ambiguous"}
			}
			starts[index], ends[index] = start, end
		}
	}
	for index, element := range elements {
		end := ends[index]
		if index+1 < len(elements) {
			if end > starts[index+1] {
				return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "terraform element comment chunks overlap"}
			}
			end = starts[index+1]
		}
		if starts[index] < block.OpenBraceRange.End.Byte || end > block.CloseBraceRange.Start.Byte || end < cloudElementEnd(element) {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "terraform element comment boundaries are ambiguous"}
		}
	}
	return sourceChunksFromBounds(parsed.source, starts, ends), nil
}

func sourceChunksFromBounds(source []byte, starts, ends []int) []sourceChunk {
	chunks := make([]sourceChunk, len(starts))
	for index, start := range starts {
		end := ends[index]
		if index+1 < len(starts) {
			end = starts[index+1]
		}
		chunks[index] = sourceChunk{start: start, end: end, text: bytes.Clone(source[start:end])}
	}
	return chunks
}

type sourceCommentPolicy struct {
	owner                  string
	elementStart           func(syntaxElement) int
	elementEnd             func(syntaxElement) int
	trailingCommentLineEnd func([]byte, int) int
	reverseTokens          bool
	preserveEarliestStart  bool
}

func assignSourceComments(parsed *parsedFile, block *hclsyntax.Block, elements []syntaxElement, starts, ends []int, policy sourceCommentPolicy) *transformDiagnostic {
	assign := func(token hclsyntax.Token) bool {
		if token.Type != hclsyntax.TokenComment || token.Range.Start.Byte < block.OpenBraceRange.End.Byte || token.Range.End.Byte > block.CloseBraceRange.Start.Byte {
			return true
		}

		owned := false
		for _, element := range elements {
			if token.Range.Start.Byte >= policy.elementStart(element) && token.Range.End.Byte <= policy.elementEnd(element) {
				owned = true
				break
			}
		}
		if owned {
			return true
		}

		for index, element := range elements {
			end := policy.elementEnd(element)
			if lineStart(parsed.source, token.Range.Start.Byte) == lineStart(parsed.source, end) && token.Range.Start.Byte >= end && horizontalWhitespace(parsed.source[end:token.Range.Start.Byte]) {
				ends[index] = policy.trailingCommentLineEnd(parsed.source, token.Range.End.Byte)
				return true
			}
		}

		for index, element := range elements {
			if policy.elementStart(element) <= token.Range.End.Byte {
				continue
			}
			commentStart := lineStart(parsed.source, token.Range.Start.Byte)
			endsWithNewline := token.Range.End.Byte > token.Range.Start.Byte && parsed.source[token.Range.End.Byte-1] == '\n'
			if token.Range.End.Byte <= starts[index] && horizontalWhitespace(parsed.source[commentStart:token.Range.Start.Byte]) && commentGapWhitespace(parsed.source[token.Range.End.Byte:starts[index]], endsWithNewline) {
				if !policy.preserveEarliestStart || commentStart < starts[index] {
					starts[index] = commentStart
				}
				return true
			}
			break
		}
		return false
	}

	if policy.reverseTokens {
		for index := len(parsed.tokens) - 1; index >= 0; index-- {
			if !assign(parsed.tokens[index]) {
				return &transformDiagnostic{Subject: parsed.tokens[index].Range, Reason: policy.owner + " body contains an ambiguous comment attachment"}
			}
		}
		return nil
	}
	for _, token := range parsed.tokens {
		if !assign(token) {
			return &transformDiagnostic{Subject: token.Range, Reason: policy.owner + " body contains an ambiguous comment attachment"}
		}
	}
	return nil
}

func applySourceEdits(source []byte, edits []sourceEdit) []byte {
	ordered := append([]sourceEdit(nil), edits...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].start > ordered[right].start })
	previousStart := len(source) + 1
	for index, edit := range ordered {
		if edit.insertAt != 0 || edit.start < 0 || edit.end < edit.start || edit.end > len(source) ||
			(index > 0 && (edit.start == previousStart || edit.end > previousStart)) {
			return bytes.Clone(source)
		}
		previousStart = edit.start
	}

	candidate := bytes.Clone(source)
	for _, edit := range ordered {
		candidate = append(bytes.Clone(candidate[:edit.start]), append(bytes.Clone(edit.text), candidate[edit.end:]...)...)
	}
	return candidate
}

func applySourceMoves(source []byte, edits []sourceEdit) []byte {
	ordered := append([]sourceEdit(nil), edits...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].start > ordered[right].start })
	for index, edit := range ordered {
		if edit.start < 0 || edit.end <= edit.start || edit.end > len(source) || edit.insertAt < 0 || edit.insertAt > len(source) ||
			(edit.insertAt > edit.start && edit.insertAt < edit.end) || len(edit.text) != edit.end-edit.start ||
			(edit.insertAt != edit.start && !bytes.Equal(edit.text, source[edit.start:edit.end])) ||
			(index > 0 && edit.end > ordered[index-1].start) {
			return bytes.Clone(source)
		}
		for otherIndex, other := range ordered {
			if index == otherIndex || edit.insertAt == edit.start {
				continue
			}
			moveStart := min(edit.start, edit.insertAt)
			moveEnd := max(edit.end, edit.insertAt)
			if edit.insertAt == other.insertAt || (moveStart < other.end && other.start < moveEnd) {
				return bytes.Clone(source)
			}
		}
	}

	candidate := bytes.Clone(source)
	for _, edit := range ordered {
		candidate = applySourceMove(candidate, edit.start, edit.end, edit.insertAt, edit.text)
	}
	return candidate
}

func transformRequiredVersionOrder(parsed *parsedFile, config Config) ([]byte, []transformDiagnostic) {
	candidate, _, diagnostics := transformRequiredVersionOrderWithChanges(parsed, config)
	return candidate, diagnostics
}

func transformTerraformOrderWithChanges(parsed *parsedFile, config Config) ([]byte, []transformChange, []transformDiagnostic) {
	candidate, changes, diagnostics := transformTerraformOrderOnlyWithChanges(parsed, config)
	if config.ReplaceRequiredVersion == "" {
		return candidate, changes, diagnostics
	}
	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return candidate, changes, diagnostics
	}
	ordered, parseDiagnostics := parseHCLSource(root.Range().Filename, candidate)
	if parseDiagnostics.HasErrors() {
		return bytes.Clone(parsed.source), nil, append(diagnostics, transformDiagnostic{Subject: root.Range(), Reason: "replacement input failed HCL reparse"})
	}
	replaced, replacementChanges, replacementDiagnostics := transformRequiredVersionReplacement(ordered, config.ReplaceRequiredVersion)
	orderedRoot := ordered.body.(*hclsyntax.Body)
	positions := make(map[int]hcl.Range)
	for index, block := range orderedRoot.Blocks {
		if block.Type != "terraform" {
			continue
		}
		attribute := block.Body.Attributes["required_version"]
		originalAttribute := root.Blocks[index].Body.Attributes["required_version"]
		if attribute != nil && originalAttribute != nil {
			positions[attribute.Expr.Range().Start.Byte] = originalAttribute.Expr.Range()
		}
	}
	for index := range replacementChanges {
		replacementChanges[index].Subject = positions[replacementChanges[index].Subject.Start.Byte]
	}
	for index := range replacementDiagnostics {
		if position, ok := positions[replacementDiagnostics[index].Subject.Start.Byte]; ok {
			replacementDiagnostics[index].Subject = position
		}
	}
	return replaced, append(changes, replacementChanges...), append(diagnostics, replacementDiagnostics...)
}

func transformTerraformOrderOnlyWithChanges(parsed *parsedFile, config Config) ([]byte, []transformChange, []transformDiagnostic) {
	return runTransformPipelineWithFinalValidation(parsed, []transformPipelineStep{
		{enabled: true, transform: func(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
			return transformRequiredVersionOrderWithChanges(parsed, config)
		}},
		{enabled: config.BackendOrder, transform: func(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
			return transformBackendOrderWithChanges(parsed, config)
		}},
		{enabled: config.CloudOrder, transform: func(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
			return transformCloudOrderWithChanges(parsed)
		}},
		{enabled: config.CloudWorkspacesOrder, transform: func(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
			return transformCloudWorkspacesOrderWithChanges(parsed)
		}},
		{enabled: config.RequiredProvidersOrder, transform: transformRequiredProvidersOrderWithChanges},
		{enabled: config.ProviderOrder, transform: transformProviderOrderWithChanges},
		{enabled: config.VariableOrder, transform: transformVariableOrderWithChanges},
		{enabled: config.OutputOrder, transform: transformOutputOrderWithChanges},
		{enabled: config.DataOrder, transform: transformDataOrderWithChanges},
		{enabled: config.ActionOrder, transform: transformActionOrderWithChanges},
		{enabled: config.CheckOrder, transform: transformCheckOrderWithChanges},
		{enabled: config.EphemeralOrder, transform: transformEphemeralOrderWithChanges},
		{enabled: config.ResourceOrder, transform: transformResourceOrderWithChanges},
		{enabled: config.ModuleOrder, transform: transformModuleOrderWithChanges},
		{enabled: config.ImportOrder, transform: transformImportOrderWithChanges},
		{enabled: config.MovedOrder, transform: transformMovedOrderWithChanges},
		{enabled: config.RemovedOrder, transform: transformRemovedOrderWithChanges},
	}, validateProviderMetaCandidate)
}

func transformRequiredProvidersOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
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
		if parsed.ruleIgnored("required_providers_order", block) {
			continue
		}
		for _, requiredProviders := range block.Body.Blocks {
			if requiredProviders.Type != "required_providers" {
				continue
			}
			for _, element := range syntaxElements(requiredProviders.Body) {
				provider := element.attribute
				if provider == nil {
					continue
				}
				providerRequirement, ok := provider.Expr.(*hclsyntax.ObjectConsExpr)
				if !ok {
					if isLegacyProviderRequirement(provider.Expr) {
						continue
					}
					diagnostics = append(diagnostics, transformDiagnostic{Subject: provider.Expr.Range(), Reason: "provider requirement is neither an object nor a legacy string"})
					continue
				}
				edit, change, diagnostic := requiredProviderObjectEdit(parsed, providerRequirement)
				if diagnostic != nil {
					diagnostics = append(diagnostics, *diagnostic)
					continue
				}
				if edit != nil {
					edits = append(edits, *edit)
					changes = append(changes, *change)
				}
			}
		}
	}

	candidate = applySourceMoves(candidate, edits)
	if len(edits) > 0 {
		if diagnostic := validateRequiredProvidersCandidate(parsed, candidate); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

type requiredProviderObjectChunk struct {
	key   string
	rank  int
	start int
	end   int
	text  []byte
}

func requiredProviderObjectEdit(parsed *parsedFile, object *hclsyntax.ObjectConsExpr) (*sourceEdit, *transformChange, *transformDiagnostic) {
	objectRange := object.Range()
	if lineStart(parsed.source, objectRange.Start.Byte) == lineStart(parsed.source, objectRange.End.Byte) {
		return nil, nil, &transformDiagnostic{Subject: objectRange, Reason: "single-line provider requirement object is not sorted"}
	}

	ranks := map[string]int{"source": 0, "version": 1, "configuration_aliases": 2}
	chunks := make([]requiredProviderObjectChunk, 0, len(object.Items))
	seen := make(map[string]struct{}, len(object.Items))
	for _, item := range object.Items {
		key, ok := staticObjectKey(item.KeyExpr)
		if !ok {
			return nil, nil, &transformDiagnostic{Subject: item.KeyExpr.Range(), Reason: "provider requirement contains a dynamic key"}
		}
		rank, known := ranks[key]
		if !known {
			return nil, nil, &transformDiagnostic{Subject: item.KeyExpr.Range(), Reason: "provider requirement contains an unknown key"}
		}
		if _, exists := seen[key]; exists {
			return nil, nil, &transformDiagnostic{Subject: item.KeyExpr.Range(), Reason: "provider requirement contains a duplicate key"}
		}
		seen[key] = struct{}{}

		start, end, ok := requiredProviderObjectChunkRange(parsed, objectRange, item)
		if !ok {
			return nil, nil, &transformDiagnostic{Subject: item.KeyExpr.Range(), Reason: "provider requirement item boundaries are ambiguous"}
		}
		chunks = append(chunks, requiredProviderObjectChunk{
			key:   key,
			rank:  rank,
			start: start,
			end:   end,
			text:  parsed.source[start:end],
		})
	}
	for _, token := range parsed.tokens {
		if token.Type != hclsyntax.TokenComment || token.Range.Start.Byte < objectRange.Start.Byte || token.Range.End.Byte > objectRange.End.Byte {
			continue
		}
		owned := false
		for _, chunk := range chunks {
			if token.Range.Start.Byte >= chunk.start && token.Range.End.Byte <= chunk.end {
				owned = true
				break
			}
		}
		if !owned {
			return nil, nil, &transformDiagnostic{Subject: token.Range, Reason: "provider requirement contains a comment with ambiguous item ownership"}
		}
	}
	if len(chunks) < 2 {
		return nil, nil, nil
	}
	for index := 1; index < len(chunks); index++ {
		if chunks[index-1].end != chunks[index].start {
			return nil, nil, &transformDiagnostic{Subject: objectRange, Reason: "provider requirement item boundaries are ambiguous"}
		}
	}
	if chunks[len(chunks)-1].end != lineStart(parsed.source, objectRange.End.Byte) {
		return nil, nil, &transformDiagnostic{Subject: objectRange, Reason: "provider requirement trailing boundary is ambiguous"}
	}

	ordered := append([]requiredProviderObjectChunk(nil), chunks...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].rank < ordered[j].rank
	})
	var replacement []byte
	for _, chunk := range ordered {
		replacement = append(replacement, chunk.text...)
	}
	if bytes.Equal(replacement, parsed.source[chunks[0].start:chunks[len(chunks)-1].end]) {
		return nil, nil, nil
	}
	edit := &sourceEdit{
		start:    chunks[0].start,
		end:      chunks[len(chunks)-1].end,
		insertAt: chunks[0].start,
		text:     replacement,
	}
	change := &transformChange{
		Subject: objectRange,
		Rule:    "required_providers_order",
		Reason:  "provider requirement keys should be ordered source, version, configuration_aliases",
	}
	return edit, change, nil
}

func staticObjectKey(expression hclsyntax.Expression) (string, bool) {
	if traversal, ok := expression.(*hclsyntax.ScopeTraversalExpr); ok {
		if len(traversal.Traversal) == 1 {
			if root, ok := traversal.Traversal[0].(hcl.TraverseRoot); ok {
				return root.Name, true
			}
		}
		return "", false
	}
	value, diagnostics := expression.Value(nil)
	if diagnostics.HasErrors() || !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.String) {
		return "", false
	}
	return value.AsString(), true
}

func isLegacyProviderRequirement(expression hclsyntax.Expression) bool {
	value, diagnostics := expression.Value(nil)
	return !diagnostics.HasErrors() && value.IsKnown() && !value.IsNull() && value.Type().Equals(cty.String)
}

func requiredProviderObjectChunkRange(parsed *parsedFile, objectRange hcl.Range, item hclsyntax.ObjectConsItem) (int, int, bool) {
	keyStart := item.KeyExpr.Range().Start.Byte
	keyLineStart := lineStart(parsed.source, keyStart)
	if keyLineStart <= lineStart(parsed.source, objectRange.Start.Byte) || !horizontalWhitespace(parsed.source[keyLineStart:keyStart]) {
		return 0, 0, false
	}
	start := keyLineStart
	end := lineEnd(parsed.source, item.ValueExpr.Range().End.Byte)
	if start < objectRange.Start.Byte || end > objectRange.End.Byte || end < start {
		return 0, 0, false
	}
	for _, token := range parsed.tokens {
		if token.Type == hclsyntax.TokenComment && token.Range.Start.Byte >= item.ValueExpr.Range().End.Byte && token.Range.Start.Byte < end && token.Range.End.Byte > end {
			end = lineEnd(parsed.source, token.Range.End.Byte)
		}
	}

	cursor := start
	for index := len(parsed.tokens) - 1; index >= 0; index-- {
		token := parsed.tokens[index]
		if token.Type != hclsyntax.TokenComment || token.Range.End.Byte > cursor {
			continue
		}
		if token.Range.Start.Byte < objectRange.Start.Byte || token.Range.End.Byte > objectRange.End.Byte {
			continue
		}
		endsWithNewline := token.Range.End.Byte > token.Range.Start.Byte && parsed.source[token.Range.End.Byte-1] == '\n'
		if !commentGapWhitespace(parsed.source[token.Range.End.Byte:cursor], endsWithNewline) {
			continue
		}
		tokenLineStart := lineStart(parsed.source, token.Range.Start.Byte)
		if !horizontalWhitespace(parsed.source[tokenLineStart:token.Range.Start.Byte]) {
			continue
		}
		start = tokenLineStart
		cursor = tokenLineStart
	}
	return start, end, start >= objectRange.Start.Byte && end <= objectRange.End.Byte
}

func validateRequiredProvidersCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	stableOrder := func(syntaxElement) (int, bool) { return 0, true }
	return validateSourceCandidate(original, candidate, map[string]sourceBodyPolicy{
		"terraform": {
			owner: "terraform", rank: stableOrder,
			children: map[string]sourceBodyPolicy{
				"required_providers": {
					owner: "required_providers", rank: stableOrder,
					preserveAttribute: preserveRequiredProviderAttribute,
				},
			},
		},
	})
}

func preserveRequiredProviderAttribute(original, candidate *parsedFile, originalElement, candidateElement syntaxElement) bool {
	originalObject, originalIsObject := originalElement.attribute.Expr.(*hclsyntax.ObjectConsExpr)
	candidateObject, candidateIsObject := candidateElement.attribute.Expr.(*hclsyntax.ObjectConsExpr)
	if !originalIsObject || !candidateIsObject {
		return sameRangeBytes(original.source, originalElement.rangeValue, candidate.source, candidateElement.rangeValue)
	}
	return sameProviderRequirementObject(originalObject, candidateObject, original, candidate)
}

func sameProviderRequirementObject(original, candidate *hclsyntax.ObjectConsExpr, originalFile, candidateFile *parsedFile) bool {
	if sameRangeBytes(originalFile.source, original.Range(), candidateFile.source, candidate.Range()) {
		return true
	}
	if len(original.Items) != len(candidate.Items) {
		return false
	}
	ranks := map[string]int{"source": 0, "version": 1, "configuration_aliases": 2}
	originalItems := make(map[string][]byte, len(original.Items))
	for _, item := range original.Items {
		key, ok := staticObjectKey(item.KeyExpr)
		if !ok {
			return false
		}
		if _, exists := ranks[key]; !exists {
			return false
		}
		if _, exists := originalItems[key]; exists {
			return false
		}
		start, end, ok := requiredProviderObjectChunkRange(originalFile, original.Range(), item)
		if !ok {
			return false
		}
		originalItems[key] = originalFile.source[start:end]
	}
	previousRank := -1
	for _, item := range candidate.Items {
		key, ok := staticObjectKey(item.KeyExpr)
		if !ok {
			return false
		}
		rank, known := ranks[key]
		if !known || rank < previousRank {
			return false
		}
		previousRank = rank
		originalChunk, exists := originalItems[key]
		start, end, ok := requiredProviderObjectChunkRange(candidateFile, candidate.Range(), item)
		if !exists || !ok || !bytes.Equal(originalChunk, candidateFile.source[start:end]) {
			return false
		}
		delete(originalItems, key)
	}
	return len(originalItems) == 0
}

func transformBackendOrder(parsed *parsedFile, config Config) ([]byte, []transformDiagnostic) {
	candidate, _, diagnostics := transformBackendOrderWithChanges(parsed, config)
	return candidate, diagnostics
}

func transformBackendOrderWithChanges(parsed *parsedFile, config Config) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	if !config.BackendOrder || parsed.ruleIgnored("backend_order", nil) {
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
		if parsed.ruleIgnored("backend_order", block) {
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
		backendIndex, requiredIndex, foundRequired := backendOrderIndices(block.Body, remote)
		if (!foundRequired && backendIndex == 0) || (foundRequired && backendIndex == requiredIndex+1) {
			continue
		}
		chunks, diagnostic := terraformBackendBodyChunks(parsed, block, syntaxElements(block.Body))
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
			continue
		}
		chunkStart, chunkEnd := chunks[backendIndex].start, chunks[backendIndex].end
		if !foundRequired {
			insertAt := chunks[0].start
			edits = append(edits, sourceEdit{start: chunkStart, end: chunkEnd, insertAt: insertAt, text: parsed.source[chunkStart:chunkEnd]})
			changes = append(changes, transformChange{
				Subject: remote.Range(),
				Rule:    "backend_order",
				Reason:  "backend or cloud should be the first terraform body element when required_version is not present",
			})
			continue
		}
		requiredChunkEnd := chunks[requiredIndex].end
		edits = append(edits, sourceEdit{start: chunkStart, end: chunkEnd, insertAt: requiredChunkEnd, text: parsed.source[chunkStart:chunkEnd]})
		changes = append(changes, transformChange{
			Subject: remote.Range(),
			Rule:    "backend_order",
			Reason:  "backend or cloud should immediately follow required_version in the terraform block",
		})
	}

	candidate = applySourceMoves(candidate, edits)
	if len(edits) > 0 && len(diagnostics) == 0 {
		if diagnostic := validateSourceCandidate(parsed, candidate, map[string]sourceBodyPolicy{
			"terraform": {owner: "terraform_backend", rank: terraformBackendElementRank},
		}); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
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

func transformCloudOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	if parsed.ruleIgnored("cloud_order", nil) {
		return candidate, nil, nil
	}
	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return candidate, nil, []transformDiagnostic{{Reason: "unsupported root body syntax"}}
	}

	var edits []sourceEdit
	var changes []transformChange
	var diagnostics []transformDiagnostic
	for _, terraformBlock := range root.Blocks {
		if terraformBlock.Type != "terraform" {
			continue
		}
		if parsed.ruleIgnored("cloud_order", terraformBlock) {
			continue
		}
		for _, cloudBlock := range terraformBlock.Body.Blocks {
			if cloudBlock.Type != "cloud" {
				continue
			}

			elements := syntaxElements(cloudBlock.Body)
			if len(elements) < 2 {
				continue
			}

			indices, needsSort, unsupportedIndex := rankedElementOrder(elements, cloudElementRank)
			if !needsSort {
				continue
			}
			if unsupportedIndex != -1 {
				diagnostics = append(diagnostics, transformDiagnostic{Subject: cloudBlock.Range(), Reason: "cloud body contains an unknown element"})
				continue
			}

			chunks, diagnostic := cloudBodyChunks(parsed, cloudBlock, elements)
			if diagnostic != nil {
				diagnostics = append(diagnostics, *diagnostic)
				continue
			}

			var replacement []byte
			for _, index := range indices {
				replacement = append(replacement, chunks[index].text...)
			}
			edits = append(edits, sourceEdit{start: chunks[0].start, end: chunks[len(chunks)-1].end, text: replacement})
			changes = append(changes, transformChange{
				Subject: cloudBlock.Range(),
				Rule:    "cloud_order",
				Reason:  "cloud body elements should be ordered organization, hostname, token, workspaces",
			})
		}
	}

	candidate = applySourceEdits(candidate, edits)
	if len(edits) > 0 {
		if diagnostic := validateSourceCandidate(parsed, candidate, map[string]sourceBodyPolicy{
			"terraform": {
				owner: "terraform", rank: terraformStableElementRank,
				children: map[string]sourceBodyPolicy{
					"cloud": {owner: "cloud", rank: cloudElementRank},
				},
			},
		}); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func transformCloudWorkspacesOrderWithChanges(parsed *parsedFile) ([]byte, []transformChange, []transformDiagnostic) {
	candidate := bytes.Clone(parsed.source)
	if parsed.ruleIgnored("cloud_workspaces_order", nil) {
		return candidate, nil, nil
	}
	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return candidate, nil, []transformDiagnostic{{Reason: "unsupported root body syntax"}}
	}

	var edits []sourceEdit
	var changes []transformChange
	var diagnostics []transformDiagnostic
	for _, terraformBlock := range root.Blocks {
		if terraformBlock.Type != "terraform" {
			continue
		}
		if parsed.ruleIgnored("cloud_workspaces_order", terraformBlock) {
			continue
		}
		for _, cloudBlock := range terraformBlock.Body.Blocks {
			if cloudBlock.Type != "cloud" {
				continue
			}

			var workspacesBlocks []*hclsyntax.Block
			for _, block := range cloudBlock.Body.Blocks {
				if block.Type == "workspaces" {
					workspacesBlocks = append(workspacesBlocks, block)
				}
			}
			if len(workspacesBlocks) == 0 {
				continue
			}

			for _, block := range workspacesBlocks {
				elements := syntaxElements(block.Body)
				indices, needsSort, _ := rankedElementOrder(elements, cloudWorkspacesElementRank)
				if !needsSort {
					continue
				}
				chunks, diagnostic := cloudBodyChunks(parsed, block, elements)
				if diagnostic != nil {
					diagnostics = append(diagnostics, *diagnostic)
					continue
				}

				var replacement []byte
				for _, index := range indices {
					replacement = append(replacement, chunks[index].text...)
				}
				edits = append(edits, sourceEdit{start: chunks[0].start, end: chunks[len(chunks)-1].end, text: replacement})
				changes = append(changes, transformChange{
					Subject: block.Range(),
					Rule:    "cloud_workspaces_order",
					Reason:  "workspaces attributes should be ordered name, project, tags",
				})
			}
		}
	}

	candidate = applySourceEdits(candidate, edits)
	if len(edits) > 0 {
		if diagnostic := validateSourceCandidate(parsed, candidate, map[string]sourceBodyPolicy{
			"terraform": {
				owner: "terraform", rank: terraformStableElementRank,
				children: map[string]sourceBodyPolicy{
					"cloud": {
						owner: "cloud", rank: terraformStableElementRank,
						children: map[string]sourceBodyPolicy{
							"workspaces": {owner: "workspaces", rank: cloudWorkspacesElementRank},
						},
					},
				},
			},
		}); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func cloudWorkspacesElementRank(element syntaxElement) (int, bool) {
	if element.attribute != nil {
		switch element.attribute.Name {
		case "name":
			return 0, true
		case "project":
			return 1, true
		case "tags":
			return 2, true
		default:
			return 3, true
		}
	}
	if element.block != nil {
		return 4, true
	}
	return 0, false
}

func cloudBodyChunks(parsed *parsedFile, cloudBlock *hclsyntax.Block, elements []syntaxElement) ([]sourceChunk, *transformDiagnostic) {
	leadingStarts := make([]int, len(elements))
	trailingEnds := make([]int, len(elements))
	for index, element := range elements {
		leadingStarts[index] = lineStart(parsed.source, element.rangeValue.Start.Byte)
		trailingEnds[index] = lineEnd(parsed.source, cloudElementEnd(element))
	}

	if diagnostic := assignSourceComments(parsed, cloudBlock, elements, leadingStarts, trailingEnds, sourceCommentPolicy{
		owner:                  "cloud",
		elementStart:           func(element syntaxElement) int { return element.rangeValue.Start.Byte },
		elementEnd:             cloudElementEnd,
		trailingCommentLineEnd: cloudCommentLineEnd,
		preserveEarliestStart:  true,
	}); diagnostic != nil {
		return nil, diagnostic
	}

	previousElementLine := -1
	for index, element := range elements {
		start := leadingStarts[index]
		elementLine := lineStart(parsed.source, element.rangeValue.Start.Byte)
		if elementLine == previousElementLine || !horizontalWhitespace(parsed.source[elementLine:element.rangeValue.Start.Byte]) {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "cloud elements do not occupy unambiguous standalone lines"}
		}
		previousElementLine = elementLine
		end := trailingEnds[index]
		if index+1 < len(elements) {
			end = leadingStarts[index+1]
		}
		if end < start {
			return nil, &transformDiagnostic{Subject: element.rangeValue, Reason: "cloud element comment chunks overlap"}
		}
	}
	if lineStart(parsed.source, cloudBlock.CloseBraceRange.Start.Byte) == previousElementLine {
		return nil, &transformDiagnostic{Subject: cloudBlock.Range(), Reason: "cloud elements do not occupy unambiguous standalone lines"}
	}
	return sourceChunksFromBounds(parsed.source, leadingStarts, trailingEnds), nil
}

func cloudCommentLineEnd(source []byte, end int) int {
	if end > 0 && (source[end-1] == '\n' || source[end-1] == '\r') {
		return end
	}
	return lineEnd(source, end)
}

func cloudElementRank(element syntaxElement) (int, bool) {
	if element.attribute != nil {
		switch element.attribute.Name {
		case "organization":
			return 0, true
		case "hostname":
			return 1, true
		case "token":
			return 2, true
		default:
			return 3, true
		}
	}
	if element.block != nil {
		if element.block.Type == "workspaces" {
			return 4, true
		}
		return 5, true
	}
	return 0, false
}

func cloudElementEnd(element syntaxElement) int {
	if element.block != nil {
		return element.block.CloseBraceRange.End.Byte
	}
	return element.rangeValue.End.Byte
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
		if token.Type != hclsyntax.TokenComment || token.Range.End.Byte > cursor {
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
		if token.Type != hclsyntax.TokenComment || token.Range.End.Byte > cursor {
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
	if !config.RequiredVersionOrder || parsed.ruleIgnored("required_version_order", nil) {
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
		if parsed.ruleIgnored("required_version_order", block) {
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

	candidate = applySourceEdits(candidate, edits)
	if len(edits) > 0 {
		if diagnostic := validateRequiredVersionCandidate(parsed, candidate); diagnostic != nil {
			return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
		}
	}
	return candidate, changes, diagnostics
}

func validateRequiredVersionCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
	return validateSourceCandidate(original, candidate, map[string]sourceBodyPolicy{
		"terraform": {owner: "terraform_required_version", rank: requiredVersionElementRank},
	})
}

func requiredVersionElementRank(element syntaxElement) (int, bool) {
	if element.attribute != nil && element.attribute.Name == "required_version" {
		return 0, true
	}
	return 1, true
}

func terraformBackendElementRank(element syntaxElement) (int, bool) {
	if element.attribute != nil && element.attribute.Name == "required_version" {
		return 0, true
	}
	if element.block != nil && (element.block.Type == "backend" || element.block.Type == "cloud") {
		return 1, true
	}
	return 2, true
}

func terraformStableElementRank(syntaxElement) (int, bool) {
	return 0, true
}

func validateProviderMetaCandidate(original *parsedFile, candidate []byte) *transformDiagnostic {
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
		return &transformDiagnostic{Subject: subject, Reason: "candidate failed HCL reparse: " + parseDiagnostics.Error()}
	}
	validatedBody, validatedOK := validated.body.(*hclsyntax.Body)
	if !validatedOK || !sameProviderMetaBlocks(originalBody, validatedBody, original.source, validated.source) {
		return &transformDiagnostic{Subject: originalBody.Range(), Reason: "candidate changed provider_meta block bytes or relative order"}
	}
	return nil
}

func sameProviderMetaBlocks(original, candidate *hclsyntax.Body, originalSource, candidateSource []byte) bool {
	originalTerraformBodies := terraformBodies(original)
	candidateTerraformBodies := terraformBodies(candidate)
	if len(originalTerraformBodies) != len(candidateTerraformBodies) {
		return false
	}
	for index, originalBody := range originalTerraformBodies {
		originalElements := syntaxElements(originalBody)
		candidateElements := syntaxElements(candidateTerraformBodies[index])
		var originalBlocks []syntaxElement
		var candidateBlocks []syntaxElement
		for _, element := range originalElements {
			if element.block != nil && element.block.Type == "provider_meta" {
				originalBlocks = append(originalBlocks, element)
			}
		}
		for _, element := range candidateElements {
			if element.block != nil && element.block.Type == "provider_meta" {
				candidateBlocks = append(candidateBlocks, element)
			}
		}
		if len(originalBlocks) != len(candidateBlocks) {
			return false
		}
		for blockIndex, originalBlock := range originalBlocks {
			candidateBlock := candidateBlocks[blockIndex]
			if !sameLabels(originalBlock.labels, candidateBlock.labels) ||
				!sameRangeBytes(originalSource, originalBlock.rangeValue, candidateSource, candidateBlock.rangeValue) {
				return false
			}
		}
	}
	return true
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

func terraformBodies(body *hclsyntax.Body) []*hclsyntax.Body {
	bodies := make([]*hclsyntax.Body, 0, len(body.Blocks))
	for _, block := range body.Blocks {
		if block.Type == "terraform" {
			bodies = append(bodies, block.Body)
		}
	}
	return bodies
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

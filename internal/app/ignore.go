package app

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type ignoreDirectives struct {
	file   map[string]struct{}
	blocks map[int]map[string]struct{}
}

var ignoreRuleNames = map[string]struct{}{
	"order_action":             {},
	"order_backend":            {},
	"order_check":              {},
	"order_cloud":              {},
	"order_cloud_workspaces":   {},
	"order_data":               {},
	"order_ephemeral":          {},
	"order_import":             {},
	"order_list":               {},
	"order_module":             {},
	"order_moved":              {},
	"order_output":             {},
	"order_provider":           {},
	"order_removed":            {},
	"order_required_providers": {},
	"order_required_version":   {},
	"order_resource":           {},
	"order_variable":           {},
	"rename_locals_file":       {},
	"rename_outputs_file":      {},
	"rename_terraform_file":    {},
	"rename_variables_file":    {},
	"replace_required_version": {},
}

type ignoreComment struct {
	end   int
	rules map[string]struct{}
	file  bool
}

func commentInsideRootElement(root *hclsyntax.Body, comment hcl.Range) bool {
	for _, attribute := range root.Attributes {
		if comment.Start.Byte >= attribute.SrcRange.Start.Byte && comment.End.Byte <= attribute.SrcRange.End.Byte {
			return true
		}
	}
	for _, block := range root.Blocks {
		blockRange := block.Range()
		if comment.Start.Byte >= blockRange.Start.Byte && comment.End.Byte <= blockRange.End.Byte {
			return true
		}
	}
	return false
}

func gapHasNoBlankLines(source []byte, start, end int) bool {
	if start < 0 || end < start || end > len(source) {
		return false
	}
	blockLineStart := lineStart(source, end)
	for line := lineStart(source, start); line < blockLineStart; {
		nextLine := bytes.IndexByte(source[line:blockLineStart], '\n')
		lineEnd := blockLineStart
		if nextLine >= 0 {
			lineEnd = line + nextLine
		}
		if horizontalWhitespace(bytes.TrimSuffix(source[line:lineEnd], []byte{'\r'})) {
			return false
		}
		line = lineEnd + 1
	}
	return true
}

func isFileRenameRule(rule string) bool {
	return strings.HasPrefix(rule, "rename_")
}

func mergeIgnoreRules(target, source map[string]struct{}) {
	for rule := range source {
		target[rule] = struct{}{}
	}
}

func nextRootElement(root *hclsyntax.Body, offset int) (rootElement, bool) {
	var next rootElement
	found := false
	for _, block := range root.Blocks {
		start := block.Range().Start.Byte
		if start >= offset && (!found || start < next.start) {
			next = rootElement{start: start, block: block}
			found = true
		}
	}
	for _, attribute := range root.Attributes {
		start := attribute.SrcRange.Start.Byte
		if start >= offset && (!found || start < next.start) {
			next = rootElement{start: start, attribute: attribute}
			found = true
		}
	}
	return next, found
}

func parseHCLIgnoreComment(token hclsyntax.Token) (ignoreComment, bool) {
	text := string(token.Bytes)
	if strings.HasPrefix(text, "# tfstylus-ignore-file:") {
		rules, ok := parseIgnoreRuleList(text, "# tfstylus-ignore-file:")
		if !ok {
			return ignoreComment{}, false
		}
		return ignoreComment{end: token.Range.End.Byte, rules: rules, file: true}, true
	}
	if strings.HasPrefix(text, "# tfstylus-ignore:") {
		rules, ok := parseIgnoreRuleList(text, "# tfstylus-ignore:")
		if !ok {
			return ignoreComment{}, false
		}
		return ignoreComment{end: token.Range.End.Byte, rules: rules}, true
	}
	return ignoreComment{}, false
}

func parseHCLIgnoreDirectives(source []byte, tokens hclsyntax.Tokens, body hcl.Body) ignoreDirectives {
	directives := ignoreDirectives{
		file:   make(map[string]struct{}),
		blocks: make(map[int]map[string]struct{}),
	}
	root, ok := body.(*hclsyntax.Body)
	if !ok {
		return directives
	}

	firstElement := len(source)
	for _, attribute := range root.Attributes {
		if attribute.SrcRange.Start.Byte < firstElement {
			firstElement = attribute.SrcRange.Start.Byte
		}
	}
	for _, block := range root.Blocks {
		if block.Range().Start.Byte < firstElement {
			firstElement = block.Range().Start.Byte
		}
	}

	comments := make([]ignoreComment, 0)
	for _, token := range tokens {
		if token.Type != hclsyntax.TokenComment || !standaloneHCLComment(source, token) {
			continue
		}
		comment, ok := parseHCLIgnoreComment(token)
		if !ok || commentInsideRootElement(root, token.Range) {
			continue
		}
		if comment.file {
			if token.Range.Start.Byte < firstElement {
				mergeIgnoreRules(directives.file, comment.rules)
			}
			continue
		}
		comments = append(comments, comment)
	}

	for _, comment := range comments {
		next, ok := nextRootElement(root, comment.end)
		if !ok || next.block == nil || !gapHasNoBlankLines(source, comment.end, next.start) {
			continue
		}
		rules := make(map[string]struct{})
		for rule := range comment.rules {
			if !isFileRenameRule(rule) {
				rules[rule] = struct{}{}
			}
		}
		if len(rules) > 0 {
			start := next.block.Range().Start.Byte
			if directives.blocks[start] == nil {
				directives.blocks[start] = make(map[string]struct{})
			}
			mergeIgnoreRules(directives.blocks[start], rules)
		}
	}
	return directives
}

func parseIgnoreRuleList(text, prefix string) (map[string]struct{}, bool) {
	if !strings.HasPrefix(text, prefix) {
		return nil, false
	}
	remainder := strings.TrimSpace(text[len(prefix):])
	if remainder == "" {
		return nil, false
	}
	rules := make(map[string]struct{})
	for _, candidate := range strings.Split(remainder, ",") {
		name := strings.TrimSpace(candidate)
		if !validIgnoreRuleName(name) {
			return nil, false
		}
		if _, known := ignoreRuleNames[name]; known {
			rules[name] = struct{}{}
		}
	}
	return rules, true
}

type rootElement struct {
	start     int
	block     *hclsyntax.Block
	attribute *hclsyntax.Attribute
}

func parseJSONFileIgnoreDirective(source []byte) ignoreDirectives {
	directives := ignoreDirectives{file: make(map[string]struct{})}
	decoder := json.NewDecoder(bytes.NewReader(source))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return directives
	}
	key, err := decoder.Token()
	if err != nil || key != "//" {
		return directives
	}
	value, err := decoder.Token()
	if err != nil {
		return directives
	}
	text, ok := value.(string)
	if !ok {
		return directives
	}
	rules, ok := parseIgnoreRuleList(text, "tfstylus-ignore-file:")
	if ok {
		directives.file = rules
	}
	return directives
}

func (parsed *parsedFile) ruleIgnored(rule string, block *hclsyntax.Block) bool {
	if _, ignored := parsed.ignores.file[rule]; ignored {
		return true
	}
	if block == nil {
		return false
	}
	_, ignored := parsed.ignores.blocks[block.Range().Start.Byte][rule]
	return ignored
}

func standaloneHCLComment(source []byte, token hclsyntax.Token) bool {
	start := lineStart(source, token.Range.Start.Byte)
	prefix := source[start:token.Range.Start.Byte]
	if start == 0 && bytes.HasPrefix(prefix, []byte{0xef, 0xbb, 0xbf}) {
		prefix = prefix[3:]
	}
	if !horizontalWhitespace(prefix) {
		return false
	}
	if bytes.HasSuffix(token.Bytes, []byte{'\n'}) {
		return true
	}
	end := token.Range.End.Byte
	if newline := bytes.IndexByte(source[end:], '\n'); newline >= 0 {
		end += newline
	} else {
		end = len(source)
	}
	return horizontalWhitespace(source[token.Range.End.Byte:end])
}

func validIgnoreRuleName(name string) bool {
	if name == "" {
		return false
	}
	for index, character := range name {
		if character >= 'a' && character <= 'z' || index > 0 && (character >= '0' && character <= '9' || character == '_') {
			continue
		}
		return false
	}
	return true
}

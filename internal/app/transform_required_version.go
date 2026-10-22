package app

import (
	"bytes"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

func transformRequiredVersionReplacement(parsed *parsedFile, constraint string) ([]byte, []transformChange, []transformDiagnostic) {
	root, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return bytes.Clone(parsed.source), nil, []transformDiagnostic{{Reason: "unsupported replacement body syntax"}}
	}
	var edits []sourceEdit
	var changes []transformChange
	var diagnostics []transformDiagnostic
	for _, block := range root.Blocks {
		if block.Type != "terraform" {
			continue
		}
		if parsed.ruleIgnored("replace_required_version", block) {
			continue
		}
		attribute := block.Body.Attributes["required_version"]
		if attribute == nil {
			continue
		}
		value, supported := quotedStringLiteral(attribute.Expr, parsed.source)
		if !supported {
			diagnostics = append(diagnostics, transformDiagnostic{Subject: attribute.Expr.Range(), Reason: "required_version replacement requires a static quoted string"})
			continue
		}
		if value == constraint {
			continue
		}
		expressionRange := attribute.Expr.Range()
		edits = append(edits, sourceEdit{start: expressionRange.Start.Byte, end: expressionRange.End.Byte, text: hclwrite.TokensForValue(cty.StringVal(constraint)).Bytes()})
		changes = append(changes, transformChange{Subject: expressionRange, Rule: "terraform/replace-required-version", Reason: "replace required_version with configured constraint"})
	}
	candidate := applySourceEdits(parsed.source, edits)
	if diagnostic := validateRequiredVersionReplacement(parsed, candidate, constraint); diagnostic != nil {
		return bytes.Clone(parsed.source), nil, append(diagnostics, *diagnostic)
	}
	return candidate, changes, diagnostics
}

func validateRequiredVersionReplacement(original *parsedFile, candidate []byte, constraint string) *transformDiagnostic {
	root, ok := original.body.(*hclsyntax.Body)
	if !ok {
		return &transformDiagnostic{Reason: "unsupported replacement body syntax"}
	}
	failure := &transformDiagnostic{Subject: root.Range(), Reason: "required_version replacement changed non-target bytes or configured value"}
	validated, diagnostics := parseHCLSource(root.Range().Filename, candidate)
	if diagnostics.HasErrors() {
		return failure
	}
	if _, ok := validated.body.(*hclsyntax.Body); !ok {
		return failure
	}
	var expected []byte
	cursor := 0
	for _, block := range root.Blocks {
		if block.Type != "terraform" {
			continue
		}
		if original.ruleIgnored("replace_required_version", block) {
			continue
		}
		attribute := block.Body.Attributes["required_version"]
		if attribute == nil {
			continue
		}
		value, supported := quotedStringLiteral(attribute.Expr, original.source)
		if !supported || value == constraint {
			continue
		}
		expressionRange := attribute.Expr.Range()
		expected = append(expected, original.source[cursor:expressionRange.Start.Byte]...)
		expected = append(expected, hclwrite.TokensForValue(cty.StringVal(constraint)).Bytes()...)
		cursor = expressionRange.End.Byte
	}
	expected = append(expected, original.source[cursor:]...)
	if !bytes.Equal(candidate, expected) {
		return failure
	}
	return nil
}

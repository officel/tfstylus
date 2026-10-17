package app

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	hcljson "github.com/hashicorp/hcl/v2/json"
)

type parsedFile struct {
	source []byte
	body   hcl.Body
	tokens hclsyntax.Tokens
}

func parseHCLSource(filename string, source []byte) (*parsedFile, hcl.Diagnostics) {
	file, diagnostics := hclsyntax.ParseConfig(source, filename, hcl.Pos{Line: 1, Column: 1})
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	tokens, tokenDiagnostics := hclsyntax.LexConfig(source, filename, hcl.Pos{Line: 1, Column: 1})
	diagnostics = append(diagnostics, tokenDiagnostics...)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	return &parsedFile{
		source: bytes.Clone(source),
		body:   file.Body,
		tokens: tokens,
	}, diagnostics
}

func parseJSONSource(filename string, source []byte) (*parsedFile, hcl.Diagnostics) {
	file, diagnostics := hcljson.Parse(source, filename)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	return &parsedFile{
		source: bytes.Clone(source),
		body:   file.Body,
	}, diagnostics
}

func parseTargetFile(filename string) error {
	source, err := os.ReadFile(filename) // #nosec G304 -- The caller selects the local Terraform file to validate.
	if err != nil {
		return fmt.Errorf("read Terraform file %q: %w", filename, err)
	}

	var diagnostics hcl.Diagnostics
	if strings.HasSuffix(filename, ".tf.json") {
		_, diagnostics = parseJSONSource(filename, source)
	} else {
		_, diagnostics = parseHCLSource(filename, source)
	}
	if diagnostics.HasErrors() {
		return fmt.Errorf("parse Terraform file %q:\n%s", filename, diagnostics.Error())
	}
	return nil
}

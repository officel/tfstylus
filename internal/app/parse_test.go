package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
)

func TestParseHCLSourceReportsLocation(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "main.tf")
	source := []byte("resource \"example\" {\n  value =\n}\n")

	parsed, diagnostics := parseHCLSource(filename, source)
	if parsed != nil {
		t.Fatal("parseHCLSource() returned a parsed file for invalid HCL")
	}
	if !diagnostics.HasErrors() {
		t.Fatal("parseHCLSource() diagnostics have no errors")
	}

	for _, diagnostic := range diagnostics {
		if diagnostic.Severity != hcl.DiagError {
			continue
		}
		if diagnostic.Subject == nil {
			t.Fatal("parseHCLSource() error diagnostic has no subject range")
		}
		if diagnostic.Subject.Filename != filename {
			t.Errorf("diagnostic filename = %q, want %q", diagnostic.Subject.Filename, filename)
		}
		if diagnostic.Subject.Start.Line < 1 || diagnostic.Subject.Start.Column < 1 {
			t.Errorf("diagnostic position = %d:%d, want a 1-based position", diagnostic.Subject.Start.Line, diagnostic.Subject.Start.Column)
		}
		return
	}
	t.Fatal("parseHCLSource() did not return an error diagnostic")
}

func TestParseHCLSourceAcceptsUnknownAttributes(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "main.tf")
	source := []byte("# retained comment\nunknown = true\n")

	parsed, diagnostics := parseHCLSource(filename, source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want no errors", diagnostics.Error())
	}
	if parsed == nil {
		t.Fatal("parseHCLSource() returned no parsed file")
	}
	if !bytes.Equal(parsed.source, source) {
		t.Errorf("parsed source = %q, want original source %q", parsed.source, source)
	}
}

func TestParseJSONSourceReportsLocation(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "main.tf.json")
	source := []byte("{\n  \"unknown\": }\n")

	parsed, diagnostics := parseJSONSource(filename, source)
	if parsed != nil {
		t.Fatal("parseJSONSource() returned a parsed file for invalid JSON")
	}
	if !diagnostics.HasErrors() {
		t.Fatal("parseJSONSource() diagnostics have no errors")
	}
	if diagnostics[0].Subject == nil {
		t.Fatal("parseJSONSource() error diagnostic has no subject range")
	}
	if diagnostics[0].Subject.Filename != filename {
		t.Errorf("diagnostic filename = %q, want %q", diagnostics[0].Subject.Filename, filename)
	}
	if diagnostics[0].Subject.Start.Line < 1 || diagnostics[0].Subject.Start.Column < 1 {
		t.Errorf("diagnostic position = %d:%d, want a 1-based position", diagnostics[0].Subject.Start.Line, diagnostics[0].Subject.Start.Column)
	}
}

func TestParseJSONSourceAcceptsUnknownAttributes(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "main.tf.json")
	source := []byte("{\"unknown\": true}")

	parsed, diagnostics := parseJSONSource(filename, source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseJSONSource() diagnostics = %s, want no errors", diagnostics.Error())
	}
	if parsed == nil {
		t.Fatal("parseJSONSource() returned no parsed file")
	}
	if !bytes.Equal(parsed.source, source) {
		t.Errorf("parsed source = %q, want original source %q", parsed.source, source)
	}
}

func TestParseTargetFileReadError(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "missing.tf")

	err := parseTargetFile(filename)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("parseTargetFile() error = %v, want an error matching os.ErrNotExist", err)
	}
	if !strings.Contains(err.Error(), filename) {
		t.Errorf("parseTargetFile() error = %q, want it to contain %q", err, filename)
	}
}

package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/pmezard/go-difflib/difflib"
)

type exitCodeError struct {
	code int
	err  error
}

func (e *exitCodeError) Error() string {
	return e.err.Error()
}

func (e *exitCodeError) Unwrap() error {
	return e.err
}

func (e *exitCodeError) ExitCode() int {
	return e.code
}

func withExitCode(code int, err error) error {
	return &exitCodeError{code: code, err: err}
}

type checkReport struct {
	filename    string
	source      []byte
	candidate   []byte
	fileInfo    os.FileInfo
	changes     []transformChange
	diagnostics []transformDiagnostic
}

func runCheck(targets []targetDirectory, configs []Config, root string, showDiff bool, output io.Writer) error {
	reports, failures := prepareCheckReports(targets, configs)
	hasChanges := false

	for _, report := range reports {
		if len(report.changes) > 0 {
			hasChanges = true
		}
	}

	for _, report := range reports {
		name := displayPath(root, report.filename)
		for _, change := range report.changes {
			if _, err := fmt.Fprintf(output, "%s:%d:%d: %s: %s\n", name, change.Subject.Start.Line, change.Subject.Start.Column, change.Rule, change.Reason); err != nil {
				return withExitCode(2, fmt.Errorf("write check output: %w", err))
			}
		}
		for _, diagnostic := range report.diagnostics {
			if diagnostic.Subject.Start.Line > 0 {
				if _, err := fmt.Fprintf(output, "%s:%d:%d: warning: skipped: %s\n", name, diagnostic.Subject.Start.Line, diagnostic.Subject.Start.Column, diagnostic.Reason); err != nil {
					return withExitCode(2, fmt.Errorf("write check output: %w", err))
				}
			} else if _, err := fmt.Fprintf(output, "%s: warning: skipped: %s\n", name, diagnostic.Reason); err != nil {
				return withExitCode(2, fmt.Errorf("write check output: %w", err))
			}
		}
		if showDiff && len(report.changes) > 0 {
			unified, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
				A:        difflib.SplitLines(string(report.source)),
				B:        difflib.SplitLines(string(report.candidate)),
				FromFile: "a/" + name,
				ToFile:   "b/" + name,
				Context:  3,
			})
			if err != nil {
				return withExitCode(2, fmt.Errorf("generate diff for %q: %w", report.filename, err))
			}
			if _, err := io.WriteString(output, unified); err != nil {
				return withExitCode(2, fmt.Errorf("write check diff: %w", err))
			}
		}
	}

	if err := errors.Join(failures...); err != nil {
		return withExitCode(2, err)
	}
	if hasChanges {
		return withExitCode(1, errors.New("check found pending changes"))
	}
	return nil
}

func prepareCheckReports(targets []targetDirectory, configs []Config) ([]checkReport, []error) {
	reports := make([]checkReport, 0)
	var failures []error

	for targetIndex, target := range targets {
		for _, filename := range target.files {
			source, err := os.ReadFile(filename)
			if err != nil {
				failures = append(failures, fmt.Errorf("read Terraform file %q: %w", filename, err))
				continue
			}

			var parsed *parsedFile
			var parseDiagnostics hcl.Diagnostics
			if filepath.Ext(filename) == ".json" {
				parsed, parseDiagnostics = parseJSONSource(filename, source)
			} else {
				parsed, parseDiagnostics = parseHCLSource(filename, source)
			}
			if parseDiagnostics.HasErrors() {
				failures = append(failures, fmt.Errorf("parse Terraform file %q:\n%s", filename, parseDiagnostics.Error()))
				continue
			}

			candidate := source
			var changes []transformChange
			var transformDiagnostics []transformDiagnostic
			if filepath.Ext(filename) != ".json" {
				candidate, changes, transformDiagnostics = transformTerraformOrderWithChanges(parsed, configs[targetIndex])
			}
			reports = append(reports, checkReport{
				filename:    filename,
				source:      source,
				candidate:   candidate,
				changes:     changes,
				diagnostics: transformDiagnostics,
			})
			for _, diagnostic := range transformDiagnostics {
				failures = append(failures, fmt.Errorf("%s: %s", filename, diagnostic.Reason))
			}
		}
	}

	return reports, failures
}

func displayPath(root, filename string) string {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return filename
	}
	relative, err := filepath.Rel(absoluteRoot, filename)
	if err != nil {
		return filename
	}
	return relative
}

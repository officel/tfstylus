package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
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
	renameTo    string
}

func runCheck(targets []targetDirectory, configs []Config, root string, showDiff bool, output io.Writer) error {
	reports, failures := prepareCheckReports(targets, configs)
	hasChanges := false

	for _, report := range reports {
		if len(report.changes) > 0 || report.renameTo != "" {
			hasChanges = true
		}
	}

	for _, report := range reports {
		name := displayPath(root, report.filename)
		if report.renameTo != "" {
			if _, err := fmt.Fprintf(output, "%s: rename to %s [%s] %s\n", name, displayPath(root, report.renameTo), localsRenameRule, localsRenameReason); err != nil {
				return withExitCode(2, fmt.Errorf("write check output: %w", err))
			}
		}
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
		if showDiff && (len(report.changes) > 0 || report.renameTo != "") {
			toName := name
			if report.renameTo != "" {
				toName = displayPath(root, report.renameTo)
				similarity := 100
				if !bytes.Equal(report.source, report.candidate) {
					similarity = int(difflib.NewMatcher(difflib.SplitLines(string(report.source)), difflib.SplitLines(string(report.candidate))).Ratio() * 100)
				}
				if _, err := fmt.Fprintf(output, "diff --git a/%s b/%s\nsimilarity index %d%%\nrename from %s\nrename to %s\n", name, toName, similarity, name, toName); err != nil {
					return withExitCode(2, fmt.Errorf("write check diff: %w", err))
				}
			}
			unified, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
				A:        difflib.SplitLines(string(report.source)),
				B:        difflib.SplitLines(string(report.candidate)),
				FromFile: "a/" + name,
				ToFile:   "b/" + toName,
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
			if strings.HasSuffix(filename, ".tfquery.hcl") {
				if configs[targetIndex].ListOrder {
					candidate, changes, transformDiagnostics = transformListOrderWithChanges(parsed)
				}
			} else if filepath.Ext(filename) != ".json" {
				candidate, changes, transformDiagnostics = transformTerraformOrderWithChanges(parsed, configs[targetIndex])
			}
			renameTo, renameFailure := localsRenameTarget(filename, parsed, configs[targetIndex].LocalsFileRename)
			if renameFailure != nil {
				failures = append(failures, renameFailure)
			}
			reports = append(reports, checkReport{
				filename:    filename,
				source:      source,
				candidate:   candidate,
				changes:     changes,
				diagnostics: transformDiagnostics,
				renameTo:    renameTo,
			})
			for _, diagnostic := range transformDiagnostics {
				failures = append(failures, fmt.Errorf("%s: %s", filename, diagnostic.Reason))
			}
		}
	}

	return reports, failures
}

const (
	localsRenameRule   = "file-name/locals"
	localsRenameReason = "local.tf contains a locals block and locals.tf does not exist"
)

// localsRenameTarget returns the rename destination, or "" when the rule does not apply.
func localsRenameTarget(filename string, parsed *parsedFile, enabled bool) (string, error) {
	if !enabled || filepath.Base(filename) != "local.tf" || !hasTopLevelBlock(parsed, "locals") {
		return "", nil
	}
	destination := filepath.Join(filepath.Dir(filename), "locals.tf")
	if _, err := os.Lstat(destination); err == nil {
		return "", nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect rename destination %q: %w", destination, err)
	}
	info, err := os.Lstat(filename)
	if err != nil {
		return "", fmt.Errorf("inspect rename source %q: %w", filename, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("refusing to rename symbolic link %q", filename)
	}
	return destination, nil
}

func hasTopLevelBlock(parsed *parsedFile, blockType string) bool {
	body, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		return false
	}
	for _, block := range body.Blocks {
		if block.Type == blockType {
			return true
		}
	}
	return false
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

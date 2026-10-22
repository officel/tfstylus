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

type planReport struct {
	filename     string
	source       []byte
	candidate    []byte
	fileInfo     os.FileInfo
	changes      []transformChange
	diagnostics  []transformDiagnostic
	renameTo     string
	renameRule   string
	renameReason string
}

func runPlan(targets []targetDirectory, configs []Config, root string, output io.Writer) error {
	reports, failures := preparePlanReports(targets, configs)
	hasChanges := false

	for _, report := range reports {
		if len(report.changes) > 0 || report.renameTo != "" {
			hasChanges = true
		}
	}

	for _, report := range reports {
		name := displayPath(root, report.filename)
		if report.renameTo != "" {
			if _, err := fmt.Fprintf(output, "%s: rename to %s [%s] %s\n", name, displayPath(root, report.renameTo), report.renameRule, report.renameReason); err != nil {
				return withExitCode(2, fmt.Errorf("write plan output: %w", err))
			}
		}
		for _, change := range report.changes {
			if _, err := fmt.Fprintf(output, "%s:%d:%d: %s: %s\n", name, change.Subject.Start.Line, change.Subject.Start.Column, change.Rule, change.Reason); err != nil {
				return withExitCode(2, fmt.Errorf("write plan output: %w", err))
			}
		}
		for _, diagnostic := range report.diagnostics {
			if diagnostic.Subject.Start.Line > 0 {
				if _, err := fmt.Fprintf(output, "%s:%d:%d: warning: skipped: %s\n", name, diagnostic.Subject.Start.Line, diagnostic.Subject.Start.Column, diagnostic.Reason); err != nil {
					return withExitCode(2, fmt.Errorf("write plan output: %w", err))
				}
			} else if _, err := fmt.Fprintf(output, "%s: warning: skipped: %s\n", name, diagnostic.Reason); err != nil {
				return withExitCode(2, fmt.Errorf("write plan output: %w", err))
			}
		}
		if len(report.changes) > 0 || report.renameTo != "" {
			if err := writeReportDiff(report, root, output); err != nil {
				return withExitCode(2, fmt.Errorf("write plan diff: %w", err))
			}
		}
	}

	if err := errors.Join(failures...); err != nil {
		return withExitCode(2, err)
	}
	if hasChanges {
		return withExitCode(1, errors.New("plan found pending changes"))
	}
	if _, err := fmt.Fprintln(output, "No changes."); err != nil {
		return withExitCode(2, fmt.Errorf("write plan output: %w", err))
	}
	return nil
}

func writeReportDiff(report planReport, root string, output io.Writer) error {
	name := displayPath(root, report.filename)
	toName := name
	if report.renameTo != "" {
		toName = displayPath(root, report.renameTo)
		similarity := 100
		if !bytes.Equal(report.source, report.candidate) {
			similarity = int(difflib.NewMatcher(difflib.SplitLines(string(report.source)), difflib.SplitLines(string(report.candidate))).Ratio() * 100)
		}
		if _, err := fmt.Fprintf(output, "diff --git a/%s b/%s\nsimilarity index %d%%\nrename from %s\nrename to %s\n", name, toName, similarity, name, toName); err != nil {
			return err
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
		return fmt.Errorf("generate diff for %q: %w", report.filename, err)
	}
	_, err = io.WriteString(output, unified)
	return err
}

func preparePlanReports(targets []targetDirectory, configs []Config) ([]planReport, []error) {
	reports := make([]planReport, 0)
	var failures []error

	for targetIndex, target := range targets {
		reportStart := len(reports)
		parsedFiles := make(map[string]*parsedFile, len(target.files))
		for _, filename := range target.files {
			source, err := os.ReadFile(filename) // #nosec G304 -- Read Terraform files discovered in the CLI-selected target directory.
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
			parsedFiles[filename] = parsed

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
			rename, renameFailure := fileRenameTarget(filename, parsed, configs[targetIndex])
			if renameFailure != nil {
				failures = append(failures, renameFailure)
			}
			reports = append(reports, planReport{
				filename:     filename,
				source:       source,
				candidate:    candidate,
				changes:      changes,
				diagnostics:  transformDiagnostics,
				renameTo:     rename.destination,
				renameRule:   rename.rule,
				renameReason: rename.reason,
			})
			for _, diagnostic := range transformDiagnostics {
				failures = append(failures, fmt.Errorf("%s: %s", filename, diagnostic.Reason))
			}
		}
		terraformSource, rename, renameFailure := terraformFileRenameTarget(target, parsedFiles, configs[targetIndex])
		if renameFailure != nil {
			failures = append(failures, renameFailure)
		}
		if terraformSource != "" {
			for reportIndex := reportStart; reportIndex < len(reports); reportIndex++ {
				if reports[reportIndex].filename == terraformSource {
					reports[reportIndex].renameTo = rename.destination
					reports[reportIndex].renameRule = rename.rule
					reports[reportIndex].renameReason = rename.reason
					break
				}
			}
		}
	}

	return reports, failures
}

func terraformFileRenameTarget(target targetDirectory, parsedFiles map[string]*parsedFile, config Config) (string, fileRenamePlan, error) {
	if !config.TerraformFileRename {
		return "", fileRenamePlan{}, nil
	}
	destination := filepath.Join(target.directory, "terraform.tf")
	if _, err := os.Lstat(destination); err == nil {
		return "", fileRenamePlan{}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fileRenamePlan{}, fmt.Errorf("inspect rename destination %q: %w", destination, err)
	}

	for _, name := range []string{"backend.tf", "providers.tf", "versions.tf"} {
		source := filepath.Join(target.directory, name)
		parsed, exists := parsedFiles[source]
		if !exists || parsed.ruleIgnored("terraform_file_rename", nil) || !hasTopLevelBlock(parsed, "terraform") {
			continue
		}
		info, err := os.Lstat(source)
		if err != nil {
			return "", fileRenamePlan{}, fmt.Errorf("inspect rename source %q: %w", source, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fileRenamePlan{}, fmt.Errorf("refusing to rename symbolic link %q", source)
		}
		reason := name + " " + terraformRenameReason
		return source, fileRenamePlan{
			destination: destination,
			rule:        terraformRenameRule,
			reason:      reason,
		}, nil
	}
	return "", fileRenamePlan{}, nil
}

const (
	localsRenameRule      = "file-name/locals"
	localsRenameReason    = "local.tf contains a locals block and locals.tf does not exist"
	terraformRenameRule   = "file-name/terraform"
	terraformRenameReason = "contains a terraform block and terraform.tf does not exist"
)

type fileRenamePlan struct {
	destination string
	rule        string
	reason      string
}

func fileRenameTarget(filename string, parsed *parsedFile, config Config) (fileRenamePlan, error) {
	var enabled bool
	var blockType, destinationName, rule, configRule, reason string
	switch filepath.Base(filename) {
	case "local.tf":
		enabled, blockType, destinationName = config.LocalsFileRename, "locals", "locals.tf"
		rule, reason = localsRenameRule, localsRenameReason
		configRule = "locals_file_rename"
	case "output.tf":
		enabled, blockType, destinationName = config.OutputsFileRename, "output", "outputs.tf"
		rule = "file-name/outputs"
		reason = "output.tf contains an output block and outputs.tf does not exist"
		configRule = "outputs_file_rename"
	case "variable.tf":
		enabled, blockType, destinationName = config.VariablesFileRename, "variable", "variables.tf"
		rule = "file-name/variables"
		reason = "variable.tf contains a variable block and variables.tf does not exist"
		configRule = "variables_file_rename"
	default:
		return fileRenamePlan{}, nil
	}
	if !enabled || parsed.ruleIgnored(configRule, nil) || !hasTopLevelBlock(parsed, blockType) {
		return fileRenamePlan{}, nil
	}
	destination := filepath.Join(filepath.Dir(filename), destinationName)
	if _, err := os.Lstat(destination); err == nil {
		return fileRenamePlan{}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fileRenamePlan{}, fmt.Errorf("inspect rename destination %q: %w", destination, err)
	}
	info, err := os.Lstat(filename)
	if err != nil {
		return fileRenamePlan{}, fmt.Errorf("inspect rename source %q: %w", filename, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fileRenamePlan{}, fmt.Errorf("refusing to rename symbolic link %q", filename)
	}
	return fileRenamePlan{destination: destination, rule: rule, reason: reason}, nil
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

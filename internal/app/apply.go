package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type applyFileOperations struct {
	createTemp func(string, string) (*os.File, error)
	write      func(*os.File, []byte) error
	rename     func(string, string) error
	link       func(string, string) error
}

func defaultApplyFileOperations() applyFileOperations {
	return applyFileOperations{
		createTemp: os.CreateTemp,
		write: func(file *os.File, content []byte) error {
			written, err := file.Write(content)
			if err != nil {
				return err
			}
			if written != len(content) {
				return io.ErrShortWrite
			}
			return nil
		},
		rename: os.Rename,
		link:   os.Link,
	}
}

func applyTargets(targets []targetDirectory, configs []Config, root string, showDiff bool, output io.Writer) error {
	reports, failures := preparePlanReports(targets, configs)
	if err := errors.Join(failures...); err != nil {
		return withExitCode(2, err)
	}

	for index := range reports {
		if len(reports[index].diagnostics) > 0 {
			return withExitCode(2, fmt.Errorf("%s: %s", reports[index].filename, reports[index].diagnostics[0].Reason))
		}
		if len(reports[index].changes) == 0 && reports[index].renameTo == "" {
			continue
		}
		info, err := verifyApplyTarget(reports[index], nil)
		if err != nil {
			return withExitCode(2, err)
		}
		reports[index].fileInfo = info
	}

	for _, report := range reports {
		if len(report.changes) == 0 {
			continue
		}
		if err := applyReport(report); err != nil {
			return withExitCode(2, fmt.Errorf("write Terraform file %q: %w", report.filename, err))
		}
	}

	// rename は全ファイルの内容置換が成功した後に行う。
	for _, report := range reports {
		if report.renameTo == "" {
			continue
		}
		if err := applyRename(report); err != nil {
			return withExitCode(2, fmt.Errorf("rename Terraform file %q: %w", report.filename, err))
		}
	}
	if showDiff {
		for _, report := range reports {
			if len(report.changes) == 0 && report.renameTo == "" {
				continue
			}
			if err := writeReportDiff(report, root, output); err != nil {
				return withExitCode(2, fmt.Errorf("write apply diff: %w", err))
			}
		}
	}
	return nil
}

// applyRename は os.Link が既存の宛先を上書きしないことを利用して rename する。
func applyRename(report planReport) error {
	return applyRenameWithOperations(report, defaultApplyFileOperations())
}

func applyRenameWithOperations(report planReport, operations applyFileOperations) error {
	info, err := os.Lstat(report.filename)
	if err != nil {
		return fmt.Errorf("inspect rename source: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("rename source %q is not a regular file", report.filename)
	}
	content, err := os.ReadFile(report.filename)
	if err != nil {
		return fmt.Errorf("re-read rename source: %w", err)
	}
	if !bytes.Equal(content, report.candidate) {
		return fmt.Errorf("rename source %q changed since it was processed", report.filename)
	}
	if err := operations.link(report.filename, report.renameTo); err != nil {
		return fmt.Errorf("link to %q: %w", report.renameTo, err)
	}
	if err := os.Remove(report.filename); err != nil {
		return fmt.Errorf("remove original after link (both files exist): %w", err)
	}
	return nil
}

func verifyApplyTarget(report planReport, expected os.FileInfo) (os.FileInfo, error) {
	info, err := os.Lstat(report.filename)
	if err != nil {
		return nil, fmt.Errorf("inspect write target %q: %w", report.filename, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to write through symbolic link %q", report.filename)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("write target %q is not a regular file", report.filename)
	}
	if expected != nil && !os.SameFile(expected, info) {
		return nil, fmt.Errorf("write target %q changed since it was read", report.filename)
	}
	content, err := os.ReadFile(report.filename)
	if err != nil {
		return nil, fmt.Errorf("re-read write target %q: %w", report.filename, err)
	}
	if !bytes.Equal(content, report.source) {
		return nil, fmt.Errorf("write target %q changed since it was read", report.filename)
	}
	finalInfo, err := os.Lstat(report.filename)
	if err != nil {
		return nil, fmt.Errorf("re-inspect write target %q: %w", report.filename, err)
	}
	if finalInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, finalInfo) {
		return nil, fmt.Errorf("write target %q changed while it was being checked", report.filename)
	}
	return info, nil
}

func applyReport(report planReport) error {
	return applyReportWithOperations(report, defaultApplyFileOperations())
}

func applyReportWithOperations(report planReport, operations applyFileOperations) error {
	if _, err := verifyApplyTarget(report, report.fileInfo); err != nil {
		return err
	}
	temporary, err := operations.createTemp(filepath.Dir(report.filename), ".tfstylus-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	temporaryName := temporary.Name()
	closed := false
	replaced := false
	defer func() {
		if !closed {
			_ = temporary.Close()
		}
		if !replaced {
			_ = os.Remove(temporaryName)
		}
	}()

	if err := operations.write(temporary, report.candidate); err != nil {
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporary.Chmod(report.fileInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		closed = true
		return fmt.Errorf("close temporary file: %w", err)
	}
	closed = true
	if _, err := verifyApplyTarget(report, report.fileInfo); err != nil {
		return err
	}
	if err := operations.rename(temporaryName, report.filename); err != nil {
		return fmt.Errorf("replace original file: %w", err)
	}
	replaced = true
	return nil
}

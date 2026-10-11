package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRunCommandAppliesRequiredVersionOrderIdempotently(t *testing.T) {
	directory := t.TempDir()
	input, err := os.ReadFile(filepath.Join("..", "..", "testdata", "required-version", "multiple-blocks.input.tf"))
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("terraform {\n  required_version = \">= 1.0\"\n  backend \"local\" {}\n  unknown_settings = local.setting\n}\n\nresource \"example\" \"kept\" {\n  value = \"unchanged\"\n}\n\nresource \"example\" \"second\" {\n  value = \"also unchanged\"\n}\n\nterraform {\n  required_version = \"~> 2.0\"\n  cloud {\n    organization = \"example\"\n  }\n}\n")
	filename := filepath.Join(directory, "main.tf")
	if err := os.WriteFile(filename, input, 0o640); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if output.Len() != 0 {
		t.Errorf("Run() output = %q, want empty output", output.String())
	}
	if got, err := os.ReadFile(filename); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("run output = %q, read error %v, want %q", got, err, want)
	}

	if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
		t.Fatalf("second Run() error = %v, want nil", err)
	}
	if got, err := os.ReadFile(filename); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("second run output = %q, read error %v, want unchanged %q", got, err, want)
	}
	if err := Run([]string{"check", "--chdir", directory}, &output); err != nil {
		t.Fatalf("Check() after run error = %v, want nil", err)
	}
}

func TestVariableCompositionPreflight(t *testing.T) {
	for _, invalid := range []string{
		"variable \"bad\" { default = {\n    a = 1\n  }\n  type = any\n}\n",
		"variable \"bad\" {\n  default = 1\n  # independent\n\n  type = number\n}\n",
		"variable \"bad\" {\n  future = true\n}\n",
	} {
		t.Run(invalid, func(t *testing.T) {
			directory := t.TempDir()
			valid := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\nvariable \"good\" {\n  default = 1\n  type = number\n}\n"
			files := map[string]string{"a-valid.tf": valid, "z-invalid.tf": invalid}
			for name, content := range files {
				writeConfig(t, filepath.Join(directory, name), content)
			}
			for _, args := range [][]string{{"check"}, {"check", "--diff"}, {"run"}} {
				err := Run(append(args, "--chdir", directory), &bytes.Buffer{})
				var exitError interface{ ExitCode() int }
				if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
					t.Fatalf("%v = %v; want exit 2", args, err)
				}
				for name, content := range files {
					if got, err := os.ReadFile(filepath.Join(directory, name)); err != nil || string(got) != content {
						t.Fatalf("%v modified %s: %s; %v", args, name, got, err)
					}
				}
			}
		})
	}
}

func TestRunCommandPreflightsAllTargetsBeforeWriting(t *testing.T) {
	directory := t.TempDir()
	input := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
	mainFilename := filepath.Join(directory, "main.tf")
	writeConfig(t, mainFilename, input)
	writeConfig(t, filepath.Join(directory, "invalid.tf"), "terraform { invalid = }\n")

	err := Run([]string{"run", "--chdir", directory}, &bytes.Buffer{})
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("Run() error = %v, want exit code 2 for syntax error", err)
	}
	if got, readErr := os.ReadFile(mainFilename); readErr != nil || string(got) != input {
		t.Errorf("run modified valid target before preflight failed: got %q, read error %v", got, readErr)
	}
}

func TestRunCommandPreflightsCloudSafetyBeforeWriting(t *testing.T) {
	directory := t.TempDir()
	validSource := "terraform {\n  cloud {\n    token = \"dummy-token\"\n    organization = \"example\"\n  }\n}\n"
	invalidSource := "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      name = \"workspace\"\n      tags = [\"app\"]\n    }\n  }\n}\n"
	validFilename := filepath.Join(directory, "a-valid.tf")
	invalidFilename := filepath.Join(directory, "z-invalid.tf")
	writeConfig(t, validFilename, validSource)
	writeConfig(t, invalidFilename, invalidSource)

	err := Run([]string{"run", "--chdir", directory}, &bytes.Buffer{})
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("Run() error = %v, want exit code 2 for unsafe cloud target", err)
	}
	for filename, want := range map[string]string{validFilename: validSource, invalidFilename: invalidSource} {
		if got, readErr := os.ReadFile(filename); readErr != nil || got == nil || string(got) != want {
			t.Errorf("run modified %s before cloud safety preflight failed: got %q, read error %v", filename, got, readErr)
		}
	}
}

func TestRunCommandAppliesAllFilesAfterPreflight(t *testing.T) {
	directory := t.TempDir()
	source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
	want := "terraform {\n  required_version = \">= 1.0\"\n  backend \"local\" {}\n}\n"
	for _, name := range []string{"a.tf", "b.tf"} {
		writeConfig(t, filepath.Join(directory, name), source)
	}
	if err := Run([]string{"run", "--chdir", directory}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	for _, name := range []string{"a.tf", "b.tf"} {
		if got, err := os.ReadFile(filepath.Join(directory, name)); err != nil || string(got) != want {
			t.Errorf("%s = %q, read error %v, want %q", name, got, err, want)
		}
	}
}

func TestRunCommandHonorsDisabledRule(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = false\n")
	writeConfig(t, filename, source)
	if err := Run([]string{"run", "--chdir", directory}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got, err := os.ReadFile(filename); err != nil || string(got) != source {
		t.Errorf("run changed file with disabled rule: got %q, error %v", got, err)
	}
}

func TestRunCommandRejectsSymlinkTargets(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(t.TempDir(), "target.tf")
	source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
	writeConfig(t, target, source)
	link := filepath.Join(directory, "linked.tf")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	err := Run([]string{"run", "--chdir", directory}, &bytes.Buffer{})
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("Run() error = %v, want exit code 2 for symlink target", err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("run replaced symlink: info = %v, error = %v", info, err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != source {
		t.Errorf("run modified symlink target: got %q, error = %v", got, err)
	}
}

func TestRunCommandPreservesPermissionMode(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
	writeConfig(t, filename, source)
	if err := os.Chmod(filename, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"run", "--chdir", directory}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o640); got != want {
		t.Errorf("permission mode = %#o, want %#o", got, want)
	}
}

func TestApplyRunReportPreservesOriginalOnFailure(t *testing.T) {
	for _, failure := range []string{"write", "rename"} {
		t.Run(failure, func(t *testing.T) {
			directory := t.TempDir()
			filename := filepath.Join(directory, "main.tf")
			source := []byte("original\n")
			if err := os.WriteFile(filename, source, 0o600); err != nil {
				t.Fatal(err)
			}
			info, err := os.Lstat(filename)
			if err != nil {
				t.Fatal(err)
			}
			tempName := ""
			operations := defaultRunFileOperations()
			operations.createTemp = func(directory, pattern string) (*os.File, error) {
				file, err := os.CreateTemp(directory, pattern)
				if err == nil {
					tempName = file.Name()
				}
				return file, err
			}
			if failure == "write" {
				operations.write = func(*os.File, []byte) error { return errors.New("injected write failure") }
			} else {
				operations.rename = func(string, string) error { return errors.New("injected rename failure") }
			}

			err = applyRunReportWithOperations(checkReport{
				filename:  filename,
				source:    source,
				candidate: []byte("candidate\n"),
				fileInfo:  info,
			}, operations)
			if err == nil {
				t.Fatal("applyRunReportWithOperations() error = nil, want injected failure")
			}
			if got, readErr := os.ReadFile(filename); readErr != nil || !bytes.Equal(got, source) {
				t.Errorf("failure modified original: got %q, read error %v", got, readErr)
			}
			if tempName != "" {
				if _, err := os.Lstat(tempName); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("temporary file still exists after failure: stat error = %v", err)
				}
			}
		})
	}
}

func TestApplyRunReportRejectsExternalModification(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := []byte("original\n")
	if err := os.WriteFile(filename, source, 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte("external change\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = applyRunReport(checkReport{
		filename:  filename,
		source:    source,
		candidate: []byte("candidate\n"),
		fileInfo:  info,
	})
	if err == nil {
		t.Fatal("applyRunReport() error = nil, want external modification error")
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != "external change\n" {
		t.Errorf("external modification was overwritten: got %q, read error %v", got, readErr)
	}
}

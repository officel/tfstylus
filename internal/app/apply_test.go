package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanAndApplyCloudWithBlankLines(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	directory := t.TempDir()
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "backend-order", "cloud-blank-lines.input.tf"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "backend-order", "cloud-blank-lines.want.tf"))
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(directory, "terraform.tf")
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "")
	writeConfig(t, filename, string(source))

	var output bytes.Buffer
	err = Run([]string{"plan", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("plan = %v, output = %s; want exit 1", err, &output)
	}
	for _, text := range []string{"backend_order", "--- a/terraform.tf", "+++ b/terraform.tf"} {
		if !strings.Contains(output.String(), text) {
			t.Fatalf("plan output = %s; want %q", &output, text)
		}
	}
	if strings.Contains(output.String(), "warning:") {
		t.Fatalf("plan warned for safe input: %s", &output)
	}
	if got, err := os.ReadFile(filename); err != nil || !bytes.Equal(got, source) {
		t.Fatalf("plan modified input: %q, %v", got, err)
	}

	output.Reset()
	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil || output.Len() != 0 {
		t.Fatalf("apply = %v, output = %s", err, &output)
	}
	if got, err := os.ReadFile(filename); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("apply result = %q, %v; want %q", got, err, want)
	}
	for _, command := range []string{"plan", "apply"} {
		output.Reset()
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("second %s = %v, output = %s; want no changes", command, err, &output)
		}
	}
}

func TestApplyCommandDiff(t *testing.T) {
	for _, flags := range [][]string{{"-d", "-r"}, {"--diff", "--recursive"}, {"--diff", "--recursive=false"}} {
		for _, filename := range []string{"main.tf", "versions.tf", "local.tf"} {
			t.Run(strings.Join(flags, " ")+"/"+filename, func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				directory := t.TempDir()
				child := filepath.Join(directory, "child")
				if err := os.Mkdir(child, 0o700); err != nil {
					t.Fatal(err)
				}
				source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
				want := "terraform {\n  required_version = \">= 1.0\"\n  backend \"local\" {}\n}\n"
				destination := filename
				switch filename {
				case "versions.tf":
					destination = "terraform.tf"
				case "local.tf":
					source, want = "locals { value = 1 }\n", "locals { value = 1 }\n"
					destination = "locals.tf"
				}
				for _, dir := range []string{directory, child} {
					writeConfig(t, filepath.Join(dir, ".tfstylus.hcl"), "")
					writeConfig(t, filepath.Join(dir, filename), source)
				}
				args := append([]string{"--chdir", directory}, flags...)
				var planOutput bytes.Buffer
				planArgs := []string{"--chdir", directory, flags[1], "plan"}
				err := Run(planArgs, &planOutput)
				var exitError interface{ ExitCode() int }
				if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
					t.Fatalf("plan = %v; want exit 1", err)
				}
				for _, dir := range []string{directory, child} {
					if got, err := os.ReadFile(filepath.Join(dir, filename)); err != nil || string(got) != source {
						t.Fatalf("plan modified input: %q, %v", got, err)
					}
				}
				var output bytes.Buffer
				if err := Run(append(args, "apply"), &output); err != nil {
					t.Fatal(err)
				}
				recursive := flags[1] != "--recursive=false"
				dirs := []string{directory}
				if recursive {
					dirs = append(dirs, child)
				}
				for _, dir := range dirs {
					if got, err := os.ReadFile(filepath.Join(dir, destination)); err != nil || string(got) != want {
						t.Fatalf("apply result = %q, %v; want %q", got, err, want)
					}
					name := displayPath(directory, filepath.Join(dir, filename))
					toName := displayPath(directory, filepath.Join(dir, destination))
					if source != want {
						for _, text := range []string{"--- a/" + name + "\n", "+++ b/" + toName + "\n", "-  required_version", "+  required_version"} {
							if !strings.Contains(output.String(), text) {
								t.Fatalf("missing %q in diff: %s", text, &output)
							}
						}
					}
					if filename != destination {
						if _, err := os.Stat(filepath.Join(dir, filename)); !errors.Is(err, os.ErrNotExist) {
							t.Fatalf("rename source still exists: %v", err)
						}
						for _, text := range []string{"rename from " + name + "\n", "rename to " + toName + "\n", "similarity index "} {
							if !strings.Contains(output.String(), text) {
								t.Fatalf("missing %q in rename diff: %s", text, &output)
							}
						}
					}
				}
				if !recursive {
					if got, err := os.ReadFile(filepath.Join(child, filename)); err != nil || string(got) != source {
						t.Fatalf("non-recursive apply modified child: %q, %v", got, err)
					}
					if strings.Contains(output.String(), "child/") {
						t.Fatalf("non-recursive diff includes child: %s", &output)
					}
				}
				if output.Len() == 0 {
					t.Fatal("apply omitted diff")
				}
				output.Reset()
				for _, command := range []string{"apply", "plan"} {
					output.Reset()
					commandArgs := args
					if command == "plan" {
						commandArgs = planArgs[:len(planArgs)-1]
					}
					if err := Run(append(commandArgs, command), &output); err != nil || output.String() != cleanCommandOutput(command) {
						t.Fatalf("second %s = %v, output = %s", command, err, &output)
					}
				}
			})
		}
	}
}

func TestApplyCommandDiffPreflight(t *testing.T) {
	directory := t.TempDir()
	source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
	filename := filepath.Join(directory, "a-valid.tf")
	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, "z-invalid.tf"), "terraform { invalid = }\n")
	var output bytes.Buffer
	err := Run([]string{"apply", "--diff", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 || output.Len() != 0 {
		t.Fatalf("apply = %v, output = %s; want exit 2 without diff", err, &output)
	}
	if got, err := os.ReadFile(filename); err != nil || string(got) != source {
		t.Fatalf("preflight modified input: %q, %v", got, err)
	}
}

func TestApplyCommandDiffOutputFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
	writeConfig(t, filename, source)
	output, err := os.CreateTemp(t.TempDir(), "closed-output")
	if err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	err = Run([]string{"apply", "--diff", "--chdir", directory}, output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 || !strings.Contains(err.Error(), "write apply diff") {
		t.Fatalf("apply = %v; want diff output error with exit 2", err)
	}
	want := "terraform {\n  required_version = \">= 1.0\"\n  backend \"local\" {}\n}\n"
	if got, err := os.ReadFile(filename); err != nil || string(got) != want {
		t.Fatalf("applied content = %q, %v; want %q", got, err, want)
	}
}

func TestApplyCommandAppliesRequiredVersionOrderIdempotently(t *testing.T) {
	directory := t.TempDir()
	input, err := os.ReadFile(filepath.Join("..", "..", "testdata", "required-version", "multiple-blocks.input.tf"))
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("terraform {\n  required_version = \">= 1.0\"\n  backend \"local\" {}\n  unknown_settings = local.setting\n}\n\nresource \"example\" \"kept\" {\n  value = \"unchanged\"\n}\n\nresource \"example\" \"second\" {\n  value = \"also unchanged\"\n}\n\nterraform {\n  required_version = \"~> 2.0\"\n  cloud {\n    organization = \"example\"\n  }\n}\n")
	filename := filepath.Join(directory, "main.tf")
	// #nosec G703 -- The destination is a fixed filename inside this test's t.TempDir, not fixture content.
	if err := os.WriteFile(filename, input, 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if output.Len() != 0 {
		t.Errorf("Run() output = %q, want empty output", output.String())
	}
	if got, err := os.ReadFile(filename); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("apply output = %q, read error %v, want %q", got, err, want)
	}

	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
		t.Fatalf("second Run() error = %v, want nil", err)
	}
	if got, err := os.ReadFile(filename); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("second apply output = %q, read error %v, want unchanged %q", got, err, want)
	}
	if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil {
		t.Fatalf("Plan() after apply error = %v, want nil", err)
	}
}

func TestVariableCompositionPreflight(t *testing.T) {
	for _, invalid := range []string{
		"variable \"bad\" { default = {\n    a = 1\n  }\n  type = any\n}\n",
		"variable \"bad\" {\n  default = 1\n  # independent\n\n  type = number\n}\n",
		"variable \"bad\" {\n  default = 1\n  # independent section\n\n  type = number\n}\n",
	} {
		t.Run(invalid, func(t *testing.T) {
			directory := t.TempDir()
			valid := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\nvariable \"good\" {\n  default = 1\n  type = number\n}\n"
			files := map[string]string{"a-valid.tf": valid, "z-invalid.tf": invalid}
			for name, content := range files {
				writeConfig(t, filepath.Join(directory, name), content)
			}
			for _, args := range [][]string{{"plan"}, {"apply"}} {
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

func TestOutputCompositionPreflight(t *testing.T) {
	directory := t.TempDir()
	valid := "terraform {\n  experiments = [\"future\"]\n  required_version = \">= 1.0\"\n}\nvariable \"good\" {\n  default = 1\n  type = number\n}\noutput \"good\" {\n  value = 1\n  description = \"good\"\n}\n"
	invalid := "terraform {\n  experiments = [\"future\"]\n  required_version = \">= 1.0\"\n}\nvariable \"bad\" {\n  default = 1\n  type = number\n}\noutput \"bad\" {\n  value = 1\n\n  # independent section\n\n  description = \"bad\"\n}\n"
	jsonSource := `{"output":{"json":{"value":1,"future":true}}}`
	files := map[string]string{"a-valid.tf": valid, "z-invalid.tf": invalid, "opaque.tf.json": jsonSource}
	for name, content := range files {
		writeConfig(t, filepath.Join(directory, name), content)
	}
	for _, args := range [][]string{{"plan"}, {"apply"}} {
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
}

func TestApplyCommandPreflightsAllTargetsBeforeWriting(t *testing.T) {
	directory := t.TempDir()
	input := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
	mainFilename := filepath.Join(directory, "main.tf")
	writeConfig(t, mainFilename, input)
	writeConfig(t, filepath.Join(directory, "invalid.tf"), "terraform { invalid = }\n")

	err := Run([]string{"apply", "--chdir", directory}, &bytes.Buffer{})
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("Run() error = %v, want exit code 2 for syntax error", err)
	}
	if got, readErr := os.ReadFile(mainFilename); readErr != nil || string(got) != input {
		t.Errorf("apply modified valid target before preflight failed: got %q, read error %v", got, readErr)
	}
}

func TestApplyCommandPreflightsCloudSafetyBeforeWriting(t *testing.T) {
	directory := t.TempDir()
	validSource := "terraform {\n  cloud {\n    token = \"dummy-token\"\n    organization = \"example\"\n  }\n}\n"
	invalidSource := "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      tags = [\"app\"]\n\n      # independent section\n\n      name = \"workspace\"\n    }\n  }\n}\n"
	validFilename := filepath.Join(directory, "a-valid.tf")
	invalidFilename := filepath.Join(directory, "z-invalid.tf")
	writeConfig(t, validFilename, validSource)
	writeConfig(t, invalidFilename, invalidSource)

	err := Run([]string{"apply", "--chdir", directory}, &bytes.Buffer{})
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("Run() error = %v, want exit code 2 for unsafe cloud target", err)
	}
	for filename, want := range map[string]string{validFilename: validSource, invalidFilename: invalidSource} {
		if got, readErr := os.ReadFile(filename); readErr != nil || got == nil || string(got) != want {
			t.Errorf("apply modified %s before cloud safety preflight failed: got %q, read error %v", filename, got, readErr)
		}
	}
}

func TestApplyCommandAppliesAllFilesAfterPreflight(t *testing.T) {
	directory := t.TempDir()
	source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
	want := "terraform {\n  required_version = \">= 1.0\"\n  backend \"local\" {}\n}\n"
	for _, name := range []string{"a.tf", "b.tf"} {
		writeConfig(t, filepath.Join(directory, name), source)
	}
	if err := Run([]string{"apply", "--chdir", directory}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	for _, name := range []string{"a.tf", "b.tf"} {
		if got, err := os.ReadFile(filepath.Join(directory, name)); err != nil || string(got) != want {
			t.Errorf("%s = %q, read error %v, want %q", name, got, err, want)
		}
	}
}

func TestApplyCommandHonorsDisabledRule(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = false\n")
	writeConfig(t, filename, source)
	if err := Run([]string{"apply", "--chdir", directory}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got, err := os.ReadFile(filename); err != nil || string(got) != source {
		t.Errorf("apply changed file with disabled rule: got %q, error %v", got, err)
	}
}

func TestApplyCommandRejectsSymlinkTargets(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(t.TempDir(), "target.tf")
	source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
	writeConfig(t, target, source)
	link := filepath.Join(directory, "linked.tf")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	err := Run([]string{"apply", "--chdir", directory}, &bytes.Buffer{})
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("Run() error = %v, want exit code 2 for symlink target", err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("apply replaced symlink: info = %v, error = %v", info, err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != source {
		t.Errorf("apply modified symlink target: got %q, error = %v", got, err)
	}
}

func TestApplyCommandPreservesPermissionMode(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
	writeConfig(t, filename, source)
	// #nosec G302 -- Verify that atomic replacement preserves the original group-readable mode.
	if err := os.Chmod(filename, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"apply", "--chdir", directory}, &bytes.Buffer{}); err != nil {
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
			operations := defaultApplyFileOperations()
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

			err = applyReportWithOperations(planReport{
				filename:  filename,
				source:    source,
				candidate: []byte("candidate\n"),
				fileInfo:  info,
			}, operations)
			if err == nil {
				t.Fatal("applyReportWithOperations() error = nil, want injected failure")
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
	err = applyReport(planReport{
		filename:  filename,
		source:    source,
		candidate: []byte("candidate\n"),
		fileInfo:  info,
	})
	if err == nil {
		t.Fatal("applyReport() error = nil, want external modification error")
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != "external change\n" {
		t.Errorf("external modification was overwritten: got %q, read error %v", got, readErr)
	}
}

package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWithVersion(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"-v"}, nil} {
		var output bytes.Buffer
		if err := RunWithVersion(args, &output, "1.2.3"); err != nil {
			t.Fatal(err)
		}
		want := "Version: 1.2.3\n"
		if len(args) == 0 {
			want += usage + "\n"
		}
		if output.String() != want {
			t.Fatalf("args %v: output = %q; want %q", args, output.String(), want)
		}
	}
}

func TestReplaceRequiredVersionCLI(t *testing.T) {
	for _, test := range []struct {
		name, filename string
		order, rename  bool
	}{
		{name: "replacement and ordering", filename: "main.tf", order: true},
		{name: "ordering disabled", filename: "main.tf"},
		{name: "rename composition", filename: "versions.tf", order: true, rename: true},
		{name: "override content", filename: "override.tf", order: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			directory := t.TempDir()
			config := "replace_required_version = \">= 1.7.0\"\n"
			if !test.order {
				config += "required_version_order = false\nbackend_order = false\n"
			}
			writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), config)
			source := "terraform {\n  backend \"local\" {}\n  # minimum\n  required_version = \">= 1.0\" # keep\n}\n"
			want := "terraform {\n  # minimum\n  required_version = \">= 1.7.0\" # keep\n  backend \"local\" {}\n}\n"
			if !test.order {
				want = "terraform {\n  backend \"local\" {}\n  # minimum\n  required_version = \">= 1.7.0\" # keep\n}\n"
			}
			filename := filepath.Join(directory, test.filename)
			// #nosec G306 -- This fixture verifies that the original group-readable mode is preserved.
			if err := os.WriteFile(filename, []byte(source), 0o640); err != nil {
				t.Fatal(err)
			}
			untouched := map[string]string{
				"opaque.tf.json":     `{"terraform":{"required_version":">= 1.0"}}`,
				"opaque.tfquery.hcl": "terraform { required_version = \">= 1.0\" }\n",
			}
			for name, content := range untouched {
				writeConfig(t, filepath.Join(directory, name), content)
			}
			{
				args := []string{"plan", "--chdir", directory}
				var output bytes.Buffer
				err := Run(args, &output)
				var exitError interface{ ExitCode() int }
				if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
					t.Fatalf("plan = %v; want exit 1", err)
				}
				if !strings.Contains(output.String(), test.filename+":4:22: terraform/replace-required-version:") {
					t.Fatalf("wrong original position: %s", output.String())
				}
				if !strings.Contains(output.String(), "->") && !strings.Contains(output.String(), "-  required_version") {
					t.Fatalf("missing replacement diff: %s", output.String())
				}
				if got, err := os.ReadFile(filename); err != nil || string(got) != source {
					t.Fatal("plan changed input")
				}
			}
			var output bytes.Buffer
			if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil || output.Len() != 0 {
				t.Fatalf("apply = %v, output = %s", err, &output)
			}
			if test.rename {
				if _, err := os.Stat(filename); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("source was not renamed")
				}
				filename = filepath.Join(directory, "terraform.tf")
			}
			if got, err := os.ReadFile(filename); err != nil || string(got) != want {
				t.Fatalf("apply = %q, %v; want %q", got, err, want)
			}
			info, err := os.Stat(filename)
			if err != nil || info.Mode().Perm() != 0o640 {
				t.Fatalf("file mode changed: %v, %v", info, err)
			}
			for name, content := range untouched {
				if got, err := os.ReadFile(filepath.Join(directory, name)); err != nil || string(got) != content {
					t.Fatalf("changed excluded content %s", name)
				}
			}
			for _, command := range []string{"apply", "plan"} {
				if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
					t.Fatalf("second %s = %v, output = %s", command, err, &output)
				}
			}
		})
	}
}

func TestReplaceRequiredVersionPreflight(t *testing.T) {
	for _, test := range []struct{ name, config, unsafe string }{
		{name: "reference", config: `replace_required_version = ">= 1.7.0"`, unsafe: `terraform { required_version = var.minimum }`},
		{name: "heredoc", config: `replace_required_version = ">= 1.7.0"`, unsafe: "terraform {\nrequired_version = <<EOT\nold\nEOT\n}\n"},
		{name: "bad constraint", config: `replace_required_version = "bad"`, unsafe: `terraform {}`},
		{name: "syntax error", config: `replace_required_version = ">= 1.7.0"`, unsafe: `terraform { required_version = }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), test.config)
			files := map[string]string{"a-valid.tf": "terraform { required_version = \"old\" }\n", "z-unsafe.tf": test.unsafe}
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
						t.Fatalf("preflight modified %s", name)
					}
				}
			}
		})
	}
}

func TestReplaceRequiredVersionRecursiveConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	directory := t.TempDir()
	child := filepath.Join(directory, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), `replace_required_version = ">= 1.7.0"`)
	writeConfig(t, filepath.Join(child, ".tfstylus.hcl"), `replace_required_version = false`)
	source := "terraform { required_version = \"old\" }\n"
	for _, path := range []string{directory, child} {
		writeConfig(t, filepath.Join(path, "main.tf"), source)
	}
	if err := Run([]string{"apply", "--chdir", directory, "--recursive"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{directory: "terraform { required_version = \">= 1.7.0\" }\n", child: source} {
		if got, err := os.ReadFile(filepath.Join(path, "main.tf")); err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v; want %q", path, got, err, want)
		}
	}
}

func TestNoArgsPrintsHelpAndVersion(t *testing.T) {
	var output bytes.Buffer

	err := Run(nil, &output)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := output.String()
	if !strings.Contains(got, "Usage:") {
		t.Errorf("Run() output = %q, want help text", got)
	}
	if !strings.Contains(got, "Version: dev") {
		t.Errorf("Run() output = %q, want development version", got)
	}
}

func TestHelpAndVersionOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "help", args: []string{"--help"}, want: usage + "\n"},
		{name: "short help", args: []string{"-h"}, want: usage + "\n"},
		{name: "version", args: []string{"--version"}, want: "Version: dev\n"},
		{name: "short version", args: []string{"-v"}, want: "Version: dev\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer

			if err := Run(test.args, &output); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := output.String(); got != test.want {
				t.Errorf("Run() output = %q, want %q", got, test.want)
			}
		})
	}
}

func TestUnknownCommand(t *testing.T) {
	var output bytes.Buffer

	err := Run([]string{"frobnicate"}, &output)
	if err == nil {
		t.Fatal("Run() error = nil, want unknown-command error")
	}
	if got, want := err.Error(), "unknown command: frobnicate"; got != want {
		t.Errorf("Run() error = %q, want %q", got, want)
	}
	if got := output.String(); got != "" {
		t.Errorf("Run() output = %q, want empty output", got)
	}
}

func TestApplyCommandWithNoTargets(t *testing.T) {
	if err := runFromDirectory(t, t.TempDir(), []string{"apply"}); err != nil {
		t.Fatalf("Run() error = %v, want nil for no targets", err)
	}
}

func TestVariableOrderCLI(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "main.tf")
	source := "variable \"example\" {\n  default = 1\n  type = number\n}\n"
	want := "variable \"example\" {\n  type = number\n  default = 1\n}\n"
	writeConfig(t, filename, source)
	{
		args := []string{"plan", "--chdir", directory}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "main.tf:1:1: variable_order:") {
			t.Fatalf("plan = %v; output = %s", err, output.String())
		}
		if !strings.Contains(output.String(), "--- a/main.tf") || !strings.Contains(output.String(), "+++ b/main.tf") {
			t.Fatalf("missing report/diff: %s", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("plan modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"apply", "apply", "plan"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "variable_order = false\n")
	for _, command := range []string{"plan", "apply"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("disabled %s = %v; %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("disabled rule changed input: %s; %v", got, err)
		}
	}
}

func TestOutputOrderCLI(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "main.tf")
	source := "output \"example\" {\n  value = \"result\"\n  description = \"Example output\"\n}\n"
	want := "output \"example\" {\n  description = \"Example output\"\n  value = \"result\"\n}\n"
	writeConfig(t, filename, source)

	{
		args := []string{"plan", "--chdir", directory}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "main.tf:1:1: output_order:") {
			t.Fatalf("plan = %v; output = %s", err, output.String())
		}
		if !strings.Contains(output.String(), "--- a/main.tf") {
			t.Fatalf("missing report/diff: %s", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("plan modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"apply", "apply", "plan"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "output_order = false\n")
	for _, command := range []string{"plan", "apply"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("disabled %s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("disabled rule changed input: %s; %v", got, err)
		}
	}
}

func TestDataOrderCLI(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  experiments = [\"future\"]\n  required_version = \">= 1.0\"\n}\ndata \"example\" \"one\" {\n  depends_on = [local.first, local.second]\n  count = 1\n}\n"
	want := "terraform {\n  required_version = \">= 1.0\"\n  experiments = [\"future\"]\n}\ndata \"example\" \"one\" {\n  count = 1\n  depends_on = [local.first, local.second]\n}\n"
	writeConfig(t, filename, source)

	{
		args := []string{"plan", "--chdir", directory}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 ||
			!strings.Contains(output.String(), "required_version_order:") || !strings.Contains(output.String(), "data_order:") {
			t.Fatalf("plan = %v; output = %s", err, output.String())
		}
		if !strings.Contains(output.String(), "--- a/main.tf") {
			t.Fatalf("missing report/diff: %s", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("plan modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"apply", "apply", "plan"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}

	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\ndata_order = false\n")
	for _, command := range []string{"plan", "apply"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("disabled %s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("disabled rule changed input: %s; %v", got, err)
		}
	}
}

func TestListOrderCLI(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "query.tfquery.hcl")
	source := "data \"example\" \"one\" {\n  depends_on = [local.first]\n  count = 1\n}\nlist \"aws_instance\" \"example\" {\n  provider = aws\n  count = 1\n}\n"
	want := "data \"example\" \"one\" {\n  depends_on = [local.first]\n  count = 1\n}\nlist \"aws_instance\" \"example\" {\n  count = 1\n  provider = aws\n}\n"
	writeConfig(t, filename, source)

	{
		args := []string{"plan", "--chdir", directory}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "query.tfquery.hcl:5:1: list_order:") {
			t.Fatalf("plan = %v; output = %s, want list_order change", err, output.String())
		}
		if !strings.Contains(output.String(), "--- a/query.tfquery.hcl") {
			t.Fatalf("missing report/diff: %s", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("plan modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"apply", "apply", "plan"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "list_order = false\n")
	for _, command := range []string{"plan", "apply"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("disabled %s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("disabled %s changed input: %s; %v", command, got, err)
		}
	}
}

func TestImportOrderCLI(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "main.tf")
	source := "import {\n  id = \"bucket-123\"\n  to = aws_s3_bucket.this\n  provider = aws.west\n  for_each = local.buckets\n}\n"
	want := "import {\n  for_each = local.buckets\n  provider = aws.west\n  id = \"bucket-123\"\n  to = aws_s3_bucket.this\n}\n"
	writeConfig(t, filename, source)

	var output bytes.Buffer
	err := Run([]string{"plan", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "import_order:") {
		t.Fatalf("plan = %v; output = %s", err, output.String())
	}
	if got, err := os.ReadFile(filename); err != nil || string(got) != source {
		t.Fatalf("plan modified input: %s; %v", got, err)
	}

	for _, command := range []string{"apply", "apply", "plan"} {
		var commandOutput bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &commandOutput); err != nil || commandOutput.String() != cleanCommandOutput(command) {
			t.Fatalf("%s = %v; output = %s", command, err, commandOutput.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}

	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "import_order = false\n")
	for _, command := range []string{"plan", "apply"} {
		var commandOutput bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &commandOutput); err != nil || commandOutput.String() != cleanCommandOutput(command) {
			t.Fatalf("disabled %s = %v; output = %s", command, err, commandOutput.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("disabled %s changed input: %s; %v", command, got, err)
		}
	}
}

func TestMovedOrderCLI(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "main.tf")
	source := "moved {\n  to = module.new\n  from = module.old\n}\n"
	want := "moved {\n  from = module.old\n  to = module.new\n}\n"
	writeConfig(t, filename, source)

	{
		args := []string{"plan", "--chdir", directory}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "moved_order:") {
			t.Fatalf("plan = %v; output = %s", err, output.String())
		}
		if !strings.Contains(output.String(), "+  from = module.old") {
			t.Fatalf("diff missing moved change: %s", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("plan modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"apply", "apply", "plan"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "moved_order = false\n")
	for _, command := range []string{"plan", "apply"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("disabled %s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("disabled %s changed input: %s; %v", command, got, err)
		}
	}
}

func TestRemovedOrderCLI(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "main.tf")
	source := "removed {\n  lifecycle { destroy = false }\n  from = module.old\n}\n"
	want := "removed {\n  from = module.old\n  lifecycle { destroy = false }\n}\n"
	writeConfig(t, filename, source)

	{
		args := []string{"plan", "--chdir", directory}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "removed_order:") {
			t.Fatalf("plan = %v; output = %s", err, output.String())
		}
		if !strings.Contains(output.String(), "+  from = module.old") {
			t.Fatalf("diff missing removed change: %s", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("plan modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"apply", "apply", "plan"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "removed_order = false\n")
	for _, input := range []string{source, "removed \"label\" {\n  lifecycle { destroy = false }\n  from = module.old\n}\n"} {
		writeConfig(t, filename, input)
		for _, command := range []string{"plan", "apply"} {
			var output bytes.Buffer
			if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
				t.Fatalf("disabled %s = %v; output = %s", command, err, output.String())
			}
			if got, err := os.ReadFile(filename); err != nil || string(got) != input {
				t.Fatalf("disabled %s changed input: %s; %v", command, got, err)
			}
		}
	}
}

func TestEphemeralOrderCLI(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "main.tf")
	source := "ephemeral \"example\" \"one\" {\n  depends_on = [local.first]\n  count = 1\n}\n"
	want := "ephemeral \"example\" \"one\" {\n  count = 1\n  depends_on = [local.first]\n}\n"
	writeConfig(t, filename, source)

	{
		args := []string{"plan", "--chdir", directory}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "ephemeral_order:") {
			t.Fatalf("plan = %v; output = %s", err, output.String())
		}
		if !strings.Contains(output.String(), "--- a/main.tf") {
			t.Fatalf("plan output = %s; want unified diff", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("plan modified input: %s; %v", got, err)
		}
	}

	for _, command := range []string{"apply", "plan"} {
		var commandOutput bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &commandOutput); err != nil || commandOutput.String() != cleanCommandOutput(command) {
			t.Fatalf("%s = %v; output = %s", command, err, commandOutput.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}

	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "ephemeral_order = false\n")
	for _, command := range []string{"plan", "apply"} {
		var commandOutput bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &commandOutput); err != nil || commandOutput.String() != cleanCommandOutput(command) {
			t.Fatalf("disabled %s = %v; output = %s", command, err, commandOutput.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("disabled %s changed input: %s; %v", command, got, err)
		}
	}
}

func TestModuleOrderCLI(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "main.tf")
	source := "module \"one\" {\n  name = var.name\n  source = \"./child\"\n}\n"
	want := "module \"one\" {\n  source = \"./child\"\n  name = var.name\n}\n"
	writeConfig(t, filename, source)
	{
		args := []string{"plan", "--chdir", directory}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "main.tf:1:1: module_order:") {
			t.Fatalf("plan = %v; output = %s", err, output.String())
		}
		if !strings.Contains(output.String(), "--- a/main.tf") {
			t.Fatalf("plan output = %s; want unified diff", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("plan modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"apply", "apply", "plan"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
	unsafe := "module \"one\" {\n  name = var.name\n\n  # section\n\n  source = \"./child\"\n}\n"
	writeConfig(t, filename, unsafe)
	for _, command := range []string{"plan", "apply"} {
		var output bytes.Buffer
		err := Run([]string{command, "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
			t.Fatalf("unsafe %s = %v; output = %s", command, err, output.String())
		}
		if command == "plan" && !strings.Contains(output.String(), "warning: skipped:") {
			t.Fatalf("plan output = %s, want warning", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != unsafe {
			t.Fatalf("unsafe %s changed input: %s; %v", command, got, err)
		}
	}
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "module_order = false\n")
	for _, input := range []string{source, unsafe} {
		writeConfig(t, filename, input)
		for _, command := range []string{"plan", "apply"} {
			var output bytes.Buffer
			if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
				t.Fatalf("disabled %s = %v; output = %s", command, err, output.String())
			}
			if got, err := os.ReadFile(filename); err != nil || string(got) != input {
				t.Fatalf("disabled %s changed input: %s; %v", command, got, err)
			}
		}
	}
}

func TestResourceOrderCLI(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "main.tf")
	source := "resource \"example\" \"one\" {\n  depends_on = [local.first]\n  count = 1\n}\n"
	want := "resource \"example\" \"one\" {\n  count = 1\n  depends_on = [local.first]\n}\n"
	writeConfig(t, filename, source)

	{
		args := []string{"plan", "--chdir", directory}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "resource_order:") {
			t.Fatalf("plan = %v; output = %s", err, output.String())
		}
		if !strings.Contains(output.String(), "--- a/main.tf") {
			t.Fatalf("plan output = %s; want unified diff", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("plan modified input: %s; %v", got, err)
		}
	}

	for _, command := range []string{"apply", "plan"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}

	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "resource_order = false\n")
	for _, command := range []string{"plan", "apply"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("disabled %s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("disabled %s changed input: %s; %v", command, got, err)
		}
	}
}

func TestActionOrderCLI(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "main.tf")
	source := "action \"example\" \"one\" {\n  config {\n    setting = true\n  }\n  provider = example.west\n  count = 1\n}\n"
	want := "action \"example\" \"one\" {\n  count = 1\n  provider = example.west\n  config {\n    setting = true\n  }\n}\n"
	writeConfig(t, filename, source)

	{
		args := []string{"plan", "--chdir", directory}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "main.tf:1:1: action_order:") {
			t.Fatalf("plan = %v; output = %s", err, output.String())
		}
		if !strings.Contains(output.String(), "--- a/main.tf") {
			t.Fatalf("missing report/diff: %s", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("plan modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"apply", "apply", "plan"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "action_order = false\n")
	for _, command := range []string{"plan", "apply"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("disabled %s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("disabled rule changed input: %s; %v", got, err)
		}
	}
}

func TestCheckOrderCLI(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "main.tf")
	source := "check \"health\" {\n  assert {\n    error_message = \"failed\"\n    condition = true\n  }\n  data \"http\" \"endpoint\" {\n    url = \"https://example.com\"\n  }\n}\n"
	want := "check \"health\" {\n  data \"http\" \"endpoint\" {\n    url = \"https://example.com\"\n  }\n  assert {\n    condition = true\n    error_message = \"failed\"\n  }\n}\n"
	writeConfig(t, filename, source)

	{
		args := []string{"plan", "--chdir", directory}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "main.tf:1:1: check_order:") {
			t.Fatalf("plan = %v; output = %s", err, output.String())
		}
		if !strings.Contains(output.String(), "--- a/main.tf") {
			t.Fatalf("missing report/diff: %s", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("plan modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"apply", "apply", "plan"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}

	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "check_order = false\n")
	for _, command := range []string{"plan", "apply"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("disabled %s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("disabled rule changed input: %s; %v", got, err)
		}
	}
}

func TestOutputOrderCompositionCLI(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  experiments = [\"future\"]\n  required_version = \">= 1.0\"\n}\nvariable \"example\" {\n  default = [2, 1]\n  type = list(number)\n}\noutput \"example\" {\n  value = [2, 1]\n  description = \"example\"\n}\n"
	want := "terraform {\n  required_version = \">= 1.0\"\n  experiments = [\"future\"]\n}\nvariable \"example\" {\n  type = list(number)\n  default = [2, 1]\n}\noutput \"example\" {\n  description = \"example\"\n  value = [2, 1]\n}\n"
	jsonFilename := filepath.Join(directory, "opaque.tf.json")
	jsonSource := `{"output":{"json":{"value":1,"future":true}}}`
	config := "required_version_order = true\nbackend_order = false\ncloud_order = false\ncloud_workspaces_order = false\nrequired_providers_order = false\nvariable_order = true\noutput_order = true\n"
	writeConfig(t, filename, source)
	writeConfig(t, jsonFilename, jsonSource)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), config)

	var output bytes.Buffer
	err := Run([]string{"plan", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("plan = %v; output = %s", err, output.String())
	}
	for _, rule := range []string{"required_version_order", "variable_order", "output_order"} {
		if !strings.Contains(output.String(), rule+":") {
			t.Fatalf("plan omitted %s: %s", rule, output.String())
		}
	}
	if !strings.Contains(output.String(), "--- a/main.tf") {
		t.Fatalf("plan omitted unified diff: %s", output.String())
	}
	if got, err := os.ReadFile(filename); err != nil || got == nil || string(got) != source {
		t.Fatalf("plan modified source: %s; %v", got, err)
	}
	if got, err := os.ReadFile(jsonFilename); err != nil || string(got) != jsonSource {
		t.Fatalf("plan modified JSON: %s; %v", got, err)
	}
	for _, command := range []string{"apply", "apply", "plan"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
		if got, err := os.ReadFile(jsonFilename); err != nil || string(got) != jsonSource {
			t.Fatalf("%s modified JSON: %s; %v", command, got, err)
		}
	}
}

func TestTerraformOrderCompositionCLI(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := `terraform {
	cloud {
		token = "dummy-token"
		workspaces {
			project = "example-project"
			name = "example"
		}
		organization = "example"
	}
	required_providers {
		aws = {
			version = "~> 5.0"
			source = "hashicorp/aws"
		}
	}
	experiments = [
		"future_alpha",
		# keep this expression comment
		"older_experiment",
	]
	provider_meta "example/first" {
		unknown_value = local.metadata # expression comment
		payload = <<-EOT
			provider-defined data
		EOT
	}
	required_version = ">= 1.0"
}
`
	want := `terraform {
	required_version = ">= 1.0"
	cloud {
		organization = "example"
		token = "dummy-token"
		workspaces {
			name = "example"
			project = "example-project"
		}
	}
	required_providers {
		aws = {
			source = "hashicorp/aws"
			version = "~> 5.0"
		}
	}
	experiments = [
		"future_alpha",
		# keep this expression comment
		"older_experiment",
	]
	provider_meta "example/first" {
		unknown_value = local.metadata # expression comment
		payload = <<-EOT
			provider-defined data
		EOT
	}
}
`
	writeConfig(t, filename, source)
	var output bytes.Buffer
	var exitError interface{ ExitCode() int }

	err := Run([]string{"plan", "--chdir", directory}, &output)
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("plan error = %v, want exit code 1 for composed ordering", err)
	}
	planOutput := output.String()
	for _, rule := range []string{"required_version_order", "cloud_order", "cloud_workspaces_order", "required_providers_order"} {
		if !strings.Contains(planOutput, rule) {
			t.Errorf("plan output = %q, want rule %q", planOutput, rule)
		}
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Fatalf("plan changed input: got %q, read error %v", got, readErr)
	}

	output.Reset()
	err = Run([]string{"plan", "--chdir", directory}, &output)
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("plan error = %v, want exit code 1 for composed ordering", err)
	}
	if got := output.String(); !strings.HasPrefix(got, planOutput) || !strings.Contains(got, "--- ") || !strings.Contains(got, "+++ ") {
		t.Errorf("plan output = %q, want the same change report followed by a unified diff", got)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Fatalf("plan changed input: got %q, read error %v", got, readErr)
	}

	output.Reset()
	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
		t.Fatalf("apply error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("apply output = %q, read error %v, want %q", got, readErr, want)
	}
	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
		t.Fatalf("second apply error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("second apply changed output: got %q, read error %v", got, readErr)
	}
}

func TestCloudAttributeOrderCLI(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  cloud {\n    token = \"dummy-token\"\n    hostname = \"example.test\"\n    workspaces { name = \"example\" }\n    organization = \"example\"\n  }\n}\n"
	want := "terraform {\n  cloud {\n    organization = \"example\"\n    hostname = \"example.test\"\n    token = \"dummy-token\"\n    workspaces { name = \"example\" }\n  }\n}\n"
	writeConfig(t, filename, source)
	var output bytes.Buffer

	err := Run([]string{"plan", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("plan error = %v, want exit code 1 for pending cloud order", err)
	}
	if !strings.Contains(output.String(), "cloud_order") {
		t.Errorf("plan output = %q, want cloud_order report", output.String())
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("plan changed input: got %q, read error %v", got, readErr)
	}
	output.Reset()
	err = Run([]string{"plan", "--chdir", directory}, &output)
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("plan error = %v, want exit code 1", err)
	}
	if !strings.Contains(output.String(), "--- ") || !strings.Contains(output.String(), "+++ ") {
		t.Errorf("plan output = %q, want unified diff headers", output.String())
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("plan changed input: got %q, read error %v", got, readErr)
	}
	output.Reset()
	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
		t.Fatalf("apply error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("apply output = %q, read error %v, want %q", got, readErr, want)
	}
	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
		t.Fatalf("second apply error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("second apply changed output: got %q, read error %v", got, readErr)
	}
}

func TestProviderMetaPreservationCLI(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := `terraform {
  provider_meta "example/first" {
    unknown_value = local.metadata # expression comment
    payload = <<-EOT
      provider-defined data
    EOT
  }
  backend "local" {}
  provider_meta "example/second" {
    unknown_attribute = { nested = ["value"] }
  }
  provider_meta "example/first" {
    unknown_value = "repeated label"
  }
  required_version = ">= 1.0"
}
`
	want := `terraform {
  required_version = ">= 1.0"
  backend "local" {}
  provider_meta "example/first" {
    unknown_value = local.metadata # expression comment
    payload = <<-EOT
      provider-defined data
    EOT
  }
  provider_meta "example/second" {
    unknown_attribute = { nested = ["value"] }
  }
  provider_meta "example/first" {
    unknown_value = "repeated label"
  }
}
`
	writeConfig(t, filename, source)
	var output bytes.Buffer
	var exitError interface{ ExitCode() int }

	for _, args := range [][]string{
		{"plan", "--chdir", directory},
		{"plan", "--chdir", directory},
	} {
		err := Run(args, &output)
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("Run(%v) error = %v, want exit code 1 for pending terraform ordering", args, err)
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Fatalf("Run(%v) changed input: got %q, read error %v", args, got, readErr)
		}
		output.Reset()
	}

	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
		t.Fatalf("apply error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("apply output = %q, read error %v, want %q", got, readErr, want)
	}
	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
		t.Fatalf("second apply error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("second apply changed output: got %q, read error %v", got, readErr)
	}
}

func TestExperimentsPreservationCLI(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := `terraform {
  experiments = [
    "future_alpha",
    # keep this expression comment
    "older_experiment",
  ]
  backend "local" {}
  required_version = ">= 1.0"
}
`
	want := `terraform {
  required_version = ">= 1.0"
  backend "local" {}
  experiments = [
    "future_alpha",
    # keep this expression comment
    "older_experiment",
  ]
}
`
	writeConfig(t, filename, source)
	var output bytes.Buffer
	var exitError interface{ ExitCode() int }

	for _, args := range [][]string{
		{"plan", "--chdir", directory},
		{"plan", "--chdir", directory},
	} {
		err := Run(args, &output)
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("Run(%v) error = %v, want exit code 1 for pending terraform ordering", args, err)
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Fatalf("Run(%v) changed input: got %q, read error %v", args, got, readErr)
		}
		output.Reset()
	}

	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
		t.Fatalf("apply error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("apply output = %q, read error %v, want %q", got, readErr, want)
	}
	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
		t.Fatalf("second apply error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("second apply changed output: got %q, read error %v", got, readErr)
	}
}

func TestCloudAttributeOrderAllowsOmittedToken(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  cloud {\n    workspaces { name = \"example\" }\n    hostname = \"example.test\"\n    organization = \"example\"\n  }\n}\n"
	want := "terraform {\n  cloud {\n    organization = \"example\"\n    hostname = \"example.test\"\n    workspaces { name = \"example\" }\n  }\n}\n"
	writeConfig(t, filename, source)
	var output bytes.Buffer

	err := Run([]string{"plan", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("plan error = %v, want exit code 1 with omitted token", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("plan changed input: got %q, read error %v", got, readErr)
	}
	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
		t.Fatalf("apply error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("apply output = %q, read error %v, want %q", got, readErr, want)
	}
}

func TestCloudAttributeOrderCanBeDisabled(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  cloud {\n    token = \"dummy-token\"\n    organization = \"example\"\n  }\n}\n"
	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = false\ncloud_order = false\nrequired_providers_order = false\n")
	var output bytes.Buffer

	if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil {
		t.Fatalf("plan error = %v, want nil with cloud_order disabled", err)
	}
	if output.String() != "No changes.\n" {
		t.Errorf("plan output = %q, want No changes. with cloud_order disabled", output.String())
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("disabled rule changed input: got %q, read error %v", got, readErr)
	}
}

func TestCloudAttributeOrderIsIndependentOfBackendOrder(t *testing.T) {
	directory := t.TempDir()
	source := "terraform {\n  cloud {\n    token = \"dummy-token\"\n    organization = \"example\"\n  }\n}\n"
	writeConfig(t, filepath.Join(directory, "main.tf"), source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = false\ncloud_order = true\nrequired_providers_order = false\n")
	var output bytes.Buffer

	err := Run([]string{"plan", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("plan error = %v, want exit code 1 with backend_order disabled", err)
	}
	if !strings.Contains(output.String(), "cloud_order") {
		t.Errorf("plan output = %q, want cloud_order report", output.String())
	}
}

func TestCloudAttributeOrderPreservesComments(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  cloud {\n    # token owner\n    token = \"dummy-token\"\n    hostname = \"example.test\" # hostname owner\n    # workspace owner\n    workspaces {\n      name = \"example\"\n      # nested comment\n    }\n    organization = \"example\" # organization owner\n  }\n}\n"
	want := "terraform {\n  cloud {\n    organization = \"example\" # organization owner\n    hostname = \"example.test\" # hostname owner\n    # token owner\n    token = \"dummy-token\"\n    # workspace owner\n    workspaces {\n      name = \"example\"\n      # nested comment\n    }\n  }\n}\n"
	writeConfig(t, filename, source)
	var output bytes.Buffer

	err := Run([]string{"plan", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("plan error = %v, want exit code 1 for cloud order with attached comments", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("plan changed input: got %q, read error %v", got, readErr)
	}
	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
		t.Fatalf("apply error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("apply output = %q, read error %v, want %q", got, readErr, want)
	}
}

func TestCloudAttributeOrderSortsMultipleWorkspacesBlocksStably(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  cloud {\n    workspaces { name = \"first\" }\n    organization = \"example\"\n    workspaces { name = \"second\" }\n  }\n}\n"
	writeConfig(t, filename, source)
	var output bytes.Buffer

	err := Run([]string{"plan", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("plan error = %v, want exit code 1 for ordering changes", err)
	}
	if strings.Contains(output.String(), "warning: skipped") {
		t.Errorf("plan output = %q, unexpected semantic warning", output.String())
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("plan changed ambiguous input: got %q, read error %v", got, readErr)
	}
}

func TestCloudAttributeOrderSafetyCases(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantExit    int
		wantWarning string
	}{
		{name: "empty cloud", source: "terraform {\n  cloud {}\n}\n"},
		{name: "partial cloud", source: "terraform {\n  cloud {\n    hostname = \"example.test\"\n  }\n}\n"},
		{
			name:     "unknown element",
			source:   "terraform {\n  cloud {\n    token = \"dummy-token\"\n    future = \"keep\"\n    organization = \"example\"\n  }\n}\n",
			wantExit: 1,
		},
		{
			name:     "singular workspace typo",
			source:   "terraform {\n  cloud {\n    token = \"dummy-token\"\n    workspace { name = \"example\" }\n    organization = \"example\"\n  }\n}\n",
			wantExit: 1,
		},
		{
			name:        "independent comment",
			source:      "terraform {\n  cloud {\n    token = \"dummy-token\"\n    # independent section\n\n    organization = \"example\"\n  }\n}\n",
			wantExit:    2,
			wantWarning: "ambiguous comment attachment",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			filename := filepath.Join(directory, "main.tf")
			writeConfig(t, filename, test.source)
			var output bytes.Buffer

			err := Run([]string{"plan", "--chdir", directory}, &output)
			if test.wantExit == 0 {
				if err != nil {
					t.Fatalf("plan error = %v, want nil", err)
				}
			} else {
				var exitError interface{ ExitCode() int }
				if !errors.As(err, &exitError) || exitError.ExitCode() != test.wantExit {
					t.Fatalf("plan error = %v, want exit code %d", err, test.wantExit)
				}
				if !strings.Contains(output.String(), test.wantWarning) {
					t.Errorf("plan output = %q, want warning containing %q", output.String(), test.wantWarning)
				}
			}
			if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != test.source {
				t.Errorf("plan changed input: got %q, read error %v", got, readErr)
			}
		})
	}
}

func TestCloudWorkspacesOrderCLI(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		want       string
		wantChange bool
	}{
		{
			name:       "name mode",
			wantChange: true,
			source:     "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      project = \"project\"\n      name = \"workspace\"\n    }\n  }\n}\n",
			want:       "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      name = \"workspace\"\n      project = \"project\"\n    }\n  }\n}\n",
		},
		{
			name:   "name only",
			source: "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      name = \"workspace\"\n    }\n  }\n}\n",
			want:   "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      name = \"workspace\"\n    }\n  }\n}\n",
		},
		{
			name:       "tags map mode",
			wantChange: true,
			source:     "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      # tag map comment\n      tags = {\n        z = \"last\" # retain inline comment\n        a = \"first\"\n      }\n      project = \"project\"\n    }\n  }\n}\n",
			want:       "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      project = \"project\"\n      # tag map comment\n      tags = {\n        z = \"last\" # retain inline comment\n        a = \"first\"\n      }\n    }\n  }\n}\n",
		},
		{
			name:       "tags list mode",
			wantChange: true,
			source:     "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      tags = [\"z\", \"a\"]\n      project = \"project\"\n    }\n  }\n}\n",
			want:       "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      project = \"project\"\n      tags = [\"z\", \"a\"]\n    }\n  }\n}\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			filename := filepath.Join(directory, "main.tf")
			writeConfig(t, filename, test.source)
			var output bytes.Buffer

			err := Run([]string{"plan", "--chdir", directory}, &output)
			var exitError interface{ ExitCode() int }
			if test.wantChange {
				if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
					t.Fatalf("plan error = %v, want exit code 1 for %s", err, test.name)
				}
				if !strings.Contains(output.String(), "cloud_workspaces_order") {
					t.Errorf("plan output = %q, want cloud_workspaces_order report", output.String())
				}
			} else if err != nil || output.String() != "No changes.\n" {
				t.Fatalf("plan result = (%v, %q), want clean success for %s", err, output.String(), test.name)
			}
			if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != test.source {
				t.Errorf("plan changed input: got %q, read error %v", got, readErr)
			}
			output.Reset()
			err = Run([]string{"plan", "--chdir", directory}, &output)
			if test.wantChange {
				if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
					t.Fatalf("plan error = %v, want exit code 1", err)
				}
				if !strings.Contains(output.String(), "--- ") || !strings.Contains(output.String(), "+++ ") {
					t.Errorf("plan output = %q, want unified diff headers", output.String())
				}
			} else if err != nil || output.String() != "No changes.\n" {
				t.Fatalf("plan result = (%v, %q), want clean success", err, output.String())
			}
			if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != test.source {
				t.Errorf("plan changed input: got %q, read error %v", got, readErr)
			}
			output.Reset()
			if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
				t.Fatalf("apply error = %v, want nil", err)
			}
			if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != test.want {
				t.Fatalf("apply output = %q, read error %v, want %q", got, readErr, test.want)
			}
			if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
				t.Fatalf("second apply error = %v, want nil", err)
			}
			if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != test.want {
				t.Fatalf("second apply changed output: got %q, read error %v", got, readErr)
			}
		})
	}
}

func TestCloudWorkspacesOrderSettings(t *testing.T) {
	t.Run("disabled independently from cloud order", func(t *testing.T) {
		directory := t.TempDir()
		source := "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      project = \"project\"\n      name = \"workspace\"\n    }\n  }\n}\n"
		filename := filepath.Join(directory, "main.tf")
		writeConfig(t, filename, source)
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = false\ncloud_order = true\ncloud_workspaces_order = false\nrequired_providers_order = false\n")
		var output bytes.Buffer

		if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil {
			t.Fatalf("plan error = %v, want nil with cloud_workspaces_order disabled", err)
		}
		if output.String() != "No changes.\n" {
			t.Errorf("plan output = %q, want No changes.", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("disabled rule changed input: got %q, read error %v", got, readErr)
		}
	})

	t.Run("enabled independently from cloud order", func(t *testing.T) {
		directory := t.TempDir()
		source := "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      project = \"project\"\n      name = \"workspace\"\n    }\n  }\n}\n"
		writeConfig(t, filepath.Join(directory, "main.tf"), source)
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = false\ncloud_order = false\ncloud_workspaces_order = true\nrequired_providers_order = false\n")
		var output bytes.Buffer

		err := Run([]string{"plan", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("plan error = %v, want exit code 1 with cloud_order disabled", err)
		}
		if !strings.Contains(output.String(), "cloud_workspaces_order") {
			t.Errorf("plan output = %q, want cloud_workspaces_order report", output.String())
		}
	})

	t.Run("disabled rule suppresses name-tags diagnostics", func(t *testing.T) {
		directory := t.TempDir()
		source := "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      name = \"workspace\"\n      tags = [\"app\"]\n    }\n  }\n}\n"
		writeConfig(t, filepath.Join(directory, "main.tf"), source)
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = false\ncloud_order = false\ncloud_workspaces_order = false\nrequired_providers_order = false\n")
		var output bytes.Buffer

		if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil {
			t.Fatalf("plan error = %v, want nil with rules disabled", err)
		}
		if output.String() != "No changes.\n" {
			t.Errorf("plan output = %q, want No changes. with rules disabled", output.String())
		}
	})
}

func TestCloudWorkspacesOrderSafetyCases(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantExit    int
		wantWarning string
	}{
		{name: "empty body", source: "terraform {\n  cloud {\n    workspaces {}\n  }\n}\n"},
		{name: "project only", source: "terraform {\n  cloud {\n    workspaces {\n      project = \"project\"\n    }\n  }\n}\n"},
		{
			name:     "null name and tags remain selectable externally",
			source:   "terraform {\n  cloud {\n    workspaces {\n      tags = null\n      name = null\n      project = \"project\"\n    }\n  }\n}\n",
			wantExit: 1,
		},
		{
			name:     "explicit name and tags conflict",
			source:   "terraform {\n  cloud {\n    workspaces {\n      tags = [\"app\"]\n      project = \"project\"\n      name = \"workspace\"\n    }\n  }\n}\n",
			wantExit: 1,
		},
		{
			name:     "aligned explicit name and tags conflict",
			source:   "terraform {\n  cloud {\n    workspaces {\n      name = \"workspace\"\n      tags = [\"app\"]\n    }\n  }\n}\n",
			wantExit: 0,
		},
		{
			name:     "empty name and empty tags are non-null conflict",
			source:   "terraform {\n  cloud {\n    workspaces {\n      tags = []\n      project = \"project\"\n      name = \"\"\n    }\n  }\n}\n",
			wantExit: 1,
		},
		{
			name:     "remote backend prefix is unknown",
			source:   "terraform {\n  cloud {\n    workspaces {\n      tags = [\"app\"]\n      prefix = \"app-\"\n      project = \"project\"\n    }\n  }\n}\n",
			wantExit: 1,
		},
		{
			name:        "independent comment",
			source:      "terraform {\n  cloud {\n    workspaces {\n      tags = [\"app\"]\n      # independent section\n\n      project = \"project\"\n    }\n  }\n}\n",
			wantExit:    2,
			wantWarning: "ambiguous comment attachment",
		},
		{
			name:     "multiple workspaces blocks",
			source:   "terraform {\n  cloud {\n    workspaces {\n      project = \"project\"\n      name = \"workspace\"\n    }\n    workspaces {\n      name = \"other\"\n    }\n  }\n}\n",
			wantExit: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			filename := filepath.Join(directory, "main.tf")
			writeConfig(t, filename, test.source)
			var output bytes.Buffer

			err := Run([]string{"plan", "--chdir", directory}, &output)
			switch test.wantExit {
			case 0:
				if err != nil {
					t.Fatalf("plan error = %v, want nil", err)
				}
			case 1:
				var exitError interface{ ExitCode() int }
				if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
					t.Fatalf("plan error = %v, want exit code 1", err)
				}
			default:
				var exitError interface{ ExitCode() int }
				if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
					t.Fatalf("plan error = %v, want exit code 2", err)
				}
				if !strings.Contains(output.String(), test.wantWarning) {
					t.Errorf("plan output = %q, want warning containing %q", output.String(), test.wantWarning)
				}
			}
			if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != test.source {
				t.Errorf("plan changed input: got %q, read error %v", got, readErr)
			}
		})
	}
}

func TestCloudWorkspacesOrderAllowsDuplicateAlignedBlocks(t *testing.T) {
	directory := t.TempDir()
	source := "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      name = \"first\"\n    }\n    workspaces {\n      name = \"second\"\n    }\n  }\n}\n"
	writeConfig(t, filepath.Join(directory, "main.tf"), source)
	var output bytes.Buffer

	err := Run([]string{"plan", "--chdir", directory}, &output)
	if err != nil || output.String() != "No changes.\n" {
		t.Fatalf("plan = %v; output = %q, want no value validation diagnostic", err, output.String())
	}
	if got, readErr := os.ReadFile(filepath.Join(directory, "main.tf")); readErr != nil || string(got) != source {
		t.Errorf("plan changed duplicate input: got %q, read error %v", got, readErr)
	}
}

func TestCloudOrderingDoesNotDependOnEnvironment(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  cloud {\n    workspaces {\n      tags = [\"app\"]\n    }\n    hostname = \"explicit.example\"\n  }\n}\n"
	writeConfig(t, filename, source)
	environment := map[string][2]string{
		"TF_CLOUD_ORGANIZATION": {"first-org", "second-org"},
		"TF_CLOUD_HOSTNAME":     {"first.example", "second.example"},
		"TF_CLOUD_PROJECT":      {"first-project", "second-project"},
		"TF_WORKSPACE":          {"first-workspace", "second-workspace"},
	}
	for key, values := range environment {
		t.Setenv(key, values[0])
	}
	var firstOutput bytes.Buffer
	firstErr := Run([]string{"plan", "--chdir", directory}, &firstOutput)
	for key, values := range environment {
		t.Setenv(key, values[1])
	}
	var secondOutput bytes.Buffer
	secondErr := Run([]string{"plan", "--chdir", directory}, &secondOutput)
	var exitError interface{ ExitCode() int }
	if !errors.As(firstErr, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("first plan error = %v, want exit code 1", firstErr)
	}
	if !errors.As(secondErr, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("second plan error = %v, want exit code 1", secondErr)
	}
	if firstOutput.String() != secondOutput.String() {
		t.Errorf("plan output changed with environment: first %q, second %q", firstOutput.String(), secondOutput.String())
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("plan changed partial cloud input: got %q, read error %v", got, readErr)
	}
}

func TestRequiredProvidersOrderCLI(t *testing.T) {
	t.Run("plan and diff are non-destructive and apply is idempotent", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  required_providers {\n    aws = {\n      version = \"~> 5.0\"\n      source = \"hashicorp/aws\"\n    }\n  }\n}\n"
		want := "terraform {\n  required_providers {\n    aws = {\n      source = \"hashicorp/aws\"\n      version = \"~> 5.0\"\n    }\n  }\n}\n"
		writeConfig(t, filename, source)
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = false\nrequired_providers_order = true\n")
		var output bytes.Buffer

		err := Run([]string{"plan", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("plan error = %v, want exit code 1", err)
		}
		if !strings.Contains(output.String(), "required_providers_order") {
			t.Errorf("plan output = %q, want required_providers_order report", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || got == nil || string(got) != source {
			t.Errorf("plan changed input: got %q, read error %v", got, readErr)
		}

		output.Reset()
		err = Run([]string{"plan", "--chdir", directory}, &output)
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("plan error = %v, want exit code 1", err)
		}
		if !strings.Contains(output.String(), "--- ") || !strings.Contains(output.String(), "+++ ") {
			t.Errorf("plan output = %q, want unified diff headers", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("plan changed input: got %q, read error %v", got, readErr)
		}

		output.Reset()
		if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
			t.Fatalf("apply error = %v, want nil", err)
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
			t.Fatalf("apply output = %q, read error %v, want %q", got, readErr, want)
		}
		if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
			t.Fatalf("second apply error = %v, want nil", err)
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
			t.Fatalf("second apply changed output: got %q, read error %v", got, readErr)
		}
	})

	t.Run("disabled rule leaves input unchanged", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  required_providers {\n    aws = {\n      version = \"~> 5.0\"\n      source = \"hashicorp/aws\"\n    }\n  }\n}\n"
		writeConfig(t, filename, source)
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_providers_order = false\n")
		var output bytes.Buffer

		if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil {
			t.Fatalf("plan with disabled rule error = %v, want nil", err)
		}
		if output.String() != "No changes.\n" {
			t.Errorf("plan with disabled rule output = %q, want empty", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("disabled rule changed input: got %q, read error %v", got, readErr)
		}
	})

	t.Run("unsafe provider object is reported without writing", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  required_providers {\n    aws = {\n      version = \"~> 5.0\"\n      source = \"hashicorp/aws\"\n      other = true\n    }\n  }\n}\n"
		writeConfig(t, filename, source)
		var output bytes.Buffer

		err := Run([]string{"plan", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
			t.Fatalf("plan error = %v, want exit code 2", err)
		}
		if !strings.Contains(output.String(), "provider requirement") || !strings.Contains(output.String(), "warning") {
			t.Errorf("plan output = %q, want a provider requirement warning", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("plan changed unsafe input: got %q, read error %v", got, readErr)
		}
	})
}

func TestPlanVariableFileRename(t *testing.T) {
	directory := t.TempDir()
	source := "variable \"name\" {\n  default = \"example\"\n}\n"
	filename := filepath.Join(directory, "variable.tf")
	writeConfig(t, filename, source)
	var output bytes.Buffer
	err := Run([]string{"plan", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("Run() error = %v, want exit code 1 for pending rename", err)
	}
	want := "variable.tf: rename to variables.tf [file-name/variables] variable.tf contains a variable block and variables.tf does not exist\n"
	want += "diff --git a/variable.tf b/variables.tf\nsimilarity index 100%\nrename from variable.tf\nrename to variables.tf\n"
	if output.String() != want {
		t.Errorf("Run() output = %q, want %q", output.String(), want)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("plan changed input: got %q, read error %v", got, readErr)
	}
	if _, statErr := os.Lstat(filepath.Join(directory, "variables.tf")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("plan created variables.tf: %v", statErr)
	}
}

func TestPlanOutputFileRename(t *testing.T) {
	directory := t.TempDir()
	source := "output \"name\" {\n  value = \"example\"\n}\n"
	filename := filepath.Join(directory, "output.tf")
	writeConfig(t, filename, source)
	var output bytes.Buffer
	err := Run([]string{"plan", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("Run() error = %v, want exit code 1 for pending rename", err)
	}
	want := "output.tf: rename to outputs.tf [file-name/outputs] output.tf contains an output block and outputs.tf does not exist\n"
	want += "diff --git a/output.tf b/outputs.tf\nsimilarity index 100%\nrename from output.tf\nrename to outputs.tf\n"
	if output.String() != want {
		t.Errorf("Run() output = %q, want %q", output.String(), want)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("plan changed input: got %q, read error %v", got, readErr)
	}
	if _, statErr := os.Lstat(filepath.Join(directory, "outputs.tf")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("plan created outputs.tf: %v", statErr)
	}
}

func TestPlanTerraformFileRenameSelectsFirstCandidate(t *testing.T) {
	directory := t.TempDir()
	source := "terraform {\n  required_version = \">= 1.0\"\n}\n"
	writeConfig(t, filepath.Join(directory, "backend.tf"), source)
	writeConfig(t, filepath.Join(directory, "providers.tf"), source)
	var output bytes.Buffer

	err := Run([]string{"plan", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("Run() error = %v, want exit code 1 for pending rename", err)
	}
	want := "backend.tf: rename to terraform.tf [file-name/terraform] backend.tf contains a terraform block and terraform.tf does not exist\n"
	want += "diff --git a/backend.tf b/terraform.tf\nsimilarity index 100%\nrename from backend.tf\nrename to terraform.tf\n"
	if output.String() != want {
		t.Errorf("Run() output = %q, want %q", output.String(), want)
	}
}

func TestPlanTerraformFileRenameSkipsNonqualifyingCandidates(t *testing.T) {
	terraformSource := "terraform {\n  required_version = \">= 1.0\"\n}\n"
	noTerraformSource := "locals {\n  keep = true\n}\n"
	for _, test := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name:  "skip backend without terraform block",
			files: map[string]string{"backend.tf": noTerraformSource, "providers.tf": terraformSource},
			want:  "providers.tf: rename to terraform.tf [file-name/terraform] providers.tf contains a terraform block and terraform.tf does not exist\ndiff --git a/providers.tf b/terraform.tf\nsimilarity index 100%\nrename from providers.tf\nrename to terraform.tf\n",
		},
		{
			name:  "skip missing backend and provider without terraform block",
			files: map[string]string{"providers.tf": noTerraformSource, "versions.tf": terraformSource},
			want:  "versions.tf: rename to terraform.tf [file-name/terraform] versions.tf contains a terraform block and terraform.tf does not exist\ndiff --git a/versions.tf b/terraform.tf\nsimilarity index 100%\nrename from versions.tf\nrename to terraform.tf\n",
		},
		{
			name:  "no qualifying candidate",
			files: map[string]string{"backend.tf": noTerraformSource, "versions.tf": noTerraformSource},
		},
		{
			name:  "no candidate files",
			files: map[string]string{},
		},
		{
			name: "terraform block is nested or commented",
			files: map[string]string{
				"backend.tf":   "resource \"example\" \"nested\" {\n  terraform {}\n}\n",
				"providers.tf": "# terraform {}\n",
			},
		},
		{
			name:  "canonical destination exists",
			files: map[string]string{"terraform.tf": terraformSource, "backend.tf": terraformSource},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			for filename, source := range test.files {
				writeConfig(t, filepath.Join(directory, filename), source)
			}
			var output bytes.Buffer
			err := Run([]string{"plan", "--chdir", directory}, &output)
			if test.want == "" {
				if err != nil || output.String() != "No changes.\n" {
					t.Fatalf("plan = %v, output %q, want clean success", err, output.String())
				}
			} else {
				var exitError interface{ ExitCode() int }
				if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || output.String() != test.want {
					t.Fatalf("plan = %v, output %q, want %q", err, output.String(), test.want)
				}
			}
			for filename, source := range test.files {
				if got, readErr := os.ReadFile(filepath.Join(directory, filename)); readErr != nil || string(got) != source {
					t.Errorf("plan changed %s: got %q, read error %v", filename, got, readErr)
				}
			}
		})
	}
}

func TestOutputFileRenameSafety(t *testing.T) {
	testNamedFileRenameSafety(t, "output", "# output comment\noutput \"name\" {\n  value = join(\"-\", [\"example\", \"kept\"]) # value comment\n}\n\nlocals { preserved = true }\n")
}

func TestVariableFileRenameSafety(t *testing.T) {
	testNamedFileRenameSafety(t, "variable", "# variable comment\nvariable \"name\" {\n  default = [\"example\", \"kept\"] # default comment\n}\n\nlocals { preserved = true }\n")
}

func TestTerraformFileRenameSafety(t *testing.T) {
	source := "# terraform comment\nterraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\" # version comment\n}\n\nprovider \"aws\" {\n  region = \"us-east-1\"\n}\n"
	want := "# terraform comment\nterraform {\n  required_version = \">= 1.0\" # version comment\n  backend \"local\" {}\n}\n\nprovider \"aws\" {\n  region = \"us-east-1\"\n}\n"

	t.Run("plan diff is non destructive", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "backend.tf")
		writeConfig(t, filename, source)
		var output bytes.Buffer

		err := Run([]string{"plan", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("plan error = %v, want exit code 1", err)
		}
		for _, expected := range []string{
			"backend.tf: rename to terraform.tf [file-name/terraform]",
			"diff --git a/backend.tf b/terraform.tf\n",
			"rename from backend.tf\nrename to terraform.tf\n",
			"--- a/backend.tf\n+++ b/terraform.tf\n",
		} {
			if !strings.Contains(output.String(), expected) {
				t.Errorf("plan output %q does not contain %q", output.String(), expected)
			}
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Errorf("plan changed source: got %q, read error %v", got, err)
		}
		if _, err := os.Lstat(filepath.Join(directory, "terraform.tf")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("plan created terraform.tf: %v", err)
		}
	})

	t.Run("apply preserves contents and mode and is idempotent", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "backend.tf")
		destination := filepath.Join(directory, "terraform.tf")
		writeConfig(t, filename, source)
		// #nosec G302 -- Verify that renaming preserves the original group-readable mode.
		if err := os.Chmod(filename, 0o640); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer

		if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
			t.Fatalf("apply error = %v", err)
		}
		if got, err := os.ReadFile(destination); err != nil || string(got) != want {
			t.Fatalf("terraform.tf = %q, error %v, want %q", got, err, want)
		}
		if _, err := os.Lstat(filename); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("backend.tf remains: %v", err)
		}
		if info, err := os.Stat(destination); err != nil || info.Mode().Perm() != 0o640 {
			t.Fatalf("terraform.tf mode = %v, error %v, want 0640", info.Mode().Perm(), err)
		}
		if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("second apply = %v, output %q", err, output.String())
		}
		if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil || output.String() != "No changes.\n" {
			t.Fatalf("second plan = %v, output %q", err, output.String())
		}
	})

	t.Run("disabled setting is independent", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "backend.tf")
		writeConfig(t, filename, source)
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "terraform_file_rename = false\noutput_order = false\n")
		var output bytes.Buffer

		err := Run([]string{"plan", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "required_version_order") || strings.Contains(output.String(), "[file-name/terraform]") {
			t.Fatalf("plan = %v, output %q, want ordering report without rename", err, output.String())
		}
		output.Reset()
		if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("apply = %v, output %q, want clean success", err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Errorf("apply result = %q, error %v, want ordering applied without rename", got, err)
		}
		if _, err := os.Lstat(filepath.Join(directory, "terraform.tf")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("disabled rule created terraform.tf: %v", err)
		}
	})

	t.Run("symbolic link source is rejected", func(t *testing.T) {
		directory := t.TempDir()
		target := filepath.Join(directory, "real.tf.txt")
		filename := filepath.Join(directory, "backend.tf")
		writeConfig(t, target, source)
		if err := os.Symlink(filepath.Base(target), filename); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		for _, command := range []string{"plan", "apply"} {
			var output bytes.Buffer
			err := Run([]string{command, "--chdir", directory}, &output)
			var exitError interface{ ExitCode() int }
			if !errors.As(err, &exitError) || exitError.ExitCode() != 2 || !strings.Contains(err.Error(), "symbolic link") {
				t.Fatalf("%s error = %v, want symlink rejection", command, err)
			}
		}
		if got, err := os.ReadFile(target); err != nil || string(got) != source {
			t.Errorf("symlink target changed: got %q, error %v", got, err)
		}
		if _, err := os.Lstat(filepath.Join(directory, "terraform.tf")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("symlink source was renamed: %v", err)
		}
	})

	t.Run("parse failure prevents rename", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "backend.tf")
		writeConfig(t, filename, source)
		writeConfig(t, filepath.Join(directory, "bad.tf"), "output {\n")
		var output bytes.Buffer

		err := Run([]string{"apply", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
			t.Fatalf("apply error = %v, want exit code 2", err)
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Errorf("parse failure changed source: got %q, error %v", got, err)
		}
		if _, err := os.Lstat(filepath.Join(directory, "terraform.tf")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("parse failure created terraform.tf: %v", err)
		}
	})
}

func testNamedFileRenameSafety(t *testing.T, blockType, source string) {
	t.Helper()
	sourceName := blockType + ".tf"
	destinationName := blockType + "s.tf"
	for _, test := range []struct {
		name        string
		filename    string
		content     string
		config      string
		destination string
	}{
		{name: "canonical", filename: destinationName, content: source},
		{name: "no matching block", filename: sourceName, content: "locals {}\n"},
		{name: "comment only", filename: sourceName, content: "# " + blockType + " \"name\" {}\n"},
		{name: "nested only", filename: sourceName, content: "resource \"example\" \"name\" {\n  " + blockType + " {}\n}\n"},
		{name: "string only", filename: sourceName, content: "locals { text = \"" + blockType + "\" }\n"},
		{name: "override", filename: blockType + "_override.tf", content: source},
		{name: "json", filename: blockType + ".tf.json", content: "{\"" + blockType + "\": {\"name\": {}}}\n"},
		{name: "query", filename: blockType + ".tfquery.hcl", content: source},
		{name: "disabled", filename: sourceName, content: source, config: blockType + "s_file_rename = false\n"},
		{name: "destination file", filename: sourceName, content: source, destination: "file"},
		{name: "destination directory", filename: sourceName, content: source, destination: "directory"},
		{name: "destination broken link", filename: sourceName, content: source, destination: "link"},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			filename := filepath.Join(directory, test.filename)
			writeConfig(t, filename, test.content)
			if test.config != "" {
				writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), test.config)
			}
			destination := filepath.Join(directory, destinationName)
			switch test.destination {
			case "file":
				writeConfig(t, destination, "# keep\n")
			case "directory":
				if err := os.Mkdir(destination, 0o700); err != nil {
					t.Fatal(err)
				}
			case "link":
				if err := os.Symlink("missing", destination); err != nil {
					t.Fatal(err)
				}
			}
			for _, command := range []string{"plan", "apply"} {
				var output bytes.Buffer
				err := Run([]string{command, "--chdir", directory}, &output)
				if test.destination == "link" {
					var exitError interface{ ExitCode() int }
					if !errors.As(err, &exitError) || exitError.ExitCode() != 2 || output.Len() != 0 {
						t.Fatalf("%s = %v, output %q, want discovery error", command, err, output.String())
					}
				} else if err != nil || output.String() != cleanCommandOutput(command) {
					t.Fatalf("%s = %v, output %q, want clean success", command, err, output.String())
				}
				if got, err := os.ReadFile(filename); err != nil || string(got) != test.content {
					t.Fatalf("source = %q, error %v, want unchanged", got, err)
				}
			}
			if test.destination == "file" {
				if got, err := os.ReadFile(destination); err != nil || string(got) != "# keep\n" {
					t.Fatalf("destination changed: %q, %v", got, err)
				}
			}
			if test.destination == "link" {
				if got, err := os.Readlink(destination); err != nil || got != "missing" {
					t.Fatalf("destination link changed: %q, %v", got, err)
				}
				parsed, diagnostics := parseHCLSource(filename, []byte(test.content))
				if diagnostics.HasErrors() {
					t.Fatal(diagnostics.Error())
				}
				plan, err := fileRenameTarget(filename, parsed, Config{OutputsFileRename: true, VariablesFileRename: true})
				if err != nil || plan.destination != "" {
					t.Fatalf("rename plan = %+v, %v, want no rename", plan, err)
				}
			}
		})
	}
	for _, test := range []struct{ name, config, prefix string }{
		{name: "rename only"},
		{name: "empty config", config: "\n"},
		{name: "order disabled", config: blockType + "_order = false\n"},
		{name: "with ordering", prefix: "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			filename := filepath.Join(directory, sourceName)
			destination := filepath.Join(directory, destinationName)
			original := test.prefix + source
			writeConfig(t, filename, original)
			// #nosec G302 -- Verify that renaming preserves the original group-readable mode.
			if err := os.Chmod(filename, 0o640); err != nil {
				t.Fatal(err)
			}
			if test.config != "" {
				writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), test.config)
			}
			var output bytes.Buffer
			err := Run([]string{"plan", "--chdir", directory}, &output)
			var exitError interface{ ExitCode() int }
			if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
				t.Fatalf("plan error = %v", err)
			}
			if !strings.Contains(output.String(), "diff --git a/"+sourceName+" b/"+destinationName+"\n") || !strings.Contains(output.String(), "rename from "+sourceName+"\nrename to "+destinationName+"\n") {
				t.Fatalf("missing rename diff: %q", output.String())
			}
			want := original
			if test.prefix == "" {
				if !strings.Contains(output.String(), "similarity index 100%\n") || strings.Contains(output.String(), "--- ") {
					t.Fatalf("unexpected content diff: %q", output.String())
				}
			} else {
				want = "terraform {\n  required_version = \">= 1.0\"\n  backend \"local\" {}\n}\n\n" + source
				if !strings.Contains(output.String(), "--- a/"+sourceName+"\n+++ b/"+destinationName+"\n") || strings.Contains(output.String(), "similarity index 100%") {
					t.Fatalf("missing content diff: %q", output.String())
				}
			}
			if got, err := os.ReadFile(filename); err != nil || string(got) != original {
				t.Fatalf("plan changed source: %q, %v", got, err)
			}
			if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("plan created destination: %v", err)
			}
			output.Reset()
			for _, command := range []string{"apply", "apply", "plan"} {
				if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.String() != cleanCommandOutput(command) {
					t.Fatalf("%s = %v, output %q", command, err, output.String())
				}
				if got, err := os.ReadFile(destination); err != nil || string(got) != want {
					t.Fatalf("destination = %q, %v, want %q", got, err, want)
				}
			}
			if _, err := os.Lstat(filename); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("source remains: %v", err)
			}
			info, err := os.Stat(destination)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o640 {
				t.Fatalf("mode = %v, want 0640", info.Mode().Perm())
			}
		})
	}
	t.Run("symbolic link source", func(t *testing.T) {
		directory := t.TempDir()
		writeConfig(t, filepath.Join(directory, "real.txt"), source)
		if err := os.Symlink("real.txt", filepath.Join(directory, sourceName)); err != nil {
			t.Fatal(err)
		}
		for _, command := range []string{"plan", "apply"} {
			var output bytes.Buffer
			err := Run([]string{command, "--chdir", directory}, &output)
			var exitError interface{ ExitCode() int }
			if !errors.As(err, &exitError) || exitError.ExitCode() != 2 || !strings.Contains(err.Error(), "symbolic link") {
				t.Fatalf("%s error = %v", command, err)
			}
		}
		if got, err := os.ReadFile(filepath.Join(directory, "real.txt")); err != nil || string(got) != source {
			t.Fatalf("link target changed: %q, %v", got, err)
		}
		if _, err := os.Lstat(filepath.Join(directory, destinationName)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("destination created: %v", err)
		}
	})
	t.Run("parse failure prevents all writes", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, sourceName)
		writeConfig(t, filename, source)
		writeConfig(t, filepath.Join(directory, "bad.tf"), "output {\n")
		var output bytes.Buffer
		err := Run([]string{"apply", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
			t.Fatalf("apply error = %v", err)
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("source changed: %q, %v", got, err)
		}
		if _, err := os.Lstat(filepath.Join(directory, destinationName)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("destination created: %v", err)
		}
	})
}

func TestOutputFileRenameFailureKeepsSource(t *testing.T) {
	testNamedFileRenameFailure(t, "output")
}

func TestVariableFileRenameFailureKeepsSource(t *testing.T) {
	testNamedFileRenameFailure(t, "variable")
}

func testNamedFileRenameFailure(t *testing.T, blockType string) {
	t.Helper()
	for _, failure := range []string{"destination appears", "link unsupported", "source changed"} {
		t.Run(failure, func(t *testing.T) {
			directory := t.TempDir()
			filename := filepath.Join(directory, blockType+".tf")
			destination := filepath.Join(directory, blockType+"s.tf")
			source := blockType + " \"name\" {}\n"
			writeConfig(t, filename, source)
			operations := defaultApplyFileOperations()
			wantSource := source
			switch failure {
			case "destination appears":
				writeConfig(t, destination, "# keep\n")
			case "link unsupported":
				operations.link = func(string, string) error { return errors.New("hard links unavailable") }
			case "source changed":
				wantSource = source + "# external change\n"
				writeConfig(t, filename, wantSource)
			}
			err := applyRenameWithOperations(planReport{filename: filename, candidate: []byte(source), renameTo: destination}, operations)
			if err == nil {
				t.Fatal("rename succeeded, want failure")
			}
			if got, err := os.ReadFile(filename); err != nil || string(got) != wantSource {
				t.Fatalf("source = %q, %v, want preserved", got, err)
			}
			if failure == "destination appears" {
				if got, err := os.ReadFile(destination); err != nil || string(got) != "# keep\n" {
					t.Fatalf("destination = %q, %v, want preserved", got, err)
				}
			} else if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("destination created: %v", err)
			}
		})
	}
}

func TestTerraformFileRenameFailureKeepsSource(t *testing.T) {
	for _, failure := range []string{"destination appears", "link unsupported", "source changed"} {
		t.Run(failure, func(t *testing.T) {
			directory := t.TempDir()
			filename := filepath.Join(directory, "backend.tf")
			destination := filepath.Join(directory, "terraform.tf")
			source := "terraform {}\n"
			writeConfig(t, filename, source)
			operations := defaultApplyFileOperations()
			wantSource := source
			switch failure {
			case "destination appears":
				writeConfig(t, destination, "# keep\n")
			case "link unsupported":
				operations.link = func(string, string) error { return errors.New("hard links unavailable") }
			case "source changed":
				wantSource = source + "# external change\n"
				writeConfig(t, filename, wantSource)
			}
			err := applyRenameWithOperations(planReport{filename: filename, candidate: []byte(source), renameTo: destination}, operations)
			if err == nil {
				t.Fatal("rename succeeded, want failure")
			}
			if got, err := os.ReadFile(filename); err != nil || string(got) != wantSource {
				t.Fatalf("source = %q, %v, want preserved", got, err)
			}
			if failure == "destination appears" {
				if got, err := os.ReadFile(destination); err != nil || string(got) != "# keep\n" {
					t.Fatalf("destination = %q, %v, want preserved", got, err)
				}
			} else if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("destination created: %v", err)
			}
		})
	}
}

func TestFileRenameIndependentSettings(t *testing.T) {
	rules := []struct{ name, sourceName, destinationName, source string }{
		{name: "locals", sourceName: "local.tf", destinationName: "locals.tf", source: "locals {}\n"},
		{name: "outputs", sourceName: "output.tf", destinationName: "outputs.tf", source: "output \"name\" {\n  value = \"example\"\n}\n"},
		{name: "variables", sourceName: "variable.tf", destinationName: "variables.tf", source: "variable \"name\" {\n  default = \"example\"\n}\n"},
		{name: "terraform", sourceName: "backend.tf", destinationName: "terraform.tf", source: "terraform {}\n"},
	}
	for _, disabled := range rules {
		t.Run(disabled.name, func(t *testing.T) {
			directory := t.TempDir()
			writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), disabled.name+"_file_rename = false\n")
			for _, rule := range rules {
				writeConfig(t, filepath.Join(directory, rule.sourceName), rule.source)
			}
			var output bytes.Buffer
			err := Run([]string{"plan", "--chdir", directory}, &output)
			var exitError interface{ ExitCode() int }
			if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
				t.Fatalf("plan error = %v", err)
			}
			if strings.Count(output.String(), ": rename to ") != len(rules)-1 || strings.Contains(output.String(), "[file-name/"+disabled.name+"]") {
				t.Fatalf("unexpected plan output: %q", output.String())
			}
			output.Reset()
			if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil || output.Len() != 0 {
				t.Fatalf("apply = %v, output %q", err, output.String())
			}
			for _, rule := range rules {
				name, absent := rule.destinationName, rule.sourceName
				if rule.name == disabled.name {
					name, absent = rule.sourceName, rule.destinationName
				}
				if got, err := os.ReadFile(filepath.Join(directory, name)); err != nil || string(got) != rule.source {
					t.Fatalf("%s = %q, %v", name, got, err)
				}
				if _, err := os.Lstat(filepath.Join(directory, absent)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("unexpected file %s: %v", absent, err)
				}
			}
			if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil || output.String() != "No changes.\n" {
				t.Fatalf("second plan = %v, output %q", err, output.String())
			}
		})
	}
}

func TestFileRenameRecursive(t *testing.T) {
	directory := t.TempDir()
	child := filepath.Join(directory, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "outputs_file_rename = false\nvariables_file_rename = false\n")
	writeConfig(t, filepath.Join(child, ".tfstylus.hcl"), "")
	for _, rule := range []struct{ sourceName, destinationName, source string }{
		{sourceName: "output.tf", destinationName: "outputs.tf", source: "output \"name\" {\n  value = \"example\"\n}\n"},
		{sourceName: "variable.tf", destinationName: "variables.tf", source: "variable \"name\" {\n  default = \"example\"\n}\n"},
	} {
		writeConfig(t, filepath.Join(directory, rule.destinationName), "# parent preserved\n")
		writeConfig(t, filepath.Join(directory, rule.sourceName), rule.source)
		writeConfig(t, filepath.Join(child, rule.sourceName), rule.source)
	}
	var output bytes.Buffer
	if err := Run([]string{"apply", "--recursive", "--chdir", directory}, &output); err != nil || output.Len() != 0 {
		t.Fatalf("apply = %v, output %q", err, output.String())
	}
	for _, blockType := range []string{"output", "variable"} {
		if _, err := os.Lstat(filepath.Join(child, blockType+".tf")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("child source remains: %v", err)
		}
		if _, err := os.Stat(filepath.Join(child, blockType+"s.tf")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(directory, blockType+".tf")); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(filepath.Join(directory, blockType+"s.tf")); err != nil || string(got) != "# parent preserved\n" {
			t.Fatalf("parent destination changed: %q, %v", got, err)
		}
	}
	if err := Run([]string{"plan", "--recursive", "--chdir", directory}, &output); err != nil || output.String() != "No changes.\n" {
		t.Fatalf("second plan = %v, output %q", err, output.String())
	}
}

func TestTerraformFileRenameRecursive(t *testing.T) {
	directory := t.TempDir()
	child := filepath.Join(directory, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	source := "terraform {\n  required_version = \">= 1.0\"\n}\n"
	writeConfig(t, filepath.Join(directory, "backend.tf"), source)
	writeConfig(t, filepath.Join(directory, "providers.tf"), source)
	writeConfig(t, filepath.Join(child, "versions.tf"), source)
	var output bytes.Buffer

	if err := Run([]string{"apply", "--recursive", "--chdir", directory}, &output); err != nil || output.Len() != 0 {
		t.Fatalf("apply = %v, output %q", err, output.String())
	}
	if got, err := os.ReadFile(filepath.Join(directory, "terraform.tf")); err != nil || string(got) != source {
		t.Fatalf("parent terraform.tf = %q, error %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(directory, "providers.tf")); err != nil || string(got) != source {
		t.Fatalf("parent non-selected providers.tf = %q, error %v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(directory, "backend.tf")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("parent backend.tf remains: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(child, "terraform.tf")); err != nil || string(got) != source {
		t.Fatalf("child terraform.tf = %q, error %v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(child, "versions.tf")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("child versions.tf remains: %v", err)
	}
	if err := Run([]string{"plan", "--recursive", "--chdir", directory}, &output); err != nil || output.String() != "No changes.\n" {
		t.Fatalf("second plan = %v, output %q", err, output.String())
	}
}

func TestTerraformFileRenameDoesNotMergeBlocks(t *testing.T) {
	directory := t.TempDir()
	source := "terraform {}\n\nterraform {}\n"
	writeConfig(t, filepath.Join(directory, "versions.tf"), source)
	var output bytes.Buffer

	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
		t.Fatalf("apply error = %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(directory, "terraform.tf")); err != nil || string(got) != source {
		t.Fatalf("terraform.tf = %q, error %v, want both source blocks unchanged", got, err)
	}
}

func TestTerraformFileRenameTreatsBrokenDestinationAsExisting(t *testing.T) {
	directory := t.TempDir()
	source := "terraform {}\n"
	sourcePath := filepath.Join(directory, "backend.tf")
	destination := filepath.Join(directory, "terraform.tf")
	writeConfig(t, sourcePath, source)
	if err := os.Symlink("missing.tf", destination); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	parsed, diagnostics := parseHCLSource(sourcePath, []byte(source))
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	_, plan, err := terraformFileRenameTarget(
		targetDirectory{directory: directory, files: []string{sourcePath}},
		map[string]*parsedFile{sourcePath: parsed},
		Config{TerraformFileRename: true},
	)
	if err != nil || plan.destination != "" {
		t.Fatalf("rename plan = %+v, %v, want no rename", plan, err)
	}

	var output bytes.Buffer
	err = Run([]string{"plan", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("plan error = %v, want target discovery error", err)
	}
	if got, err := os.ReadFile(sourcePath); err != nil || string(got) != source {
		t.Errorf("source changed: got %q, error %v", got, err)
	}
}

func TestPlanLocalFileRename(t *testing.T) {
	const localsSource = "locals {\n  name = \"example\"\n}\n"

	t.Run("reports rename without modifying input", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "local.tf")
		writeConfig(t, filename, localsSource)
		var output bytes.Buffer

		err := Run([]string{"plan", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("Run() error = %v, want exit code 1 for pending rename", err)
		}
		if !strings.HasPrefix(output.String(), "local.tf: rename to locals.tf [file-name/locals] ") {
			t.Errorf("Run() output = %q, want rename report", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != localsSource {
			t.Errorf("plan changed input: got %q, read error %v", got, readErr)
		}
		if _, statErr := os.Lstat(filepath.Join(directory, "locals.tf")); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("plan created locals.tf: %v", statErr)
		}
	})

	for name, files := range map[string]map[string]string{
		"locals.tf only":                {"locals.tf": localsSource},
		"local.tf without locals block": {"local.tf": "resource \"null_resource\" \"a\" {}\n"},
		"locals block only in comment":  {"local.tf": "# locals {}\n"},
		"both files exist":              {"local.tf": localsSource, "locals.tf": localsSource},
	} {
		t.Run("no changes for "+name, func(t *testing.T) {
			directory := t.TempDir()
			for filename, source := range files {
				writeConfig(t, filepath.Join(directory, filename), source)
			}
			var output bytes.Buffer

			if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			if output.String() != "No changes.\n" {
				t.Errorf("Run() output = %q, want No changes.", output.String())
			}
		})
	}

	t.Run("disabled by config reports no changes", func(t *testing.T) {
		directory := t.TempDir()
		writeConfig(t, filepath.Join(directory, "local.tf"), localsSource)
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "locals_file_rename = false\n")
		var output bytes.Buffer

		if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil || output.String() != "No changes.\n" {
			t.Fatalf("Run() = %v, output %q, want clean success", err, output.String())
		}
	})

	t.Run("symbolic link source is an error", func(t *testing.T) {
		directory := t.TempDir()
		writeConfig(t, filepath.Join(directory, "real.tf.txt"), localsSource)
		if err := os.Symlink("real.tf.txt", filepath.Join(directory, "local.tf")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		var output bytes.Buffer

		err := Run([]string{"plan", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 2 || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("Run() error = %v, want exit code 2 for symbolic link", err)
		}
	})

	t.Run("diff shows git style rename with similarity", func(t *testing.T) {
		directory := t.TempDir()
		writeConfig(t, filepath.Join(directory, "local.tf"), localsSource)
		var output bytes.Buffer

		_ = Run([]string{"plan", "--chdir", directory}, &output)
		want := "diff --git a/local.tf b/locals.tf\nsimilarity index 100%\nrename from local.tf\nrename to locals.tf\n"
		if !strings.Contains(output.String(), want) || strings.Contains(output.String(), "--- ") {
			t.Errorf("Run() output = %q, want %q without content diff", output.String(), want)
		}
	})

	t.Run("diff shows content change after rename header", func(t *testing.T) {
		directory := t.TempDir()
		source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n\n" + localsSource
		writeConfig(t, filepath.Join(directory, "local.tf"), source)
		var output bytes.Buffer

		_ = Run([]string{"plan", "--chdir", directory}, &output)
		got := output.String()
		if !strings.Contains(got, "rename from local.tf\nrename to locals.tf\n--- a/local.tf\n+++ b/locals.tf\n") || strings.Contains(got, "similarity index 100%") || !strings.Contains(got, "similarity index ") {
			t.Errorf("Run() output = %q, want rename header followed by content diff and partial similarity", got)
		}
	})
}

func TestApplyLocalFileRename(t *testing.T) {
	const localsSource = "locals {\n  name = \"example\"\n}\n"

	t.Run("renames after content replacement and is idempotent", func(t *testing.T) {
		directory := t.TempDir()
		source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n\n" + localsSource
		writeConfig(t, filepath.Join(directory, "local.tf"), source)
		// #nosec G302 -- Verify that content replacement and renaming preserve the original group-readable mode.
		if err := os.Chmod(filepath.Join(directory, "local.tf"), 0o640); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer

		if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := "terraform {\n  required_version = \">= 1.0\"\n  backend \"local\" {}\n}\n\n" + localsSource
		got, err := os.ReadFile(filepath.Join(directory, "locals.tf"))
		if err != nil || string(got) != want {
			t.Fatalf("locals.tf = %q, error %v, want %q", got, err, want)
		}
		if _, err := os.Lstat(filepath.Join(directory, "local.tf")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("local.tf still exists: %v", err)
		}
		if info, err := os.Stat(filepath.Join(directory, "locals.tf")); err != nil || info.Mode().Perm() != 0o640 {
			t.Errorf("locals.tf mode = %v, error %v, want 0640", info.Mode().Perm(), err)
		}
		if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil || output.String() != "No changes.\n" {
			t.Errorf("second plan = %v, output %q, want clean", err, output.String())
		}
	})

	t.Run("renames without content change", func(t *testing.T) {
		directory := t.TempDir()
		writeConfig(t, filepath.Join(directory, "local.tf"), localsSource)
		var output bytes.Buffer

		if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, err := os.ReadFile(filepath.Join(directory, "locals.tf")); err != nil || string(got) != localsSource {
			t.Fatalf("locals.tf = %q, error %v", got, err)
		}
	})

	t.Run("does nothing when disabled or locals.tf exists", func(t *testing.T) {
		directory := t.TempDir()
		writeConfig(t, filepath.Join(directory, "local.tf"), localsSource)
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "locals_file_rename = false\n")
		var output bytes.Buffer
		if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if _, err := os.Lstat(filepath.Join(directory, "local.tf")); err != nil {
			t.Errorf("disabled rule renamed local.tf: %v", err)
		}

		existing := t.TempDir()
		writeConfig(t, filepath.Join(existing, "local.tf"), localsSource)
		writeConfig(t, filepath.Join(existing, "locals.tf"), "# keep\n")
		if err := Run([]string{"apply", "--chdir", existing}, &output); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, err := os.ReadFile(filepath.Join(existing, "locals.tf")); err != nil || string(got) != "# keep\n" {
			t.Errorf("locals.tf = %q, error %v, want unchanged", got, err)
		}
		if _, err := os.Lstat(filepath.Join(existing, "local.tf")); err != nil {
			t.Errorf("local.tf was removed: %v", err)
		}
	})

	t.Run("refuses symbolic link source", func(t *testing.T) {
		directory := t.TempDir()
		writeConfig(t, filepath.Join(directory, "real.tf.txt"), localsSource)
		if err := os.Symlink("real.tf.txt", filepath.Join(directory, "local.tf")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		var output bytes.Buffer

		err := Run([]string{"apply", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
			t.Fatalf("Run() error = %v, want exit code 2", err)
		}
		if _, err := os.Lstat(filepath.Join(directory, "locals.tf")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("locals.tf was created: %v", err)
		}
	})
}

func TestApplyRenameKeepsSourceWhenDestinationAppears(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "local.tf")
	destination := filepath.Join(directory, "locals.tf")
	writeConfig(t, source, "locals {}\n")
	writeConfig(t, destination, "# other\n")

	err := applyRename(planReport{filename: source, candidate: []byte("locals {}\n"), renameTo: destination})
	if err == nil {
		t.Fatal("applyRename() error = nil, want destination conflict")
	}
	if got, readErr := os.ReadFile(destination); readErr != nil || string(got) != "# other\n" {
		t.Errorf("destination = %q, error %v, want unchanged", got, readErr)
	}
	if got, readErr := os.ReadFile(source); readErr != nil || string(got) != "locals {}\n" {
		t.Errorf("source = %q, error %v, want kept", got, readErr)
	}
}

func TestApplyRenameKeepsSourceWhenLinkIsUnsupported(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "local.tf")
	writeConfig(t, source, "locals {}\n")
	operations := defaultApplyFileOperations()
	operations.link = func(string, string) error { return errors.New("hard links are not supported") }

	err := applyRenameWithOperations(planReport{filename: source, candidate: []byte("locals {}\n"), renameTo: filepath.Join(directory, "locals.tf")}, operations)
	if err == nil || !strings.Contains(err.Error(), "hard links are not supported") {
		t.Fatalf("applyRenameWithOperations() error = %v, want link failure", err)
	}
	if got, readErr := os.ReadFile(source); readErr != nil || string(got) != "locals {}\n" {
		t.Errorf("source = %q, error %v, want kept", got, readErr)
	}
	if _, statErr := os.Lstat(filepath.Join(directory, "locals.tf")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("locals.tf was created: %v", statErr)
	}
}

func TestPlanCommand(t *testing.T) {
	t.Run("reports changes without modifying input", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
		writeConfig(t, filename, source)
		var output bytes.Buffer

		err := Run([]string{"plan", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("Run() error = %v, want exit code 1 for pending changes", err)
		}
		if !strings.Contains(output.String(), "main.tf:3:3") || !strings.Contains(output.String(), "required_version") {
			t.Errorf("Run() output = %q, want original source location and rule", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("plan changed input: got %q, read error %v", got, readErr)
		}
	})

	t.Run("prints unified diff without modifying input", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
		writeConfig(t, filename, source)
		var output bytes.Buffer

		err := Run([]string{"plan", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("Run() error = %v, want exit code 1 for pending changes", err)
		}
		if !strings.Contains(output.String(), "--- ") || !strings.Contains(output.String(), "+++ ") {
			t.Errorf("Run() output = %q, want unified diff headers", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("plan changed input: got %q, read error %v", got, readErr)
		}
	})

	t.Run("clean file reports no changes", func(t *testing.T) {
		directory := t.TempDir()
		source := "terraform {\n  required_version = \">= 1.0\"\n}\n"
		writeConfig(t, filepath.Join(directory, "main.tf"), source)
		var output bytes.Buffer

		if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if output.String() != "No changes.\n" {
			t.Errorf("Run() output = %q, want No changes.", output.String())
		}
	})

	t.Run("backend order is reported and fixed by apply", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  required_version = \">= 1.0\"\n  required_providers {\n    aws = {\n      source = \"hashicorp/aws\"\n    }\n  }\n  backend \"local\" {}\n}\n"
		writeConfig(t, filename, source)
		var output bytes.Buffer

		err := Run([]string{"plan", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("Run() error = %v, want exit code 1 for pending backend change", err)
		}
		if !strings.Contains(output.String(), "backend_order") {
			t.Errorf("Run() output = %q, want backend ordering report", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("plan changed input: got %q, read error %v", got, readErr)
		}

		if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil {
			t.Fatalf("apply failed to apply backend order: %v", err)
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != "terraform {\n  required_version = \">= 1.0\"\n  backend \"local\" {}\n  required_providers {\n    aws = {\n      source = \"hashicorp/aws\"\n    }\n  }\n}\n" {
			t.Fatalf("apply output mismatch: got %q, read error %v", got, readErr)
		}
	})

	t.Run("backend order still runs when required_version rule is disabled", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  required_version = \">= 1.0\"\n  required_providers {\n    aws = {\n      source = \"hashicorp/aws\"\n    }\n  }\n  backend \"local\" {}\n}\n"
		writeConfig(t, filename, source)
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = true\n")
		var output bytes.Buffer

		err := Run([]string{"plan", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("Run() error = %v, want exit code 1 for backend ordering while required_version rule is disabled", err)
		}
		if !strings.Contains(output.String(), "backend_order") {
			t.Errorf("Run() output = %q, want backend ordering report even when required_version_order is disabled", output.String())
		}
	})

	t.Run("already ordered with boundary comment reports no changes", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  required_version = \">= 1.15.0\"\n  # コメント\n}\n"
		writeConfig(t, filename, source)
		var output bytes.Buffer

		if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if output.String() != "No changes.\n" {
			t.Errorf("Run() output = %q, want No changes.", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("plan changed input: got %q, read error %v", got, readErr)
		}
	})

	t.Run("disabled rule reports no changes", func(t *testing.T) {
		directory := t.TempDir()
		source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = false\n")
		writeConfig(t, filepath.Join(directory, "main.tf"), source)
		var output bytes.Buffer

		if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if output.String() != "No changes.\n" {
			t.Errorf("Run() output = %q, want No changes.", output.String())
		}
	})

	t.Run("ambiguous comment is reported as skipped", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  backend \"local\" {}\n  # independent section comment\n\n  required_version = \">= 1.0\"\n}\n"
		writeConfig(t, filename, source)
		var output bytes.Buffer

		err := Run([]string{"plan", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
			t.Fatalf("Run() error = %v, want exit code 2 for skipped transformation", err)
		}
		if !strings.Contains(output.String(), "warning: skipped") {
			t.Errorf("Run() output = %q, want skip warning", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("plan changed skipped input: got %q, read error %v", got, readErr)
		}
	})

	t.Run("multiple files are reported in stable order", func(t *testing.T) {
		directory := t.TempDir()
		source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
		writeConfig(t, filepath.Join(directory, "b.tf"), source)
		writeConfig(t, filepath.Join(directory, "a.tf"), source)
		var output bytes.Buffer

		err := Run([]string{"plan", "--chdir", directory}, &output)
		if err == nil {
			t.Fatal("Run() error = nil, want exit code 1 for pending changes")
		}
		if strings.Index(output.String(), "a.tf:") > strings.Index(output.String(), "b.tf:") {
			t.Errorf("Run() output = %q, want files in lexical order", output.String())
		}
	})

	t.Run("syntax error has exit code 2", func(t *testing.T) {
		directory := t.TempDir()
		writeConfig(t, filepath.Join(directory, "main.tf"), "terraform { invalid = }\n")
		var output bytes.Buffer

		err := Run([]string{"plan", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
			t.Fatalf("Run() error = %v, want exit code 2 for syntax error", err)
		}
	})
}

func TestApplyCollectsTargetSyntaxErrorsWithoutModifyingFiles(t *testing.T) {
	directory := t.TempDir()
	files := map[string]string{
		"main.tf":           "locals {\n  value =\n}\n",
		"variables.tf.json": "{\n  \"unknown\": }\n",
	}
	for name, content := range files {
		writeConfig(t, filepath.Join(directory, name), content)
	}

	err := runFromDirectory(t, directory, []string{"plan"})
	if err == nil {
		t.Fatal("Run() error = nil, want target syntax errors")
	}
	for name := range files {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("Run() error = %q, want it to contain %q", err, name)
		}
		content, readErr := os.ReadFile(filepath.Join(directory, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(content) != files[name] {
			t.Errorf("Run() modified %s: got %q, want %q", name, content, files[name])
		}
	}
}

func TestRunConfigDiscovery(t *testing.T) {
	t.Run("loads config from current directory", func(t *testing.T) {
		workingDirectory := t.TempDir()
		writeConfig(t, filepath.Join(workingDirectory, ".tfstylus.hcl"), "")

		err := runFromDirectory(t, workingDirectory, []string{"plan"})
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	})

	t.Run("uses no config when none is found", func(t *testing.T) {
		err := runFromDirectory(t, t.TempDir(), []string{"apply"})
		if err != nil {
			t.Fatalf("Run() error = %v, want nil without config", err)
		}
	})

	t.Run("does not fall back after selected config fails", func(t *testing.T) {
		parent := t.TempDir()
		writeConfig(t, filepath.Join(parent, ".tfstylus.hcl"), "")
		child := filepath.Join(parent, "child")
		if err := os.Mkdir(child, 0o700); err != nil {
			t.Fatal(err)
		}
		writeConfig(t, filepath.Join(child, ".tfstylus.hcl"), "invalid {")

		err := runFromDirectory(t, child, []string{"plan"})
		if err == nil || !strings.Contains(err.Error(), "parse config") {
			t.Fatalf("Run() error = %v, want selected config parse error", err)
		}
	})

	t.Run("explicit config takes precedence", func(t *testing.T) {
		workingDirectory := t.TempDir()
		writeConfig(t, filepath.Join(workingDirectory, ".tfstylus.hcl"), "invalid {")
		explicitConfig := filepath.Join(t.TempDir(), "explicit.hcl")
		writeConfig(t, explicitConfig, "")

		err := runFromDirectory(t, workingDirectory, []string{"plan", "--config", explicitConfig})
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	})
}

func TestExplicitConfigMissingFile(t *testing.T) {
	for _, command := range []string{"plan", "apply"} {
		for _, absolute := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/absolute=%t", command, absolute), func(t *testing.T) {
				root := t.TempDir()
				t.Chdir(root)
				writeConfig(t, filepath.Join(root, "child", "missing.hcl"), "")
				writeConfig(t, filepath.Join(root, "child", ".tfstylus.hcl"), "")
				configPath := "missing.hcl"
				if absolute {
					configPath = filepath.Join(root, configPath)
				}
				err := Run([]string{command, "--chdir=child", "-r", "-c", configPath}, &bytes.Buffer{})
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("Run() error = %v, want an error matching os.ErrNotExist", err)
				}
				var exitError interface{ ExitCode() int }
				if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
					t.Fatalf("Run() error = %v, want exit code 2 for config error", err)
				}
			})
		}
	}
}

func TestExplicitConfigFromWorkingDirectory(t *testing.T) {
	for _, command := range []string{"plan", "apply"} {
		for _, test := range []struct {
			name      string
			flags     []string
			configArg string
		}{
			{"recursive", []string{"-r"}, "settings/explicit.hcl"},
			{"chdir", []string{"--chdir", "child"}, "settings/explicit.hcl"},
			{"recursive chdir", []string{"--chdir=child", "--recursive"}, "settings/explicit.hcl"},
			{"parent relative", []string{"--chdir=child", "-r"}, "../settings/explicit.hcl"},
			{"absolute", []string{"--chdir=child", "-r"}, "{root}/settings/explicit.hcl"},
		} {
			t.Run(command+"/"+test.name, func(t *testing.T) {
				root := filepath.Join(t.TempDir(), "work")
				if err := os.Mkdir(root, 0o700); err != nil {
					t.Fatal(err)
				}
				t.Chdir(root)
				config := "replace_required_version = \">= 1.7.0\"\n"
				writeConfig(t, filepath.Join(root, "settings", "explicit.hcl"), config)
				writeConfig(t, filepath.Join(root, "..", "settings", "explicit.hcl"), config)
				source := "terraform {\n  required_version = \">= 1.0\"\n}\n"
				want := "terraform {\n  required_version = \">= 1.7.0\"\n}\n"
				for _, directory := range []string{".", "child", "child/deep"} {
					writeConfig(t, filepath.Join(root, directory, ".tfstylus.hcl"), "invalid {")
					writeConfig(t, filepath.Join(root, directory, "main.tf"), source)
				}
				configArg := strings.ReplaceAll(test.configArg, "{root}", root)
				args := append([]string{command}, test.flags...)
				if command == "plan" {
					args = append(args, "--config="+configArg)
				} else {
					args = append(args, "-c", configArg)
				}
				var output bytes.Buffer
				err := Run(args, &output)
				if command == "plan" {
					var exitError interface{ ExitCode() int }
					if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
						t.Fatalf("Run() error = %v, want pending changes", err)
					}
					if !strings.Contains(output.String(), "+  required_version = \">= 1.7.0\"\n") {
						t.Fatalf("missing configured replacement: %s", &output)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				for _, directory := range []string{".", "child", "child/deep"} {
					selected := !strings.Contains(strings.Join(test.flags, " "), "--chdir") || directory != "."
					if test.name == "chdir" && directory == "child/deep" {
						selected = false
					}
					expected := source
					if command == "apply" && selected {
						expected = want
					}
					filename := filepath.Join(root, directory, "main.tf")
					content, err := os.ReadFile(filename)
					if err != nil || string(content) != expected {
						t.Fatalf("%s content = %q, error = %v; want %q", filename, content, err, expected)
					}
					if command == "plan" && selected {
						diffPath := "b/" + filepath.ToSlash(filepath.Join(directory, "main.tf"))
						if strings.Contains(strings.Join(test.flags, " "), "--chdir") {
							relative, err := filepath.Rel("child", filepath.Join(directory, "main.tf"))
							if err != nil {
								t.Fatal(err)
							}
							diffPath = "b/" + filepath.ToSlash(relative)
						}
						if !strings.Contains(output.String(), "+++ "+diffPath+"\n") {
							t.Fatalf("missing diff for %s: %s", filename, &output)
						}
					}
				}
				if command == "apply" {
					if err := Run(args, &bytes.Buffer{}); err != nil {
						t.Fatalf("second apply: %v", err)
					}
				}
			})
		}
	}
}

func TestExplicitConfigFlagPositions(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "empty.hcl")
	if err := os.WriteFile(configPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{name: "short before command", args: []string{"-c", configPath, "plan"}},
		{name: "short after command", args: []string{"plan", "-c", configPath}},
		{name: "long before command", args: []string{"--config", configPath, "apply"}},
		{name: "long after command", args: []string{"apply", "--config", configPath}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer

			err := Run(test.args, &output)
			if strings.Contains(strings.Join(test.args, " "), "apply") && err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			if !strings.Contains(strings.Join(test.args, " "), "apply") && err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
		})
	}
}

func TestExplicitConfigArgumentErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing value", args: []string{"plan", "-c"}},
		{name: "repeated flag", args: []string{"plan", "-c", "first.hcl", "--config", "second.hcl"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := Run(test.args, &bytes.Buffer{}); err == nil {
				t.Fatal("Run() error = nil, want argument error")
			}
		})
	}
}

func TestExplicitConfigRejectsInvalidHCL(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "invalid.hcl")
	if err := os.WriteFile(configPath, []byte("invalid {"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Run([]string{"plan", "--config", configPath}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("Run() error = %v, want HCL parse error", err)
	}
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("Run() error = %v, want exit code 2 for invalid config", err)
	}
}

func TestParseArgs(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want options
	}{
		{"defaults", []string{"plan"}, options{command: "plan", directory: "."}},
		{"before and after", []string{"--chdir", "child", "apply", "--recursive", "-c", "settings.hcl"}, options{command: "apply", directory: "child", recursive: true, configPath: "settings.hcl"}},
		{"equals", []string{"plan", "--recursive=true", "--chdir=child"}, options{command: "plan", directory: "child", recursive: true}},
		{"false", []string{"--recursive=false", "plan"}, options{command: "plan", directory: "."}},
		{"repeated recursive", []string{"plan", "--recursive", "--recursive"}, options{command: "plan", directory: ".", recursive: true}},
		{"short recursive", []string{"apply", "-r"}, options{command: "apply", directory: ".", recursive: true}},
		{"short recursive false", []string{"-r=false", "plan"}, options{command: "plan", directory: "."}},
		{"apply diff", []string{"apply", "--diff", "--recursive"}, options{command: "apply", directory: ".", diff: true, recursive: true}},
		{"apply short diff before command", []string{"-d", "-r", "apply"}, options{command: "apply", directory: ".", diff: true, recursive: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseArgs(test.args)
			if err != nil || got != test.want {
				t.Fatalf("parseArgs() = %#v, %v, want %#v", got, err, test.want)
			}
		})
	}
	for _, args := range [][]string{
		{"plan", "--chdir"},
		{"--chdir=", "plan"},
		{"plan", "--chdir", ""},
		{"plan", "--chdir", "--recursive"},
		{"--chdir=a", "plan", "--chdir=b"},
		{"plan", "--recursive=invalid"},
		{"plan", "--recursive="},
		{"plan", "--diff=false"},
		{"init", "--diff"},
		{"plan", "-d=false"},
		{"init", "-d"},
		{"plan", "-r=invalid"},
		{"apply", "--diff=true"},
		{"--diff"},
		{"plan", "--unknown"},
		{"plan", "child"},
		{"plan", "apply"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, err := parseArgs(args); err == nil {
				t.Fatal("parseArgs() error = nil, want argument error")
			}
		})
	}
}

func TestRunTargetDiscovery(t *testing.T) {
	for _, args := range [][]string{
		{"plan", "--chdir", "child"},
		{"--chdir=child", "apply"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, filepath.Join(root, ".tfstylus.hcl"), "invalid {")
			writeConfig(t, filepath.Join(root, "child", ".tfstylus.hcl"), "")
			if err := runFromDirectory(t, root, args); err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			if directory, err := os.Getwd(); err != nil || directory != root {
				t.Fatalf("working directory = %q, %v, want %q", directory, err, root)
			}
		})
	}
	for _, test := range []struct {
		name    string
		args    []string
		configs map[string]string
		wantErr string
	}{
		{"recursive child config", []string{"plan", "--recursive"}, map[string]string{"child/.tfstylus.hcl": "invalid {"}, "parse config"},
		{"direct skips child", []string{"plan"}, map[string]string{"child/.tfstylus.hcl": "invalid {"}, ""},
		{"false skips child", []string{"apply", "--recursive=false"}, map[string]string{"child/.tfstylus.hcl": "invalid {"}, ""},
		{"both flags", []string{"--recursive", "apply", "--chdir=child"}, map[string]string{"child/grandchild/.tfstylus.hcl": "invalid {"}, "parse config"},
		{"chdir inherits parent", []string{"plan", "--chdir=child"}, map[string]string{".tfstylus.hcl": "invalid {"}, "parse config"},
		{"relative explicit after chdir", []string{"plan", "--chdir=child", "-c", "explicit.hcl"}, map[string]string{"explicit.hcl": "invalid {", "child/explicit.hcl": ""}, "parse config"},
		{"recursive relative explicit", []string{"plan", "--recursive", "--config", "explicit.hcl"}, map[string]string{"explicit.hcl": ""}, ""},
		{"recursive absolute explicit", []string{"plan", "--recursive", "--config", "{root}/explicit.hcl"}, map[string]string{"explicit.hcl": "", "child/.tfstylus.hcl": "invalid {"}, ""},
		{"hidden child ignored", []string{"plan", "--recursive"}, map[string]string{".terraform/.tfstylus.hcl": "invalid {", ".git/.tfstylus.hcl": "invalid {"}, ""},
		{"missing directory", []string{"plan", "--chdir=missing"}, nil, "inspect target directory"},
		{"file as directory", []string{"apply", "--recursive", "--chdir=main.tf"}, nil, "not a directory"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, filepath.Join(root, ".tfstylus.hcl"), "")
			writeConfig(t, filepath.Join(root, "main.tf"), "locals { value = 1 }\n")
			writeConfig(t, filepath.Join(root, "child", "main.tf"), "")
			for path, source := range test.configs {
				writeConfig(t, filepath.Join(root, path), source)
			}
			args := make([]string, len(test.args))
			for index, argument := range test.args {
				args[index] = strings.ReplaceAll(argument, "{root}", root)
			}
			err := runFromDirectory(t, root, args)
			if test.wantErr == "" && err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("Run() error = %v, want %q", err, test.wantErr)
			}
			if directory, err := os.Getwd(); err != nil || directory != root {
				t.Fatalf("working directory = %q, %v, want %q", directory, err, root)
			}
		})
	}
	t.Run("validates empty child config", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, filepath.Join(root, ".tfstylus.hcl"), "")
		writeConfig(t, filepath.Join(root, "empty", ".tfstylus.hcl"), "invalid {")
		err := runFromDirectory(t, root, []string{"plan", "--recursive"})
		if err == nil || !strings.Contains(err.Error(), "parse config") {
			t.Fatalf("Run() error = %v, want empty child config error", err)
		}
	})
	t.Run("validates candidate links", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, filepath.Join(root, ".tfstylus.hcl"), "")
		if err := os.Symlink("missing", filepath.Join(root, "broken.tf")); err != nil {
			t.Fatal(err)
		}
		err := runFromDirectory(t, root, []string{"apply"})
		if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "broken.tf") {
			t.Fatalf("Run() error = %v, want broken candidate error", err)
		}
	})
}

func TestHelpAndVersionDoNotDiscoverTargets(t *testing.T) {
	for _, command := range []string{"--help", "--version"} {
		if err := Run([]string{"--chdir=missing", "--recursive", "--config", "missing.hcl", command}, &bytes.Buffer{}); err != nil {
			t.Fatalf("Run(%s) error = %v, want no filesystem checks", command, err)
		}
	}
}

func runFromDirectory(t *testing.T, directory string, args []string) error {
	t.Helper()
	t.Setenv("HOME", directory)
	t.Setenv("USERPROFILE", directory)
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	return Run(args, &bytes.Buffer{})
}

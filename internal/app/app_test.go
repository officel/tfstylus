package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		{name: "help", args: []string{"--help"}, want: "Usage: tfstylus <command> [flags]\n\nFlags:\n  -c, --config PATH  Load an explicit configuration file\n  --chdir DIR        Set the target directory (default: current directory)\n  -r, --recursive    Search directories recursively\n  -d, --diff         Show a unified diff (check only)\n"},
		{name: "short help", args: []string{"-h"}, want: "Usage: tfstylus <command> [flags]\n\nFlags:\n  -c, --config PATH  Load an explicit configuration file\n  --chdir DIR        Set the target directory (default: current directory)\n  -r, --recursive    Search directories recursively\n  -d, --diff         Show a unified diff (check only)\n"},
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

func TestRunCommandWithNoTargets(t *testing.T) {
	if err := runFromDirectory(t, t.TempDir(), []string{"run"}); err != nil {
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
	var report string
	for _, diff := range []bool{false, true} {
		args := []string{"check", "--chdir", directory}
		if diff {
			args = append(args, "--diff")
		}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "main.tf:1:1: variable_order:") {
			t.Fatalf("check = %v; output = %s", err, output.String())
		}
		if diff {
			if !strings.HasPrefix(output.String(), report) || !strings.Contains(output.String(), "--- a/main.tf") || !strings.Contains(output.String(), "+++ b/main.tf") {
				t.Fatalf("missing report/diff: %s", output.String())
			}
		} else {
			report = output.String()
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("check modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"run", "run", "check"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "variable_order = false\n")
	for _, command := range []string{"check", "run"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
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

	var report string
	for _, diff := range []bool{false, true} {
		args := []string{"check", "--chdir", directory}
		if diff {
			args = append(args, "--diff")
		}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "main.tf:1:1: output_order:") {
			t.Fatalf("check = %v; output = %s", err, output.String())
		}
		if diff {
			if !strings.HasPrefix(output.String(), report) || !strings.Contains(output.String(), "--- a/main.tf") {
				t.Fatalf("missing report/diff: %s", output.String())
			}
		} else {
			report = output.String()
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("check modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"run", "run", "check"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "output_order = false\n")
	for _, command := range []string{"check", "run"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
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

	var report string
	for _, diff := range []bool{false, true} {
		args := []string{"check", "--chdir", directory}
		if diff {
			args = append(args, "--diff")
		}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 ||
			!strings.Contains(output.String(), "required_version_order:") || !strings.Contains(output.String(), "data_order:") {
			t.Fatalf("check = %v; output = %s", err, output.String())
		}
		if diff {
			if !strings.HasPrefix(output.String(), report) || !strings.Contains(output.String(), "--- a/main.tf") {
				t.Fatalf("missing report/diff: %s", output.String())
			}
		} else {
			report = output.String()
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("check modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"run", "run", "check"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}

	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\ndata_order = false\n")
	for _, command := range []string{"check", "run"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
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

	var report string
	for _, diff := range []bool{false, true} {
		args := []string{"check", "--chdir", directory}
		if diff {
			args = append(args, "--diff")
		}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "query.tfquery.hcl:5:1: list_order:") {
			t.Fatalf("check = %v; output = %s, want list_order change", err, output.String())
		}
		if diff {
			if !strings.HasPrefix(output.String(), report) || !strings.Contains(output.String(), "--- a/query.tfquery.hcl") {
				t.Fatalf("missing report/diff: %s", output.String())
			}
		} else {
			report = output.String()
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("check modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"run", "run", "check"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "list_order = false\n")
	for _, command := range []string{"check", "run"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
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
	err := Run([]string{"check", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "import_order:") {
		t.Fatalf("check = %v; output = %s", err, output.String())
	}
	if got, err := os.ReadFile(filename); err != nil || string(got) != source {
		t.Fatalf("check modified input: %s; %v", got, err)
	}

	for _, command := range []string{"run", "run", "check"} {
		var commandOutput bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &commandOutput); err != nil || commandOutput.Len() != 0 {
			t.Fatalf("%s = %v; output = %s", command, err, commandOutput.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}

	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "import_order = false\n")
	for _, command := range []string{"check", "run"} {
		var commandOutput bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &commandOutput); err != nil || commandOutput.Len() != 0 {
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

	for _, diff := range []bool{false, true} {
		args := []string{"check", "--chdir", directory}
		if diff {
			args = append(args, "--diff")
		}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "moved_order:") {
			t.Fatalf("check = %v; output = %s", err, output.String())
		}
		if diff && !strings.Contains(output.String(), "+  from = module.old") {
			t.Fatalf("diff missing moved change: %s", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("check modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"run", "run", "check"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "moved_order = false\n")
	for _, command := range []string{"check", "run"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
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

	for _, diff := range []bool{false, true} {
		args := []string{"check", "--chdir", directory}
		if diff {
			args = append(args, "--diff")
		}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "removed_order:") {
			t.Fatalf("check = %v; output = %s", err, output.String())
		}
		if diff && !strings.Contains(output.String(), "+  from = module.old") {
			t.Fatalf("diff missing removed change: %s", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("check modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"run", "run", "check"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "removed_order = false\n")
	for _, input := range []string{source, "removed \"label\" {\n  lifecycle { destroy = false }\n  from = module.old\n}\n"} {
		writeConfig(t, filename, input)
		for _, command := range []string{"check", "run"} {
			var output bytes.Buffer
			if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
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

	for _, diff := range []bool{false, true} {
		args := []string{"check", "--chdir", directory}
		if diff {
			args = append(args, "--diff")
		}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "ephemeral_order:") {
			t.Fatalf("check = %v; output = %s", err, output.String())
		}
		if diff && !strings.Contains(output.String(), "--- a/main.tf") {
			t.Fatalf("check --diff output = %s; want unified diff", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("check modified input: %s; %v", got, err)
		}
	}

	for _, command := range []string{"run", "check"} {
		var commandOutput bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &commandOutput); err != nil || commandOutput.Len() != 0 {
			t.Fatalf("%s = %v; output = %s", command, err, commandOutput.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}

	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "ephemeral_order = false\n")
	for _, command := range []string{"check", "run"} {
		var commandOutput bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &commandOutput); err != nil || commandOutput.Len() != 0 {
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
	for _, diff := range []bool{false, true} {
		args := []string{"check", "--chdir", directory}
		if diff {
			args = append(args, "--diff")
		}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "main.tf:1:1: module_order:") {
			t.Fatalf("check = %v; output = %s", err, output.String())
		}
		if diff && !strings.Contains(output.String(), "--- a/main.tf") {
			t.Fatalf("check --diff output = %s; want unified diff", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("check modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"run", "run", "check"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
	unsafe := "module \"one\" {\n  name = var.name\n\n  # section\n\n  source = \"./child\"\n}\n"
	writeConfig(t, filename, unsafe)
	for _, command := range []string{"check", "run"} {
		var output bytes.Buffer
		err := Run([]string{command, "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
			t.Fatalf("unsafe %s = %v; output = %s", command, err, output.String())
		}
		if command == "check" && !strings.Contains(output.String(), "warning: skipped:") {
			t.Fatalf("check output = %s, want warning", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != unsafe {
			t.Fatalf("unsafe %s changed input: %s; %v", command, got, err)
		}
	}
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "module_order = false\n")
	for _, input := range []string{source, unsafe} {
		writeConfig(t, filename, input)
		for _, command := range []string{"check", "run"} {
			var output bytes.Buffer
			if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
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

	for _, diff := range []bool{false, true} {
		args := []string{"check", "--chdir", directory}
		if diff {
			args = append(args, "--diff")
		}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "resource_order:") {
			t.Fatalf("check = %v; output = %s", err, output.String())
		}
		if diff && !strings.Contains(output.String(), "--- a/main.tf") {
			t.Fatalf("check --diff output = %s; want unified diff", output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("check modified input: %s; %v", got, err)
		}
	}

	for _, command := range []string{"run", "check"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}

	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "resource_order = false\n")
	for _, command := range []string{"check", "run"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
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

	var report string
	for _, diff := range []bool{false, true} {
		args := []string{"check", "--chdir", directory}
		if diff {
			args = append(args, "--diff")
		}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "main.tf:1:1: action_order:") {
			t.Fatalf("check = %v; output = %s", err, output.String())
		}
		if diff {
			if !strings.HasPrefix(output.String(), report) || !strings.Contains(output.String(), "--- a/main.tf") {
				t.Fatalf("missing report/diff: %s", output.String())
			}
		} else {
			report = output.String()
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("check modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"run", "run", "check"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}
	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "action_order = false\n")
	for _, command := range []string{"check", "run"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
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

	var report string
	for _, diff := range []bool{false, true} {
		args := []string{"check", "--chdir", directory}
		if diff {
			args = append(args, "--diff")
		}
		var output bytes.Buffer
		err := Run(args, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || !strings.Contains(output.String(), "main.tf:1:1: check_order:") {
			t.Fatalf("check = %v; output = %s", err, output.String())
		}
		if diff {
			if !strings.HasPrefix(output.String(), report) || !strings.Contains(output.String(), "--- a/main.tf") {
				t.Fatalf("missing report/diff: %s", output.String())
			}
		} else {
			report = output.String()
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != source {
			t.Fatalf("check modified input: %s; %v", got, err)
		}
	}
	for _, command := range []string{"run", "run", "check"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("%s = %v; output = %s", command, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || string(got) != want {
			t.Fatalf("%s result = %s; %v", command, got, err)
		}
	}

	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "check_order = false\n")
	for _, command := range []string{"check", "run"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
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
	err := Run([]string{"check", "--chdir", directory, "--diff"}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("check = %v; output = %s", err, output.String())
	}
	for _, rule := range []string{"required_version_order", "variable_order", "output_order"} {
		if !strings.Contains(output.String(), rule+":") {
			t.Fatalf("check omitted %s: %s", rule, output.String())
		}
	}
	if !strings.Contains(output.String(), "--- a/main.tf") {
		t.Fatalf("check omitted unified diff: %s", output.String())
	}
	if got, err := os.ReadFile(filename); err != nil || got == nil || string(got) != source {
		t.Fatalf("check modified source: %s; %v", got, err)
	}
	if got, err := os.ReadFile(jsonFilename); err != nil || string(got) != jsonSource {
		t.Fatalf("check modified JSON: %s; %v", got, err)
	}
	for _, command := range []string{"run", "run", "check"} {
		var output bytes.Buffer
		if err := Run([]string{command, "--chdir", directory}, &output); err != nil || output.Len() != 0 {
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

	err := Run([]string{"check", "--chdir", directory}, &output)
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("check error = %v, want exit code 1 for composed ordering", err)
	}
	checkOutput := output.String()
	for _, rule := range []string{"required_version_order", "cloud_order", "cloud_workspaces_order", "required_providers_order"} {
		if !strings.Contains(checkOutput, rule) {
			t.Errorf("check output = %q, want rule %q", checkOutput, rule)
		}
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Fatalf("check changed input: got %q, read error %v", got, readErr)
	}

	output.Reset()
	err = Run([]string{"check", "--chdir", directory, "--diff"}, &output)
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("check --diff error = %v, want exit code 1 for composed ordering", err)
	}
	if got := output.String(); !strings.HasPrefix(got, checkOutput) || !strings.Contains(got, "--- ") || !strings.Contains(got, "+++ ") {
		t.Errorf("check --diff output = %q, want the same change report followed by a unified diff", got)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Fatalf("check --diff changed input: got %q, read error %v", got, readErr)
	}

	output.Reset()
	if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
		t.Fatalf("run error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("run output = %q, read error %v, want %q", got, readErr, want)
	}
	if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
		t.Fatalf("second run error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("second run changed output: got %q, read error %v", got, readErr)
	}
}

func TestCloudAttributeOrderCLI(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  cloud {\n    token = \"dummy-token\"\n    hostname = \"example.test\"\n    workspaces { name = \"example\" }\n    organization = \"example\"\n  }\n}\n"
	want := "terraform {\n  cloud {\n    organization = \"example\"\n    hostname = \"example.test\"\n    token = \"dummy-token\"\n    workspaces { name = \"example\" }\n  }\n}\n"
	writeConfig(t, filename, source)
	var output bytes.Buffer

	err := Run([]string{"check", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("check error = %v, want exit code 1 for pending cloud order", err)
	}
	if !strings.Contains(output.String(), "cloud_order") {
		t.Errorf("check output = %q, want cloud_order report", output.String())
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("check changed input: got %q, read error %v", got, readErr)
	}
	output.Reset()
	err = Run([]string{"check", "--chdir", directory, "--diff"}, &output)
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("check --diff error = %v, want exit code 1", err)
	}
	if !strings.Contains(output.String(), "--- ") || !strings.Contains(output.String(), "+++ ") {
		t.Errorf("check --diff output = %q, want unified diff headers", output.String())
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("check --diff changed input: got %q, read error %v", got, readErr)
	}
	output.Reset()
	if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
		t.Fatalf("run error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("run output = %q, read error %v, want %q", got, readErr, want)
	}
	if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
		t.Fatalf("second run error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("second run changed output: got %q, read error %v", got, readErr)
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
		{"check", "--chdir", directory},
		{"check", "--chdir", directory, "--diff"},
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

	if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
		t.Fatalf("run error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("run output = %q, read error %v, want %q", got, readErr, want)
	}
	if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
		t.Fatalf("second run error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("second run changed output: got %q, read error %v", got, readErr)
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
		{"check", "--chdir", directory},
		{"check", "--chdir", directory, "--diff"},
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

	if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
		t.Fatalf("run error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("run output = %q, read error %v, want %q", got, readErr, want)
	}
	if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
		t.Fatalf("second run error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("second run changed output: got %q, read error %v", got, readErr)
	}
}

func TestCloudAttributeOrderAllowsOmittedToken(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  cloud {\n    workspaces { name = \"example\" }\n    hostname = \"example.test\"\n    organization = \"example\"\n  }\n}\n"
	want := "terraform {\n  cloud {\n    organization = \"example\"\n    hostname = \"example.test\"\n    workspaces { name = \"example\" }\n  }\n}\n"
	writeConfig(t, filename, source)
	var output bytes.Buffer

	err := Run([]string{"check", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("check error = %v, want exit code 1 with omitted token", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("check changed input: got %q, read error %v", got, readErr)
	}
	if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
		t.Fatalf("run error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("run output = %q, read error %v, want %q", got, readErr, want)
	}
}

func TestCloudAttributeOrderCanBeDisabled(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  cloud {\n    token = \"dummy-token\"\n    organization = \"example\"\n  }\n}\n"
	writeConfig(t, filename, source)
	writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = false\ncloud_order = false\nrequired_providers_order = false\n")
	var output bytes.Buffer

	if err := Run([]string{"check", "--chdir", directory}, &output); err != nil {
		t.Fatalf("check error = %v, want nil with cloud_order disabled", err)
	}
	if output.Len() != 0 {
		t.Errorf("check output = %q, want empty output with cloud_order disabled", output.String())
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

	err := Run([]string{"check", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("check error = %v, want exit code 1 with backend_order disabled", err)
	}
	if !strings.Contains(output.String(), "cloud_order") {
		t.Errorf("check output = %q, want cloud_order report", output.String())
	}
}

func TestCloudAttributeOrderPreservesComments(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  cloud {\n    # token owner\n    token = \"dummy-token\"\n    hostname = \"example.test\" # hostname owner\n    # workspace owner\n    workspaces {\n      name = \"example\"\n      # nested comment\n    }\n    organization = \"example\" # organization owner\n  }\n}\n"
	want := "terraform {\n  cloud {\n    organization = \"example\" # organization owner\n    hostname = \"example.test\" # hostname owner\n    # token owner\n    token = \"dummy-token\"\n    # workspace owner\n    workspaces {\n      name = \"example\"\n      # nested comment\n    }\n  }\n}\n"
	writeConfig(t, filename, source)
	var output bytes.Buffer

	err := Run([]string{"check", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("check error = %v, want exit code 1 for cloud order with attached comments", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("check changed input: got %q, read error %v", got, readErr)
	}
	if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
		t.Fatalf("run error = %v, want nil", err)
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
		t.Fatalf("run output = %q, read error %v, want %q", got, readErr, want)
	}
}

func TestCloudAttributeOrderSortsMultipleWorkspacesBlocksStably(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "main.tf")
	source := "terraform {\n  cloud {\n    workspaces { name = \"first\" }\n    organization = \"example\"\n    workspaces { name = \"second\" }\n  }\n}\n"
	writeConfig(t, filename, source)
	var output bytes.Buffer

	err := Run([]string{"check", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("check error = %v, want exit code 1 for ordering changes", err)
	}
	if strings.Contains(output.String(), "warning: skipped") {
		t.Errorf("check output = %q, unexpected semantic warning", output.String())
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("check changed ambiguous input: got %q, read error %v", got, readErr)
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

			err := Run([]string{"check", "--chdir", directory}, &output)
			if test.wantExit == 0 {
				if err != nil {
					t.Fatalf("check error = %v, want nil", err)
				}
			} else {
				var exitError interface{ ExitCode() int }
				if !errors.As(err, &exitError) || exitError.ExitCode() != test.wantExit {
					t.Fatalf("check error = %v, want exit code %d", err, test.wantExit)
				}
				if !strings.Contains(output.String(), test.wantWarning) {
					t.Errorf("check output = %q, want warning containing %q", output.String(), test.wantWarning)
				}
			}
			if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != test.source {
				t.Errorf("check changed input: got %q, read error %v", got, readErr)
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

			err := Run([]string{"check", "--chdir", directory}, &output)
			var exitError interface{ ExitCode() int }
			if test.wantChange {
				if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
					t.Fatalf("check error = %v, want exit code 1 for %s", err, test.name)
				}
				if !strings.Contains(output.String(), "cloud_workspaces_order") {
					t.Errorf("check output = %q, want cloud_workspaces_order report", output.String())
				}
			} else if err != nil || output.Len() != 0 {
				t.Fatalf("check result = (%v, %q), want silent success for %s", err, output.String(), test.name)
			}
			if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != test.source {
				t.Errorf("check changed input: got %q, read error %v", got, readErr)
			}
			output.Reset()
			err = Run([]string{"check", "--chdir", directory, "--diff"}, &output)
			if test.wantChange {
				if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
					t.Fatalf("check --diff error = %v, want exit code 1", err)
				}
				if !strings.Contains(output.String(), "--- ") || !strings.Contains(output.String(), "+++ ") {
					t.Errorf("check --diff output = %q, want unified diff headers", output.String())
				}
			} else if err != nil || output.Len() != 0 {
				t.Fatalf("check --diff result = (%v, %q), want silent success", err, output.String())
			}
			if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != test.source {
				t.Errorf("check --diff changed input: got %q, read error %v", got, readErr)
			}
			output.Reset()
			if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
				t.Fatalf("run error = %v, want nil", err)
			}
			if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != test.want {
				t.Fatalf("run output = %q, read error %v, want %q", got, readErr, test.want)
			}
			if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
				t.Fatalf("second run error = %v, want nil", err)
			}
			if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != test.want {
				t.Fatalf("second run changed output: got %q, read error %v", got, readErr)
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

		if err := Run([]string{"check", "--chdir", directory}, &output); err != nil {
			t.Fatalf("check error = %v, want nil with cloud_workspaces_order disabled", err)
		}
		if output.Len() != 0 {
			t.Errorf("check output = %q, want empty output", output.String())
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

		err := Run([]string{"check", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("check error = %v, want exit code 1 with cloud_order disabled", err)
		}
		if !strings.Contains(output.String(), "cloud_workspaces_order") {
			t.Errorf("check output = %q, want cloud_workspaces_order report", output.String())
		}
	})

	t.Run("disabled rule suppresses name-tags diagnostics", func(t *testing.T) {
		directory := t.TempDir()
		source := "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      name = \"workspace\"\n      tags = [\"app\"]\n    }\n  }\n}\n"
		writeConfig(t, filepath.Join(directory, "main.tf"), source)
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = false\ncloud_order = false\ncloud_workspaces_order = false\nrequired_providers_order = false\n")
		var output bytes.Buffer

		if err := Run([]string{"check", "--chdir", directory}, &output); err != nil {
			t.Fatalf("check error = %v, want nil with rules disabled", err)
		}
		if output.Len() != 0 {
			t.Errorf("check output = %q, want empty output with rules disabled", output.String())
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

			err := Run([]string{"check", "--chdir", directory}, &output)
			switch test.wantExit {
			case 0:
				if err != nil {
					t.Fatalf("check error = %v, want nil", err)
				}
			case 1:
				var exitError interface{ ExitCode() int }
				if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
					t.Fatalf("check error = %v, want exit code 1", err)
				}
			default:
				var exitError interface{ ExitCode() int }
				if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
					t.Fatalf("check error = %v, want exit code 2", err)
				}
				if !strings.Contains(output.String(), test.wantWarning) {
					t.Errorf("check output = %q, want warning containing %q", output.String(), test.wantWarning)
				}
			}
			if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != test.source {
				t.Errorf("check changed input: got %q, read error %v", got, readErr)
			}
		})
	}
}

func TestCloudWorkspacesOrderAllowsDuplicateAlignedBlocks(t *testing.T) {
	directory := t.TempDir()
	source := "terraform {\n  cloud {\n    organization = \"example\"\n    workspaces {\n      name = \"first\"\n    }\n    workspaces {\n      name = \"second\"\n    }\n  }\n}\n"
	writeConfig(t, filepath.Join(directory, "main.tf"), source)
	var output bytes.Buffer

	err := Run([]string{"check", "--chdir", directory}, &output)
	if err != nil || output.Len() != 0 {
		t.Fatalf("check = %v; output = %q, want no value validation diagnostic", err, output.String())
	}
	if got, readErr := os.ReadFile(filepath.Join(directory, "main.tf")); readErr != nil || string(got) != source {
		t.Errorf("check changed duplicate input: got %q, read error %v", got, readErr)
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
	firstErr := Run([]string{"check", "--chdir", directory, "--diff"}, &firstOutput)
	for key, values := range environment {
		t.Setenv(key, values[1])
	}
	var secondOutput bytes.Buffer
	secondErr := Run([]string{"check", "--chdir", directory, "--diff"}, &secondOutput)
	var exitError interface{ ExitCode() int }
	if !errors.As(firstErr, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("first check error = %v, want exit code 1", firstErr)
	}
	if !errors.As(secondErr, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("second check error = %v, want exit code 1", secondErr)
	}
	if firstOutput.String() != secondOutput.String() {
		t.Errorf("check output changed with environment: first %q, second %q", firstOutput.String(), secondOutput.String())
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
		t.Errorf("check changed partial cloud input: got %q, read error %v", got, readErr)
	}
}

func TestRequiredProvidersOrderCLI(t *testing.T) {
	t.Run("check and diff are non-destructive and run is idempotent", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  required_providers {\n    aws = {\n      version = \"~> 5.0\"\n      source = \"hashicorp/aws\"\n    }\n  }\n}\n"
		want := "terraform {\n  required_providers {\n    aws = {\n      source = \"hashicorp/aws\"\n      version = \"~> 5.0\"\n    }\n  }\n}\n"
		writeConfig(t, filename, source)
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = false\nrequired_providers_order = true\n")
		var output bytes.Buffer

		err := Run([]string{"check", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("check error = %v, want exit code 1", err)
		}
		if !strings.Contains(output.String(), "required_providers_order") {
			t.Errorf("check output = %q, want required_providers_order report", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || got == nil || string(got) != source {
			t.Errorf("check changed input: got %q, read error %v", got, readErr)
		}

		output.Reset()
		err = Run([]string{"check", "--chdir", directory, "--diff"}, &output)
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("check --diff error = %v, want exit code 1", err)
		}
		if !strings.Contains(output.String(), "--- ") || !strings.Contains(output.String(), "+++ ") {
			t.Errorf("check --diff output = %q, want unified diff headers", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("check --diff changed input: got %q, read error %v", got, readErr)
		}

		output.Reset()
		if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
			t.Fatalf("run error = %v, want nil", err)
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
			t.Fatalf("run output = %q, read error %v, want %q", got, readErr, want)
		}
		if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
			t.Fatalf("second run error = %v, want nil", err)
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != want {
			t.Fatalf("second run changed output: got %q, read error %v", got, readErr)
		}
	})

	t.Run("disabled rule leaves input unchanged", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  required_providers {\n    aws = {\n      version = \"~> 5.0\"\n      source = \"hashicorp/aws\"\n    }\n  }\n}\n"
		writeConfig(t, filename, source)
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_providers_order = false\n")
		var output bytes.Buffer

		if err := Run([]string{"check", "--chdir", directory}, &output); err != nil {
			t.Fatalf("check with disabled rule error = %v, want nil", err)
		}
		if output.Len() != 0 {
			t.Errorf("check with disabled rule output = %q, want empty", output.String())
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

		err := Run([]string{"check", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
			t.Fatalf("check error = %v, want exit code 2", err)
		}
		if !strings.Contains(output.String(), "provider requirement") || !strings.Contains(output.String(), "warning") {
			t.Errorf("check output = %q, want a provider requirement warning", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("check changed unsafe input: got %q, read error %v", got, readErr)
		}
	})
}

func TestCheckCommand(t *testing.T) {
	t.Run("reports changes without modifying input", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
		writeConfig(t, filename, source)
		var output bytes.Buffer

		err := Run([]string{"check", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("Run() error = %v, want exit code 1 for pending changes", err)
		}
		if !strings.Contains(output.String(), "main.tf:3:3") || !strings.Contains(output.String(), "required_version") {
			t.Errorf("Run() output = %q, want original source location and rule", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("check changed input: got %q, read error %v", got, readErr)
		}
	})

	t.Run("prints unified diff without modifying input", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
		writeConfig(t, filename, source)
		var output bytes.Buffer

		err := Run([]string{"check", "--chdir", directory, "--diff"}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("Run() error = %v, want exit code 1 for pending changes", err)
		}
		if !strings.Contains(output.String(), "--- ") || !strings.Contains(output.String(), "+++ ") {
			t.Errorf("Run() output = %q, want unified diff headers", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("check --diff changed input: got %q, read error %v", got, readErr)
		}
	})

	t.Run("clean file is silent", func(t *testing.T) {
		directory := t.TempDir()
		source := "terraform {\n  required_version = \">= 1.0\"\n}\n"
		writeConfig(t, filepath.Join(directory, "main.tf"), source)
		var output bytes.Buffer

		if err := Run([]string{"check", "--chdir", directory}, &output); err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if output.Len() != 0 {
			t.Errorf("Run() output = %q, want empty output", output.String())
		}
	})

	t.Run("backend order is reported and fixed by run", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  required_version = \">= 1.0\"\n  required_providers {\n    aws = {\n      source = \"hashicorp/aws\"\n    }\n  }\n  backend \"local\" {}\n}\n"
		writeConfig(t, filename, source)
		var output bytes.Buffer

		err := Run([]string{"check", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("Run() error = %v, want exit code 1 for pending backend change", err)
		}
		if !strings.Contains(output.String(), "backend_order") {
			t.Errorf("Run() output = %q, want backend ordering report", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("check changed input: got %q, read error %v", got, readErr)
		}

		if err := Run([]string{"run", "--chdir", directory}, &output); err != nil {
			t.Fatalf("run failed to apply backend order: %v", err)
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != "terraform {\n  required_version = \">= 1.0\"\n  backend \"local\" {}\n  required_providers {\n    aws = {\n      source = \"hashicorp/aws\"\n    }\n  }\n}\n" {
			t.Fatalf("run output mismatch: got %q, read error %v", got, readErr)
		}
	})

	t.Run("backend order still runs when required_version rule is disabled", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  required_version = \">= 1.0\"\n  required_providers {\n    aws = {\n      source = \"hashicorp/aws\"\n    }\n  }\n  backend \"local\" {}\n}\n"
		writeConfig(t, filename, source)
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = true\n")
		var output bytes.Buffer

		err := Run([]string{"check", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			t.Fatalf("Run() error = %v, want exit code 1 for backend ordering while required_version rule is disabled", err)
		}
		if !strings.Contains(output.String(), "backend_order") {
			t.Errorf("Run() output = %q, want backend ordering report even when required_version_order is disabled", output.String())
		}
	})

	t.Run("already ordered with boundary comment is silent", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  required_version = \">= 1.15.0\"\n  # コメント\n}\n"
		writeConfig(t, filename, source)
		var output bytes.Buffer

		if err := Run([]string{"check", "--chdir", directory}, &output); err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if output.Len() != 0 {
			t.Errorf("Run() output = %q, want empty output", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("check changed input: got %q, read error %v", got, readErr)
		}
	})

	t.Run("disabled rule is silent", func(t *testing.T) {
		directory := t.TempDir()
		source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
		writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "required_version_order = false\nbackend_order = false\n")
		writeConfig(t, filepath.Join(directory, "main.tf"), source)
		var output bytes.Buffer

		if err := Run([]string{"check", "--chdir", directory}, &output); err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if output.Len() != 0 {
			t.Errorf("Run() output = %q, want empty output", output.String())
		}
	})

	t.Run("ambiguous comment is reported as skipped", func(t *testing.T) {
		directory := t.TempDir()
		filename := filepath.Join(directory, "main.tf")
		source := "terraform {\n  backend \"local\" {}\n  # independent section comment\n\n  required_version = \">= 1.0\"\n}\n"
		writeConfig(t, filename, source)
		var output bytes.Buffer

		err := Run([]string{"check", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
			t.Fatalf("Run() error = %v, want exit code 2 for skipped transformation", err)
		}
		if !strings.Contains(output.String(), "warning: skipped") {
			t.Errorf("Run() output = %q, want skip warning", output.String())
		}
		if got, readErr := os.ReadFile(filename); readErr != nil || string(got) != source {
			t.Errorf("check changed skipped input: got %q, read error %v", got, readErr)
		}
	})

	t.Run("multiple files are reported in stable order", func(t *testing.T) {
		directory := t.TempDir()
		source := "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
		writeConfig(t, filepath.Join(directory, "b.tf"), source)
		writeConfig(t, filepath.Join(directory, "a.tf"), source)
		var output bytes.Buffer

		err := Run([]string{"check", "--chdir", directory}, &output)
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

		err := Run([]string{"check", "--chdir", directory}, &output)
		var exitError interface{ ExitCode() int }
		if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
			t.Fatalf("Run() error = %v, want exit code 2 for syntax error", err)
		}
	})
}

func TestRunCollectsTargetSyntaxErrorsWithoutModifyingFiles(t *testing.T) {
	directory := t.TempDir()
	files := map[string]string{
		"main.tf":           "locals {\n  value =\n}\n",
		"variables.tf.json": "{\n  \"unknown\": }\n",
	}
	for name, content := range files {
		writeConfig(t, filepath.Join(directory, name), content)
	}

	err := runFromDirectory(t, directory, []string{"check"})
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

		err := runFromDirectory(t, workingDirectory, []string{"check"})
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	})

	t.Run("uses no config when none is found", func(t *testing.T) {
		err := runFromDirectory(t, t.TempDir(), []string{"run"})
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

		err := runFromDirectory(t, child, []string{"check"})
		if err == nil || !strings.Contains(err.Error(), "parse config") {
			t.Fatalf("Run() error = %v, want selected config parse error", err)
		}
	})

	t.Run("explicit config takes precedence", func(t *testing.T) {
		workingDirectory := t.TempDir()
		writeConfig(t, filepath.Join(workingDirectory, ".tfstylus.hcl"), "invalid {")
		explicitConfig := filepath.Join(t.TempDir(), "explicit.hcl")
		writeConfig(t, explicitConfig, "")

		err := runFromDirectory(t, workingDirectory, []string{"check", "--config", explicitConfig})
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	})
}

func TestExplicitConfigMissingFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.hcl")
	var output bytes.Buffer

	err := Run([]string{"check", "-c", configPath}, &output)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Run() error = %v, want an error matching os.ErrNotExist", err)
	}
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("Run() error = %v, want exit code 2 for config error", err)
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
		{name: "short before command", args: []string{"-c", configPath, "check"}},
		{name: "short after command", args: []string{"check", "-c", configPath}},
		{name: "long before command", args: []string{"--config", configPath, "run"}},
		{name: "long after command", args: []string{"run", "--config", configPath}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer

			err := Run(test.args, &output)
			if strings.Contains(strings.Join(test.args, " "), "run") && err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			if !strings.Contains(strings.Join(test.args, " "), "run") && err != nil {
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
		{name: "missing value", args: []string{"check", "-c"}},
		{name: "repeated flag", args: []string{"check", "-c", "first.hcl", "--config", "second.hcl"}},
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

	err := Run([]string{"check", "--config", configPath}, &bytes.Buffer{})
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
		{"defaults", []string{"check"}, options{command: "check", directory: "."}},
		{"diff", []string{"check", "--diff"}, options{command: "check", directory: ".", diff: true}},
		{"before and after", []string{"--chdir", "child", "run", "--recursive", "-c", "settings.hcl"}, options{command: "run", directory: "child", recursive: true, configPath: "settings.hcl"}},
		{"equals", []string{"check", "--recursive=true", "--chdir=child"}, options{command: "check", directory: "child", recursive: true}},
		{"false", []string{"--recursive=false", "check"}, options{command: "check", directory: "."}},
		{"repeated recursive", []string{"check", "--recursive", "--recursive"}, options{command: "check", directory: ".", recursive: true}},
		{"short diff", []string{"check", "-d"}, options{command: "check", directory: ".", diff: true}},
		{"short recursive", []string{"run", "-r"}, options{command: "run", directory: ".", recursive: true}},
		{"short recursive false", []string{"-r=false", "check"}, options{command: "check", directory: "."}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseArgs(test.args)
			if err != nil || got != test.want {
				t.Fatalf("parseArgs() = %#v, %v, want %#v", got, err, test.want)
			}
		})
	}
	for _, args := range [][]string{
		{"check", "--chdir"}, {"--chdir=", "check"}, {"check", "--chdir", ""},
		{"check", "--chdir", "--recursive"}, {"--chdir=a", "check", "--chdir=b"},
		{"check", "--recursive=invalid"}, {"check", "--recursive="}, {"check", "--diff=false"}, {"run", "--diff"},
		{"check", "-d=false"}, {"run", "-d"}, {"check", "-r=invalid"},
		{"check", "--unknown"}, {"check", "child"}, {"check", "run"},
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
		{"check", "--chdir", "child"},
		{"--chdir=child", "run"},
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
		{"recursive child config", []string{"check", "--recursive"}, map[string]string{"child/.tfstylus.hcl": "invalid {"}, "parse config"},
		{"direct skips child", []string{"check"}, map[string]string{"child/.tfstylus.hcl": "invalid {"}, ""},
		{"false skips child", []string{"run", "--recursive=false"}, map[string]string{"child/.tfstylus.hcl": "invalid {"}, ""},
		{"both flags", []string{"--recursive", "run", "--chdir=child"}, map[string]string{"child/grandchild/.tfstylus.hcl": "invalid {"}, "parse config"},
		{"chdir inherits parent", []string{"check", "--chdir=child"}, map[string]string{".tfstylus.hcl": "invalid {"}, "parse config"},
		{"relative explicit after chdir", []string{"check", "--chdir=child", "-c", "explicit.hcl"}, map[string]string{"explicit.hcl": "invalid {", "child/explicit.hcl": ""}, ""},
		{"recursive relative explicit", []string{"check", "--recursive", "--config", "explicit.hcl"}, map[string]string{"explicit.hcl": ""}, "read config"},
		{"recursive absolute explicit", []string{"check", "--recursive", "--config", "{root}/explicit.hcl"}, map[string]string{"explicit.hcl": "", "child/.tfstylus.hcl": "invalid {"}, ""},
		{"hidden child ignored", []string{"check", "--recursive"}, map[string]string{".terraform/.tfstylus.hcl": "invalid {", ".git/.tfstylus.hcl": "invalid {"}, ""},
		{"missing directory", []string{"check", "--chdir=missing"}, nil, "inspect target directory"},
		{"file as directory", []string{"run", "--recursive", "--chdir=main.tf"}, nil, "not a directory"},
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
		err := runFromDirectory(t, root, []string{"check", "--recursive"})
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
		err := runFromDirectory(t, root, []string{"run"})
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

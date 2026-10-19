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

func cleanCommandOutput(command string) string {
	if command == "plan" {
		return "No changes.\n"
	}
	return ""
}

func TestPlanApplyCommandContract(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	directory := t.TempDir()
	var output bytes.Buffer
	if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil || output.String() != "No changes.\n" {
		t.Fatalf("empty plan = %v, %q", err, &output)
	}
	for _, command := range []string{"check", "run"} {
		output.Reset()
		err := Run([]string{command, "--chdir", directory}, &output)
		if err == nil || err.Error() != "unknown command: "+command || output.Len() != 0 {
			t.Fatalf("removed %s = %v, %q", command, err, &output)
		}
	}
	for _, flag := range []string{"-d", "--diff"} {
		for _, args := range [][]string{{"plan", flag}, {flag, "plan"}} {
			if _, err := parseArgs(args); err == nil {
				t.Fatalf("accepted %v", args)
			}
		}
		for _, args := range [][]string{{"apply", flag}, {flag, "apply"}} {
			opts, err := parseArgs(args)
			if err != nil || !opts.diff || opts.command != "apply" {
				t.Fatalf("apply flags %v = %+v, %v", args, opts, err)
			}
		}
	}
}

func TestPlanCleanOutput(t *testing.T) {
	for _, files := range []map[string]string{
		{},
		{"main.tf": "variable \"x\" {\n  type = number\n  default = 1\n}\n"},
		{"main.tf": "variable \"x\" {\n  default = 1\n  type = number\n}\n", ".tfstylus.hcl": "variable_order = false\n"},
		{"main.tf.json": `{"variable":{"x":{"default":1,"type":"number"}}}`},
		{"a.tf": "locals { value = 1 }\n", "child/b.tf": "locals { value = 2 }\n"},
	} {
		t.Run(fmt.Sprint(files), func(t *testing.T) {
			directory := t.TempDir()
			t.Setenv("HOME", t.TempDir())
			for name, content := range files {
				filename := filepath.Join(directory, name)
				if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
					t.Fatal(err)
				}
				writeConfig(t, filename, content)
			}
			var output bytes.Buffer
			if err := Run([]string{"plan", "-r", "--chdir", directory}, &output); err != nil || output.String() != "No changes.\n" {
				t.Fatalf("clean plan = %v, %q", err, &output)
			}
		})
	}
}

func TestPlanFailureDoesNotReportNoChanges(t *testing.T) {
	for _, source := range []string{
		"terraform { invalid = }\n",
		"terraform {\n  backend \"local\" {}\n  # independent\n\n  required_version = \">= 1.0\"\n}\n",
	} {
		for _, mixed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/mixed=%t", source, mixed), func(t *testing.T) {
				directory := t.TempDir()
				t.Setenv("HOME", t.TempDir())
				filename := filepath.Join(directory, "z-invalid.tf")
				writeConfig(t, filename, source)
				if mixed {
					writeConfig(t, filepath.Join(directory, "a-valid.tf"), "variable \"x\" {\n  default = 1\n  type = number\n}\n")
				}
				var output bytes.Buffer
				err := Run([]string{"plan", "--chdir", directory}, &output)
				var exitError interface{ ExitCode() int }
				if !errors.As(err, &exitError) || exitError.ExitCode() != 2 || strings.Contains(output.String(), "No changes.") {
					t.Fatalf("failed plan = %v, %s", err, &output)
				}
				if mixed && !strings.Contains(output.String(), "--- a/a-valid.tf\n") {
					t.Fatalf("missing safe candidate diff: %s", &output)
				}
				if got, err := os.ReadFile(filename); err != nil || string(got) != source {
					t.Fatalf("plan changed invalid input: %q, %v", got, err)
				}
			})
		}
	}
}

func TestPlanOutputFailure(t *testing.T) {
	const changed = "variable \"example\" {\n  default = 1\n  type = number\n}\n"
	const warning = "terraform {\n  backend \"local\" {}\n  # independent\n\n  required_version = \">= 1.0\"\n}\n"
	for _, test := range []struct {
		name   string
		source string
		diff   bool
	}{
		{name: "no changes"},
		{name: "change report", source: changed},
		{name: "diff", source: changed, diff: true},
		{name: "warning", source: warning},
		{name: "rename", source: "locals { value = 1 }\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			t.Setenv("HOME", t.TempDir())
			filename := filepath.Join(directory, "local.tf")
			writeConfig(t, filename, test.source)
			limit := 0
			if test.diff {
				var output bytes.Buffer
				_ = Run([]string{"plan", "--chdir", directory}, &output)
				limit = strings.Index(output.String(), "--- a/local.tf\n")
				if limit < 0 {
					t.Fatalf("missing baseline diff: %s", &output)
				}
			}
			wantErr := errors.New("output unavailable")
			err := Run([]string{"plan", "--chdir", directory}, &planFailWriter{remaining: limit, err: wantErr})
			var exitError interface{ ExitCode() int }
			if !errors.As(err, &exitError) || exitError.ExitCode() != 2 || !errors.Is(err, wantErr) {
				t.Fatalf("output failure = %v; want exit 2 wrapping output error", err)
			}
			if got, err := os.ReadFile(filename); err != nil || string(got) != test.source {
				t.Fatalf("output failure changed input: %q, %v", got, err)
			}
			if _, err := os.Lstat(filepath.Join(directory, "locals.tf")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("output failure renamed input: %v", err)
			}
		})
	}
}

type planFailWriter struct {
	remaining int
	err       error
}

func (w *planFailWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, w.err
	}
	w.remaining -= len(p)
	return len(p), nil
}

func TestPlanApplyDefaultDiff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	directory := t.TempDir()
	var output bytes.Buffer
	filename := filepath.Join(directory, "main.tf")
	source := "variable \"example\" {\n  default = 1\n  type = number\n}\n"
	writeConfig(t, filename, source)
	err := Run([]string{"plan", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("plan = %v, %s", err, &output)
	}
	for _, text := range []string{"variable_order", "--- a/main.tf\n", "+++ b/main.tf\n", "-  type = number", "+  type = number"} {
		if !strings.Contains(output.String(), text) {
			t.Fatalf("plan omitted %q: %s", text, &output)
		}
	}
	wantDiff := "--- a/main.tf\n+++ b/main.tf\n@@ -1,5 +1,5 @@\n variable \"example\" {\n+  type = number\n   default = 1\n-  type = number\n }\n \n"
	if !strings.HasSuffix(output.String(), wantDiff) {
		t.Fatalf("plan diff = %s; want suffix %s", &output, wantDiff)
	}
	if got, err := os.ReadFile(filename); err != nil || string(got) != source {
		t.Fatalf("plan changed input: %q, %v", got, err)
	}
	output.Reset()
	if err := Run([]string{"apply", "--diff", "--chdir", directory}, &output); err != nil || output.String() != wantDiff {
		t.Fatalf("apply diff = %v, %s; want %s", err, &output, wantDiff)
	}
	want := "variable \"example\" {\n  type = number\n  default = 1\n}\n"
	if got, err := os.ReadFile(filename); err != nil || string(got) != want {
		t.Fatalf("apply content = %q, %v; want %q", got, err, want)
	}
	output.Reset()
	if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil || output.Len() != 0 {
		t.Fatalf("apply = %v, %s", err, &output)
	}
	if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil || output.String() != "No changes.\n" {
		t.Fatalf("post-apply plan = %v, %s", err, &output)
	}
}

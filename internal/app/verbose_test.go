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

func TestVerbosePlanApplyPreparationFailures(t *testing.T) {
	for _, command := range []string{"plan", "apply"} {
		for _, stage := range []string{"target discovery", "config discovery", "config loading"} {
			t.Run(command+"/"+stage, func(t *testing.T) {
				directory := t.TempDir()
				path := filepath.Join(directory, "terraform.tf")
				const source = "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
				if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
				target := directory
				config := filepath.Join(directory, ".tfstylus.hcl")
				var flags []string
				var wantError string
				switch stage {
				case "target discovery":
					target = filepath.Join(directory, "missing")
					wantError = fmt.Sprintf("inspect target directory %q:", target)
				case "config discovery":
					if err := os.Mkdir(config, 0o700); err != nil {
						t.Fatal(err)
					}
					wantError = fmt.Sprintf("config %q is not a regular file", config)
				case "config loading":
					flags = []string{"--config", config}
					wantError = fmt.Sprintf("read config %q:", config)
				}
				args := append([]string{command, "--chdir", target, "-v"}, flags...)
				var output, logs bytes.Buffer
				err := RunWithWriters(args, &output, &logs, "dev")
				var exitError interface{ ExitCode() int }
				if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
					t.Fatalf("error = %v; want exit 2", err)
				}
				if !strings.HasPrefix(err.Error(), wantError) {
					t.Fatalf("error = %q; want prefix %q", err, wantError)
				}
				if output.Len() != 0 {
					t.Fatalf("unexpected command output: %q", &output)
				}
				wantLogs := fmt.Sprintf("[INFO] starting %s\n", command)
				if stage != "target discovery" {
					wantLogs += "[INFO] found 1 directories and 1 files\n"
				}
				wantLogs += fmt.Sprintf("[INFO] %s finished with a nonzero status\n", command)
				if logs.String() != wantLogs {
					t.Fatalf("logs = %q; want %q", &logs, wantLogs)
				}
				if got, err := os.ReadFile(path); err != nil || string(got) != source {
					t.Fatalf("input changed: %q, %v", got, err)
				}
			})
		}
	}
}

func TestVerboseArguments(t *testing.T) {
	for _, test := range []struct {
		args []string
		want int
	}{
		{[]string{"plan"}, 0},
		{[]string{"-v", "plan"}, 1},
		{[]string{"plan", "--verbose"}, 1},
		{[]string{"-vv", "plan"}, 2},
		{[]string{"-v", "plan", "--verbose"}, 2},
		{[]string{"--verbose", "--verbose", "plan"}, 2},
		{[]string{"plan", "-vvvv"}, 3},
	} {
		opts, err := parseArgs(test.args)
		if err != nil {
			t.Fatal(err)
		}
		if opts.verbosity != test.want {
			t.Errorf("%v: got %d, want %d", test.args, opts.verbosity, test.want)
		}
	}
	for _, arg := range []string{"-v=true", "--verbose=2", "-vvr"} {
		if _, err := parseArgs([]string{"plan", arg}); err == nil {
			t.Errorf("accepted %s", arg)
		}
	}
}

func TestVerbosePreservesOutput(t *testing.T) {
	directory := t.TempDir()
	for _, command := range []string{"plan", "apply", "version", "init"} {
		// init is checked on an existing configuration so repeated calls are identical.
		if command == "init" {
			if err := Run([]string{"init", "--chdir", directory}, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
		}
		args := []string{command, "--chdir", directory}
		var baseline bytes.Buffer
		baselineErr := Run(args, &baseline)
		for level := 0; level <= 4; level++ {
			var output, logs bytes.Buffer
			verboseArgs := append([]string{}, args...)
			if level > 0 {
				verboseArgs = append(verboseArgs, "-"+strings.Repeat("v", level))
			}
			err := RunWithWriters(verboseArgs, &output, &logs, "dev")
			if fmt.Sprint(err) != fmt.Sprint(baselineErr) || output.String() != baseline.String() {
				t.Fatalf("%s level %d: output=%q err=%v", command, level, &output, err)
			}
			for threshold, label := range []string{"INFO", "DEBUG", "TRACE"} {
				if command != "plan" && command != "apply" && threshold > 0 {
					continue
				}
				if got := strings.Contains(logs.String(), label); got != (level > threshold) {
					t.Errorf("%s level %d: logs=%q label=%s", command, level, &logs, label)
				}
			}
		}
	}
}

func TestVerbosePreservesDiffAndFiles(t *testing.T) {
	const source = "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n"
	for _, command := range []string{"plan", "apply"} {
		var baselineOutput, baselineFile string
		var baselineErr string
		for level := 0; level <= 3; level++ {
			directory := t.TempDir()
			path := filepath.Join(directory, "terraform.tf")
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(directory, ".tfstylus.hcl")
			if err := os.WriteFile(config, []byte("order_required_version = true\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{command, "--chdir", directory, "-c", config}
			if command == "apply" {
				args = append(args, "-d")
			}
			if level > 0 {
				args = append(args, "-"+strings.Repeat("v", level))
			}
			var output, logs bytes.Buffer
			err := RunWithWriters(args, &output, &logs, "dev")
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if level == 0 {
				baselineOutput, baselineFile, baselineErr = output.String(), string(content), fmt.Sprint(err)
				if !strings.Contains(baselineOutput, "@@") {
					t.Fatalf("missing diff: %s (%v)", baselineOutput, err)
				}
			} else if output.String() != baselineOutput || string(content) != baselineFile || fmt.Sprint(err) != baselineErr {
				t.Fatalf("%s level %d changed output, file or error", command, level)
			}
			if command == "plan" && string(content) != source {
				t.Fatal("plan modified input")
			}
			if command == "apply" {
				output.Reset()
				if err := RunWithWriters(args, &output, &logs, "dev"); err != nil || output.Len() != 0 {
					t.Fatalf("not idempotent: %q %v", &output, err)
				}
			}
		}
	}
}

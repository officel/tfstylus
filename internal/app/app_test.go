package app

import (
	"bytes"
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
		{name: "help", args: []string{"--help"}, want: "Usage: tfstylus <command> [flags]\n"},
		{name: "short help", args: []string{"-h"}, want: "Usage: tfstylus <command> [flags]\n"},
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

func TestKnownCommandNotImplemented(t *testing.T) {
	for _, command := range []string{"check", "run"} {
		t.Run(command, func(t *testing.T) {
			var output bytes.Buffer

			err := Run([]string{command}, &output)
			if err == nil {
				t.Fatal("Run() error = nil, want not-implemented error")
			}
			if got, want := err.Error(), "commands are not implemented yet"; got != want {
				t.Errorf("Run() error = %q, want %q", got, want)
			}
			if got := output.String(); got != "" {
				t.Errorf("Run() output = %q, want empty output", got)
			}
		})
	}
}

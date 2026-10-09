package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{name: "empty file"},
		{name: "whitespace", content: "\n  \t\n"},
		{name: "invalid syntax", content: "invalid {", wantErr: "parse config"},
		{name: "unknown attribute", content: "unknown = true", wantErr: "validate config"},
		{name: "unknown block", content: "unknown {}", wantErr: "validate config"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.hcl")
			if err := os.WriteFile(configPath, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}

			_, err := loadConfig(configPath)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("loadConfig() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("loadConfig() error = %v, want error containing %q", err, test.wantErr)
			}
		})
	}
}

func TestLoadConfigFileErrors(t *testing.T) {
	directory := t.TempDir()
	missingPath := filepath.Join(directory, "missing.hcl")

	if _, err := loadConfig(missingPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("loadConfig() error = %v, want an error matching os.ErrNotExist", err)
	}
	if _, err := loadConfig(directory); err == nil {
		t.Error("loadConfig() error = nil for a directory, want read error")
	}
}

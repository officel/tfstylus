package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestInitCreatesDisabledConfig(t *testing.T) {
	directory := t.TempDir()
	t.Chdir(directory)
	var output bytes.Buffer
	if err := Run([]string{"init"}, &output); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, ".tfstylus.hcl")
	config, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if config != (Config{}) {
		t.Fatalf("generated config enables rules: %+v", config)
	}
	if !strings.Contains(output.String(), "Created") || !strings.Contains(output.String(), ".tfstylus.hcl") {
		t.Fatalf("missing creation message: %q", &output)
	}
	source := "variable \"example\" {\n  default = null\n  type = string\n}\nlocals {\n  example = true\n}\n"
	writeConfig(t, filepath.Join(directory, "local.tf"), source)
	for _, command := range []string{"check", "run"} {
		output.Reset()
		if err := Run([]string{command}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("%s = %v, output = %q", command, err, &output)
		}
		if got, err := os.ReadFile(filepath.Join(directory, "local.tf")); err != nil || string(got) != source {
			t.Fatalf("%s changed input: %q, %v", command, got, err)
		}
	}
}

func TestInitExistingConfig(t *testing.T) {
	for _, name := range []string{".tfstylus.hcl", ".config/.tfstylus.hcl", "custom.hcl"} {
		for _, kind := range []string{"file", "directory", "symlink", "dangling symlink"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				directory := t.TempDir()
				path := filepath.Join(directory, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				const original = "invalid HCL must not be read\n"
				switch kind {
				case "file":
					writeConfig(t, path, original)
				case "directory":
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
				default:
					target := filepath.Join(directory, "target.hcl")
					if kind == "symlink" {
						writeConfig(t, target, original)
					}
					if err := os.Symlink(target, path); err != nil {
						t.Fatal(err)
					}
				}
				before, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				args := []string{"init", "--chdir", directory}
				if name == "custom.hcl" {
					args = append(args, "-c", name)
				}
				var output bytes.Buffer
				if err := Run(args, &output); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(output.String(), "already exists") || !strings.Contains(output.String(), path) {
					t.Fatalf("missing existing config message: %q", &output)
				}
				after, err := os.Lstat(path)
				if err != nil || !os.SameFile(before, after) {
					t.Fatalf("existing path was replaced: %v", err)
				}
				if kind == "file" || kind == "symlink" {
					if got, err := os.ReadFile(path); err != nil || string(got) != original {
						t.Fatalf("existing content changed: %q, %v", got, err)
					}
				}
				if name != ".tfstylus.hcl" {
					if _, err := os.Lstat(filepath.Join(directory, ".tfstylus.hcl")); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("unexpected default config: %v", err)
					}
				}
			})
		}
	}
}

func TestInitExplicitConfig(t *testing.T) {
	for _, flag := range []string{"-c", "--config", "-c=", "--config="} {
		t.Run(flag, func(t *testing.T) {
			directory := t.TempDir()
			configDirectory := filepath.Join(directory, ".config")
			if err := os.Mkdir(configDirectory, 0o700); err != nil {
				t.Fatal(err)
			}
			writeConfig(t, filepath.Join(directory, ".tfstylus.hcl"), "keep default\n")
			writeConfig(t, filepath.Join(configDirectory, ".tfstylus.hcl"), "keep alternate\n")
			path := filepath.Join(".config", "custom.hcl")
			if flag == "--config=" {
				path = filepath.Join(configDirectory, "custom.hcl")
			}
			args := []string{"--chdir", directory}
			if strings.HasSuffix(flag, "=") {
				args = append(args, flag+path)
			} else {
				args = append(args, flag, path)
			}
			args = append(args, "init")
			if err := Run(args, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			config, err := loadConfig(filepath.Join(configDirectory, "custom.hcl"))
			if err != nil || config != (Config{}) {
				t.Fatalf("config = %+v, error = %v", config, err)
			}
			for path, want := range map[string]string{
				filepath.Join(directory, ".tfstylus.hcl"):       "keep default\n",
				filepath.Join(configDirectory, ".tfstylus.hcl"): "keep alternate\n",
			} {
				if got, err := os.ReadFile(path); err != nil || string(got) != want {
					t.Fatalf("changed %s: %q, %v", path, got, err)
				}
			}
		})
	}
}

func TestInitDoesNotSearchParentsOrHome(t *testing.T) {
	parent := t.TempDir()
	t.Setenv("HOME", parent)
	writeConfig(t, filepath.Join(parent, ".tfstylus.hcl"), "invalid parent config\n")
	child := filepath.Join(parent, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, filepath.Join(child, "main.tf"), "invalid Terraform\n")
	if err := Run([]string{"init", "--chdir", child}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if config, err := loadConfig(filepath.Join(child, ".tfstylus.hcl")); err != nil || config != (Config{}) {
		t.Fatalf("config = %+v, error = %v", config, err)
	}
}

func TestInitPathErrors(t *testing.T) {
	directory := t.TempDir()
	writeConfig(t, filepath.Join(directory, "file"), "")
	for _, path := range []string{"missing/config.hcl", "file/config.hcl"} {
		t.Run(path, func(t *testing.T) {
			var output bytes.Buffer
			err := Run([]string{"init", "--chdir", directory, "-c", path}, &output)
			var exitError interface{ ExitCode() int }
			if !errors.As(err, &exitError) || exitError.ExitCode() != 2 || output.Len() != 0 {
				t.Fatalf("error = %v, output = %q; want exit 2 without success message", err, &output)
			}
		})
	}
}

func TestInitConcurrentCreation(t *testing.T) {
	directory := t.TempDir()
	var workers sync.WaitGroup
	outputs := make([]bytes.Buffer, 8)
	errs := make([]error, len(outputs))
	for index := range outputs {
		workers.Go(func() {
			errs[index] = Run([]string{"init", "--chdir", directory}, &outputs[index])
		})
	}
	workers.Wait()
	created := 0
	for index, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case strings.Contains(outputs[index].String(), "Created"):
			created++
		case strings.Contains(outputs[index].String(), "already exists"):
		default:
			t.Fatalf("unexpected message: %q", &outputs[index])
		}
	}
	if created != 1 {
		t.Fatalf("created %d times; want 1", created)
	}
	if config, err := loadConfig(filepath.Join(directory, ".tfstylus.hcl")); err != nil || config != (Config{}) {
		t.Fatalf("config = %+v, error = %v", config, err)
	}
}

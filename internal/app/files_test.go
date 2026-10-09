package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTargetFiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"z.tf", "a.tf", "override.tf", "test_override.tf", "override.tf.json", "test_override.tf.json", "main.tf.json",
		".hidden.tf", "main.tf~", "#main.tf#", "terraform.tfvars", "main.tfvars.json", "README.md", "main.TF",
		"child/main.tf", "child/grandchild/main.tf.json", "modules/local/main.tf",
		".terraform/main.tf", ".git/main.tf", "child/.hidden/main.tf",
	} {
		writeConfig(t, filepath.Join(root, name), "content: "+name)
	}
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "directory.tf"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "child.tf"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, destination := range map[string]string{
		"link.tf": "a.tf", "directory-link.tf": "child", "cycle": ".", "external": t.TempDir(), "irrelevant-link": "missing",
	} {
		if err := os.Symlink(destination, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	rootFiles := []string{"a.tf", "link.tf", "main.tf.json", "override.tf", "override.tf.json", "test_override.tf", "test_override.tf.json", "z.tf"}
	for _, recursive := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "recursive"}[recursive], func(t *testing.T) {
			want := []targetDirectory{{directory: root, files: absolutePaths(root, rootFiles)}}
			if recursive {
				for _, entry := range []struct {
					directory string
					files     []string
				}{
					{"child", []string{"main.tf"}},
					{"child.tf", nil},
					{"child/grandchild", []string{"main.tf.json"}},
					{"directory.tf", nil},
					{"empty", nil},
					{"modules", nil},
					{"modules/local", []string{"main.tf"}},
				} {
					directory := filepath.Join(root, entry.directory)
					want = append(want, targetDirectory{directory: directory, files: absolutePaths(directory, entry.files)})
				}
			}
			got, err := discoverTargetFiles(root, recursive)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("discoverTargetFiles() = %#v, %v, want %#v", got, err, want)
			}
			again, err := discoverTargetFiles(root, recursive)
			if err != nil || !reflect.DeepEqual(again, got) {
				t.Fatalf("repeated discovery = %#v, %v, want %#v", again, err, got)
			}
		})
	}
}

func TestTargetFilesEmpty(t *testing.T) {
	root := t.TempDir()
	for _, recursive := range []bool{false, true} {
		got, err := discoverTargetFiles(root, recursive)
		want := []targetDirectory{{directory: root}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("discoverTargetFiles() = %#v, %v, want %#v", got, err, want)
		}
	}
}

func TestTargetFilesErrors(t *testing.T) {
	for _, recursive := range []bool{false, true} {
		for _, kind := range []string{"missing", "file", "broken link"} {
			t.Run(kind+map[bool]string{false: " direct", true: " recursive"}[recursive], func(t *testing.T) {
				root := t.TempDir()
				directory := root
				switch kind {
				case "missing":
					directory = filepath.Join(root, "missing")
				case "file":
					directory = filepath.Join(root, "main.tf")
					writeConfig(t, directory, "")
				case "broken link":
					writeConfig(t, filepath.Join(root, "a.tf"), "")
					if err := os.Symlink("missing", filepath.Join(root, "broken.tf")); err != nil {
						t.Fatal(err)
					}
				}
				got, err := discoverTargetFiles(directory, recursive)
				if err == nil || got != nil || !strings.Contains(err.Error(), directory) {
					t.Fatalf("discoverTargetFiles() = %#v, %v, want no partial results and path error", got, err)
				}
				if kind != "file" && !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("error = %v, want os.ErrNotExist", err)
				}
			})
		}
	}
}

func TestTargetFilesUnreadableDirectory(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(blocked, 0o700); err != nil {
			t.Error(err)
		}
	})
	if _, err := os.ReadDir(blocked); err == nil {
		t.Skip("process can read directories without read permission")
	}
	for _, test := range []struct {
		directory string
		recursive bool
	}{{blocked, false}, {root, true}} {
		got, err := discoverTargetFiles(test.directory, test.recursive)
		if got != nil || !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), blocked) {
			t.Fatalf("discoverTargetFiles() = %#v, %v, want permission error without partial results", got, err)
		}
	}
}

func TestTargetFilesRootExclusions(t *testing.T) {
	root := t.TempDir()
	hidden := filepath.Join(root, ".hidden")
	writeConfig(t, filepath.Join(hidden, "main.tf"), "")
	link := filepath.Join(root, "link")
	if err := os.Symlink(hidden, link); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{hidden, link} {
		got, err := discoverTargetFiles(directory, true)
		if err != nil || len(got) != 0 {
			t.Fatalf("recursive discovery of %q = %#v, %v, want empty result", directory, got, err)
		}
		got, err = discoverTargetFiles(directory, false)
		if err != nil || len(got) != 1 || !reflect.DeepEqual(got[0].files, []string{filepath.Join(directory, "main.tf")}) {
			t.Fatalf("direct discovery of %q = %#v, %v, want main.tf", directory, got, err)
		}
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(hidden); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Error(err)
		}
	})
	got, err := discoverTargetFiles(".", true)
	if err != nil || len(got) != 1 || got[0].directory != hidden {
		t.Fatalf("discovery of current hidden directory = %#v, %v, want %q", got, err, hidden)
	}
}

func TestTargetFilesReadOnly(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, filepath.Join(root, ".tfstylus.hcl"), "")
	writeConfig(t, filepath.Join(root, "main.tf"), "locals { value = 1 }\n")
	writeConfig(t, filepath.Join(root, "child", "main.tf.json"), "{}\n")
	writeConfig(t, filepath.Join(root, ".terraform", "ignored.tf"), "unchanged\n")
	if err := os.Symlink("main.tf", filepath.Join(root, "link.tf")); err != nil {
		t.Fatal(err)
	}
	type fileState struct {
		mode     os.FileMode
		modified time.Time
		content  string
	}
	snapshot := func() map[string]fileState {
		states := make(map[string]fileState)
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			state := fileState{mode: info.Mode(), modified: info.ModTime()}
			if info.Mode().IsRegular() {
				source, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				state.content = string(source)
			} else if info.Mode()&os.ModeSymlink != 0 {
				state.content, err = os.Readlink(path)
				if err != nil {
					return err
				}
			}
			states[path] = state
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return states
	}
	before := snapshot()
	for _, recursive := range []bool{false, true} {
		if _, err := discoverTargetFiles(root, recursive); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range []string{"check", "run"} {
		err := runFromDirectory(t, root, []string{command, "--recursive"})
		if command == "check" && err != nil {
			t.Fatalf("Run(check) error = %v, want nil", err)
		}
		if command == "run" && (err == nil || err.Error() != "commands are not implemented yet") {
			t.Fatalf("Run(run) error = %v, want command-not-implemented error", err)
		}
	}
	if after := snapshot(); !reflect.DeepEqual(after, before) {
		t.Fatalf("discovery changed files: before = %#v, after = %#v", before, after)
	}
}

func absolutePaths(directory string, names []string) []string {
	var paths []string
	for _, name := range names {
		paths = append(paths, filepath.Join(directory, name))
	}
	return paths
}

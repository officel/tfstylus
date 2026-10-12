package app

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type targetDirectory struct {
	directory string
	files     []string
}

func discoverTargetFiles(directory string, recursive bool) ([]targetDirectory, error) {
	info, err := os.Stat(directory)
	if err != nil {
		return nil, fmt.Errorf("inspect target directory %q: %w", directory, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("target %q is not a directory", directory)
	}

	var targets []targetDirectory
	addDirectory := func(path string) error {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve target directory %q: %w", path, err)
		}
		files, err := targetFilesInDirectory(absolute)
		if err != nil {
			return err
		}
		targets = append(targets, targetDirectory{directory: absolute, files: files})
		return nil
	}
	if recursive {
		err = filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				return nil
			}
			if path != "." && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return addDirectory(path)
		})
	} else {
		err = addDirectory(directory)
	}
	if err != nil {
		return nil, fmt.Errorf("discover targets in %q: %w", directory, err)
	}
	sort.Slice(targets, func(first, second int) bool {
		return targets[first].directory < targets[second].directory
	})
	return targets, nil
}

func targetFilesInDirectory(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read target directory %q: %w", directory, err)
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") ||
			(strings.HasPrefix(name, "#") && strings.HasSuffix(name, "#")) {
			continue
		}
		if !strings.HasSuffix(name, ".tf") && !strings.HasSuffix(name, ".tf.json") && !strings.HasSuffix(name, ".tfquery.hcl") {
			continue
		}
		path := filepath.Join(directory, name)
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect target file %q: %w", path, err)
		}
		if info.Mode().IsRegular() {
			files = append(files, path)
		}
	}
	return files, nil
}

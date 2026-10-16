package app

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

//go:embed init.hcl
var initConfigTemplate string

func runInit(opts options, output io.Writer) error {
	path := opts.configPath
	if path == "" {
		candidates := []string{
			filepath.Join(opts.directory, ".tfstylus.hcl"),
			filepath.Join(opts.directory, ".config", ".tfstylus.hcl"),
		}
		for _, candidate := range candidates {
			if _, err := os.Lstat(candidate); err == nil {
				_, err = fmt.Fprintf(output, "Configuration already exists: %s\n", candidate)
				return err
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("inspect config %q: %w", candidate, err)
			}
		}
		path = candidates[0]
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(opts.directory, path)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		_, err = fmt.Fprintf(output, "Configuration already exists: %s\n", path)
		return err
	}
	if err != nil {
		return fmt.Errorf("create config %q: %w", path, err)
	}
	written, writeErr := file.WriteString(initConfigTemplate)
	if writeErr == nil && written != len(initConfigTemplate) {
		writeErr = io.ErrShortWrite
	}
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return fmt.Errorf("write config %q: %w", path, errors.Join(err, os.Remove(path)))
	}
	_, err = fmt.Fprintf(output, "Created configuration: %s\n", path)
	return err
}

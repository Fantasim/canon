package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fantasim/canonlang/internal/project"
)

// writeFile writes rel (project-relative) under out, canonicalising a `.canon` file first.
func writeFile(out, rel string, content []byte) error {
	if strings.HasSuffix(rel, project.SourceExt) {
		formatted, err := formatCanon(rel, content)
		if err != nil {
			return err
		}
		content = formatted
	}
	path := filepath.Join(out, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("%w: %w", errWrite, err)
	}
	if len(content) == 0 || content[len(content)-1] != '\n' {
		content = append(content, '\n')
	}
	if err := os.WriteFile(path, content, filePerm); err != nil {
		return fmt.Errorf("%w: %w", errWrite, err)
	}
	return nil
}

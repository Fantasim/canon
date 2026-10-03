package fixture

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
)

// CopyRenames writes the renames example, with what it imports, from the examples directory
// into dir; expected/ and the README stay behind.
func CopyRenames(dir, examplesDir string) error {
	if err := write(dir, project.FileName, []byte(renamesProject)); err != nil {
		return err
	}
	for _, src := range renamesSources {
		if err := copyTree(os.DirFS(examplesDir), src, dir); err != nil {
			return err
		}
	}
	return nil
}

// copyTree copies the .canon and .json files of src in fsys, but expected/, under dir.
func copyTree(fsys fs.FS, src, dir string) error {
	err := fs.WalkDir(fsys, src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: %s: %w", errWalk, p, err)
		}
		if d.IsDir() && d.Name() == skippedDir {
			return fs.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, project.SourceExt) && !strings.HasSuffix(p, ir.JSONExt) {
			return nil
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", errRead, p, err)
		}
		return write(dir, p, data)
	})
	if err != nil {
		return fmt.Errorf("%w: %s: %w", errCopy, src, err)
	}
	return nil
}

// write writes data at the slash path rel under dir, its directories made.
func write(dir, rel string, data []byte) error {
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), dirMode); err != nil {
		return fmt.Errorf("%w: %s: %w", errWrite, rel, err)
	}
	if err := os.WriteFile(p, data, fileMode); err != nil {
		return fmt.Errorf("%w: %s: %w", errWrite, rel, err)
	}
	return nil
}

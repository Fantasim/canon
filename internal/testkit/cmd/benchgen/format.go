package main

import (
	"fmt"
	"path/filepath"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// formatCanon returns rel's canonical layout (Duration units, wrapped lists, bound text): the
// Edit benchmark must measure canonical files, not whatever schema.go happened to spell.
func formatCanon(rel string, content []byte) ([]byte, error) {
	var fs source.FileSet
	f, err := fs.Add(rel, rel, content)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errWrite, err)
	}
	kind := syntax.FileSource
	if filepath.Base(rel) == project.FileName {
		kind = syntax.FileProject
	}
	out, err := format.Source(f, kind, diag.NewBag(&fs, ""))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", errWrite, rel, err)
	}
	return out, nil
}

// formatJSON returns rel's canonical JSON-source layout (WIRE.md §7.4, FORMATTER.md FMT-02).
func formatJSON(rel string, content []byte) ([]byte, error) {
	var fs source.FileSet
	f, err := fs.Add(rel, rel, content)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errWrite, err)
	}
	root, err := jsonsrc.Parse(f, diag.NewBag(&fs, ""))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", errWrite, rel, err)
	}
	return jsonsrc.Format(root), nil
}

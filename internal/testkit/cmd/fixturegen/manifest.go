package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// row is one fixture of the manifest.
type row struct {
	path   string
	method string
	names  []string
}

// parseManifest reads the rows: known methods, clean relative paths, at least one name, and
// paths in strictly increasing byte order, so the output never depends on the file's order.
func parseManifest(text string) ([]row, error) {
	var rows []row
	for i, line := range strings.Split(text, lineBreak) {
		if line == "" || strings.HasPrefix(line, commentPrefix) {
			continue
		}
		fields := strings.Split(line, fieldSep)
		if len(fields) != manifestFields {
			return nil, fmt.Errorf("%w: line %d: %d fields", errManifest, i+1, len(fields))
		}
		r := row{path: fields[fieldPath], method: fields[fieldMethod], names: strings.Fields(fields[fieldNames])}
		_, known := methods[r.method]
		clean := path.Clean(r.path) == r.path && fs.ValidPath(r.path) && r.path != "."
		sorted := len(rows) == 0 || rows[len(rows)-1].path < r.path
		if !known || !clean || !sorted || len(r.names) == 0 {
			return nil, fmt.Errorf("%w: line %d: %s", errManifest, i+1, line)
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// checkBudget sums the fixture tree as it will be: the files the manifest does not produce,
// plus the new outputs.
func checkBudget(out string, rows []row, outputs [][]byte) error {
	produced := map[string]bool{}
	total := int64(0)
	for i, r := range rows {
		produced[r.path] = true
		total += int64(len(outputs[i]))
	}
	err := filepath.WalkDir(out, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(out, p)
		if err != nil {
			return fmt.Errorf("budget: %w", err)
		}
		if produced[filepath.ToSlash(rel)] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("budget: %w", err)
		}
		total += info.Size()
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("budget: %w", err)
	}
	if total > maxFixtureBytes {
		return fmt.Errorf("%w: %d bytes, at most %d", errBudget, total, maxFixtureBytes)
	}
	return nil
}

package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// CheckRuntime checks the signalling calls of every file of fsys in a runtime directory.
func (c *Catalog) CheckRuntime(fsys fs.FS) error {
	allowed := map[RuntimeText]bool{}
	for _, r := range c.Runtime {
		allowed[r] = true
	}
	var errs []error
	walkErr := fs.WalkDir(fsys, walkRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || path.Base(path.Dir(p)) != runtimeDir {
			return nil
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", errRuntime, p, err)
		}
		errs = append(errs, checkRuntimeText(p, string(data), allowed))
		return nil
	})
	return errors.Join(append(errs, walkErr)...)
}

// checkRuntimeText checks every signalling call of one helper text, line by line.
func checkRuntimeText(name, text string, allowed map[RuntimeText]bool) error {
	var errs []error
	for i, line := range strings.Split(text, lineBreak) {
		for _, loc := range reRuntimeCallStart.FindAllStringIndex(line, allMatches) {
			m := reRuntimeCall.FindStringSubmatch(line[loc[0]:])
			if m == nil {
				errs = append(errs, fmt.Errorf("%w: %s:%d: a signalling call must pass a code and a text literal", errRuntime, name, i+1))
				continue
			}
			pair := RuntimeText{Code: m[runtimeCodeGroup], Text: m[runtimeTextGroup]}
			if !allowed[pair] {
				errs = append(errs, fmt.Errorf("%w: %s:%d: (%s, %q) is not a pair of ERRORS.md §1.6", errRuntime, name, i+1, pair.Code, pair.Text))
			}
		}
	}
	return errors.Join(errs...)
}

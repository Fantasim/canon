package project

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// Find is dir or its nearest parent holding project.canon, else E1003 and ErrNoProject (CLI.md §2.1).
func Find(fsys FS, dir string, bag *diag.Bag) (string, error) {
	start := path.Clean(dir)
	for d := start; ; {
		found, err := has(fsys, d)
		if err != nil {
			return "", err
		}
		if found {
			return d, nil
		}
		up := path.Dir(d)
		if up == d || up == currentSeg {
			break
		}
		d = up
	}
	diag.E1003.At(source.Span{}, start).Report(bag)
	return "", ErrNoProject
}

// Require is has, reporting E1003 and returning ErrNoProject when dir holds no project.canon.
func Require(fsys FS, dir string, bag *diag.Bag) error {
	found, err := has(fsys, dir)
	if err != nil {
		return err
	}
	if !found {
		diag.E1003.At(source.Span{}, dir).Report(bag)
		return ErrNoProject
	}
	return nil
}

// has reports a file named exactly project.canon in dir, letter case included (IMPLEMENTATION-PLAN.md §10).
func has(fsys FS, dir string) (bool, error) {
	entries, err := fsys.ReadDir(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, fmt.Errorf(fmtWrap, err)
	}
	return slices.ContainsFunc(entries, func(e fs.DirEntry) bool { return e.Name() == FileName && !e.IsDir() }), nil
}

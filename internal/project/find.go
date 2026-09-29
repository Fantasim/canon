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
	start := onVolume(path.Clean, dir)
	for d := start; ; {
		found, err := has(fsys, d)
		if err != nil {
			return "", err
		}
		if found {
			return d, nil
		}
		up := parent(d)
		if up == d || up == currentSeg {
			break
		}
		d = up
	}
	diag.E1003.At(source.Span{}, start).Report(bag)
	return "", ErrNoProject
}

// parent is the directory above d, path.Dir applied after d's volume (onVolume).
func parent(d string) string {
	return onVolume(path.Dir, d)
}

// onVolume applies op after name's volume (volumeOf): path.Clean and path.Dir alone turn the
// Windows top "C:/" into the drive-relative "C:", the process's working directory on that
// drive, and "//host/share/x" into "/host/share/x".
func onVolume(op func(string) string, name string) string {
	vol := volumeOf(name)
	return vol + op(name[len(vol):])
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

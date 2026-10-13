package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/writelock"
)

// write replaces each changed file atomically under its packages' writer locks, refusing all if one changed since fmt read it (CLI.md §2.5, §3.6, DECISIONS 351, 352).
func (r *fmtRun) write() error {
	if len(r.changes) == 0 {
		return nil
	}
	held, err := writelock.Acquire(r.inv.ctx, filepath.FromSlash(r.root), r.writers())
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	defer func() { _ = held.Release() }() // closing the lock files: the writes are done or refused
	for _, c := range r.changes {
		if err := r.unchanged(c); err != nil {
			return err
		}
	}
	staging, err := build.NewStaging()
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	for _, c := range r.changes {
		if err := r.inv.ctx.Err(); err != nil {
			return err
		}
		if err := staging.WriteAtomic(r.fsys, c.abs, c.updated); err != nil {
			return fmt.Errorf(fmtArgs, c.display, writeCause(err))
		}
	}
	return nil
}

// writers are the exclusive writer locks of the packages the changes rewrite: a source's package
// line, a JSON source's readers; project.canon and a file of no package take none.
func (r *fmtRun) writers() []writelock.Request {
	names := map[string]bool{}
	for _, c := range r.changes {
		for _, pkg := range c.readers {
			names[pkg] = true
		}
		if pkg := packageLine(c); pkg != "" {
			names[pkg] = true
		}
	}
	reqs := make([]writelock.Request, 0, len(names))
	for _, name := range slices.Sorted(maps.Keys(names)) {
		reqs = append(reqs, writelock.Request{Package: name, Mode: writelock.Exclusive})
	}
	return reqs
}

// packageLine is the package a changed .canon source names, read from the bytes fmt formatted;
// "" for a JSON source, project.canon or a file with no package line.
func packageLine(c fmtChange) string {
	if c.readers != nil {
		return ""
	}
	var set source.FileSet
	src, err := set.Add(c.display, c.abs, c.old)
	if err != nil {
		return ""
	}
	f := syntax.Parse(src, syntax.FileSource, diag.NewBag(&set, ""))
	if f.Package == nil {
		return ""
	}
	return syntax.Qualified(f.Package)
}

// unchanged refuses a file whose bytes are no longer those fmt formatted.
func (r *fmtRun) unchanged(c fmtChange) error {
	data, err := r.fsys.ReadFile(c.abs)
	if err != nil {
		return fmt.Errorf(fmtArgs, c.display, readCause(err))
	}
	if !bytes.Equal(data, c.old) {
		return fmt.Errorf(fmtArgs, c.display, errChangedMeanwhile)
	}
	return nil
}

// writeCause is the fixed cause of a write that failed, chosen by errors.Is.
func writeCause(err error) error {
	if errors.Is(err, fs.ErrPermission) {
		return errDenied
	}
	return errUnwritable
}

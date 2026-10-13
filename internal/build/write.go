package build

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/fantasim/canonlang/internal/project"
)

// WriteFS is a project's file system with the writes a build makes (API.md §2.2).
type WriteFS interface {
	project.FS
	WriteFile(name string, data []byte) error
	Rename(oldname, newname string) error
	Remove(name string) error
	MkdirAll(name string) error
}

// lockOut is a lock to write, with the content it replaces (nil: no file).
type lockOut struct {
	Lock
	old []byte
}

// change is one file a build replaces: its new and previous content, and the path a write error names (WIRE.md §2.3).
type change struct {
	abs, display string
	data, old    []byte
	existed      bool
	remove       bool   // delete the file: renamed aside, then removed once every other write is done
	respell      string // the spelling on disk of the file abs names, which takes abs's (DECISIONS 357)
}

// commit writes what changed, or, in Check mode, marks it stale and writes nothing (CLI.md §3.4).
func (r *run) commit(check bool, out *BuildResult, outputs []*output, locks []*lockOut) error {
	var changes, removals []change
	for _, o := range outputs {
		switch {
		case o.remove:
			removals = append(removals, change{abs: o.Abs, display: o.Path, remove: true})
		case o.Status == StatusWritten || o.Status == StatusAdopted:
			changes = append(changes, change{abs: o.Abs, display: o.Path, data: o.Content, old: o.old, existed: o.existed, respell: o.respell})
		}
		out.Outputs = append(out.Outputs, o.Output)
	}
	for _, l := range locks {
		if l.Status == StatusWritten {
			changes = append(changes, change{abs: l.Abs, display: l.Path, data: l.Content, old: l.old, existed: l.old != nil})
		}
		out.Locks = append(out.Locks, l.Lock)
	}
	changes = append(changes, removals...)
	sortOutputs(out.Outputs)
	if check {
		out.Stale = len(changes) > 0
		markStale(out)
		return nil
	}
	if len(changes) == 0 {
		return nil
	}
	fsys, ok := r.p.fs.(WriteFS)
	if !ok {
		return ErrReadOnly
	}
	w := r.writes()
	if w.err != nil {
		return w.err
	}
	return (&writer{fsys: fsys, token: w.token, may: r.bounds().allows}).writeAll(changes)
}

// markStale gives every output and lock a build would write or adopt the status stale, an
// --adopt output included: under --check nothing is written (DECISIONS 201).
func markStale(out *BuildResult) {
	for i := range out.Outputs {
		if out.Outputs[i].Status == StatusWritten || out.Outputs[i].Status == StatusAdopted {
			out.Outputs[i].Status = StatusStale
		}
	}
	for i := range out.Locks {
		if out.Locks[i].Status == StatusWritten {
			out.Locks[i].Status = StatusStale
		}
	}
}

// ModeFS is a file system that sets a file's permissions; a replaced file keeps its own.
type ModeFS interface {
	Chmod(name string, mode fs.FileMode) error
}

// displayErr prints a display path, op and cause in place of an OS error's absolute path, and
// unwraps to the whole original error, so errors.Is and errors.As see all it wrapped.
type displayErr struct {
	msg string
	err error
}

// Error is the display message.
func (e *displayErr) Error() string { return e.msg }

// Unwrap is the original error.
func (e *displayErr) Unwrap() error { return e.err }

// displayError names display in place of the absolute path (or two, a rename's) a *fs.PathError
// or *os.LinkError carries, its op and cause kept (DECISIONS 201).
func displayError(display string, err error) error {
	if pe, ok := asPathError(err); ok {
		return &displayErr{msg: fmt.Sprintf(fmtDisplayOpCause, display, pe.Op, pe.Err), err: err}
	}
	return fmt.Errorf(fmtDisplayCause, display, err)
}

// asPathError is err's *fs.PathError, or its rename's *os.LinkError as one naming the target;
// their paths are in the OS's form, usually absolute, and never printed as they are.
func asPathError(err error) (*fs.PathError, bool) {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe, true
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return &fs.PathError{Op: le.Op, Path: le.New, Err: le.Err}, true
	}
	return nil, false
}

// displayErrorIn names err's path by its display path in the project at dir (DECISIONS 201).
func displayErrorIn(dir string, err error) error {
	pe, ok := asPathError(err)
	if !ok {
		return err
	}
	return displayError(relativeTo(dir, pe.Path, filepath.Separator), err)
}

// relativeTo is the '/'-separated display path of name, an OS path separated by sep, in dir:
// "." for dir, dir's prefix cut at a separator, and only the last element of a path outside it.
func relativeTo(dir, name string, sep rune) string {
	abs := path.Clean(strings.ReplaceAll(name, string(sep), pathSep))
	root := path.Clean(dir)
	if abs == root {
		return listingMark
	}
	if rel, ok := strings.CutPrefix(abs, strings.TrimSuffix(root, pathSep)+pathSep); ok {
		return rel
	}
	if base := path.Base(abs); base != pathSep {
		return base
	}
	return listingMark
}

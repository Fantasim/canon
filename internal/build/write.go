package build

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"

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

// OS is the operating system's file system, writable.
func OS() WriteFS { return osFS{FS: project.OS()} }

type osFS struct {
	project.FS
}

func (osFS) WriteFile(name string, data []byte) error {
	return wrapIO(os.WriteFile(filepath.FromSlash(name), data, fileMode))
}

func (osFS) Rename(oldname, newname string) error {
	return wrapIO(os.Rename(filepath.FromSlash(oldname), filepath.FromSlash(newname)))
}

func (osFS) Remove(name string) error { return wrapIO(os.Remove(filepath.FromSlash(name))) }

func (osFS) Chmod(name string, mode fs.FileMode) error {
	return wrapIO(os.Chmod(filepath.FromSlash(name), mode))
}

func (osFS) MkdirAll(name string) error {
	return wrapIO(os.MkdirAll(filepath.FromSlash(name), dirMode))
}

func wrapIO(err error) error {
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return nil
}

// lockOut is a lock to write, with the content it replaces (nil: no file).
type lockOut struct {
	Lock
	old []byte
}

// change is one file a build replaces: its new and previous content.
type change struct {
	abs       string
	data, old []byte
	existed   bool
}

// commit writes what changed, or, in Check mode, marks it stale and writes nothing (CLI.md §3.4).
func (r *run) commit(check bool, out *BuildResult, outputs []*output, locks []*lockOut) error {
	var changes []change
	for _, o := range outputs {
		if o.Status == StatusWritten || o.Status == StatusAdopted {
			changes = append(changes, change{abs: o.Abs, data: o.Content, old: o.old, existed: o.existed})
		}
		out.Outputs = append(out.Outputs, o.Output)
	}
	for _, l := range locks {
		if l.Status == StatusWritten {
			changes = append(changes, change{abs: l.Abs, data: l.Content, old: l.old, existed: l.old != nil})
		}
		out.Locks = append(out.Locks, l.Lock)
	}
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
	return writeAll(fsys, changes)
}

// markStale gives every output and lock a build would write the status stale.
func markStale(out *BuildResult) {
	for i := range out.Outputs {
		if out.Outputs[i].Status == StatusWritten {
			out.Outputs[i].Status = StatusStale
		}
	}
	for i := range out.Locks {
		if out.Locks[i].Status == StatusWritten {
			out.Locks[i].Status = StatusStale
		}
	}
}

// modeFS is a file system that sets a file's permissions; a replaced file keeps its own.
type modeFS interface {
	Chmod(name string, mode fs.FileMode) error
}

// writer is one all-or-nothing write: the directories it created, removed again on a failure.
type writer struct {
	fsys    WriteFS
	created []string
}

// writeAll writes every file to a temporary file beside it, then renames each over its target;
// after a failure, the files already renamed get their previous content back and the
// directories it created are removed (API.md N11).
func writeAll(fsys WriteFS, changes []change) error {
	w := &writer{fsys: fsys}
	for i, c := range changes {
		if err := w.stage(c); err != nil {
			removeTemps(fsys, changes[:i+1])
			w.removeDirs()
			return err
		}
	}
	for i, c := range changes {
		if err := fsys.Rename(tempOf(c.abs), c.abs); err != nil {
			removeTemps(fsys, changes[i:])
			err = errors.Join(err, restore(fsys, changes[:i]))
			w.removeDirs()
			return err
		}
	}
	return nil
}

// stage writes a change's temporary file, its directory created, an existing file's mode kept.
func (w *writer) stage(c change) error {
	if err := w.mkdirs(path.Dir(c.abs)); err != nil {
		return err
	}
	tmp := tempOf(c.abs)
	if err := w.fsys.WriteFile(tmp, c.data); err != nil {
		return err
	}
	chmod, ok := w.fsys.(modeFS)
	if !c.existed || !ok {
		return nil
	}
	info, err := w.fsys.Stat(c.abs)
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return chmod.Chmod(tmp, info.Mode().Perm())
}

// mkdirs creates dir and records the directories that did not exist.
func (w *writer) mkdirs(dir string) error {
	var missing []string
	for d := dir; d != path.Dir(d); d = path.Dir(d) {
		if _, err := w.fsys.Stat(d); !errors.Is(err, fs.ErrNotExist) {
			break
		}
		missing = append(missing, d)
	}
	w.created = append(w.created, missing...)
	return w.fsys.MkdirAll(dir)
}

// removeDirs removes the directories the write created, deepest first.
func (w *writer) removeDirs() {
	slices.SortFunc(w.created, func(a, b string) int { return cmp.Or(cmp.Compare(len(b), len(a)), cmp.Compare(a, b)) })
	for _, d := range slices.Compact(w.created) {
		_ = w.fsys.Remove(d) // one another writer filled meanwhile stays
	}
}

// tempOf is the temporary file a change is written to first: hidden, in the target's directory.
func tempOf(abs string) string {
	return path.Join(path.Dir(abs), tempPrefix+path.Base(abs)+tempSuffix)
}

func removeTemps(fsys WriteFS, changes []change) {
	for _, c := range changes {
		_ = fsys.Remove(tempOf(c.abs)) // a temporary file that was never written is no failure
	}
}

// restore puts back the previous content of files already replaced, or removes a new one.
func restore(fsys WriteFS, done []change) error {
	var errs []error
	for _, c := range done {
		if !c.existed {
			errs = append(errs, fsys.Remove(c.abs))
			continue
		}
		err := fsys.WriteFile(tempOf(c.abs), c.old)
		if err == nil {
			err = fsys.Rename(tempOf(c.abs), c.abs)
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

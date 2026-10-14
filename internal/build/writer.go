package build

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/fantasim/canonlang/internal/project"
)

// writer is one all-or-nothing write: the run's staging token (DECISIONS 351), the directories
// it created, removed again on a failure, and the directories it may create (nil: any).
type writer struct {
	fsys    WriteFS
	token   string
	created []string
	may     func(dir string) bool
}

// stageOf is the name the writer stages abs under, or sets it aside under.
func (w *writer) stageOf(abs string) string { return stagedAs(abs, w.token) }

// writeAll writes every file to a staged file beside it, then renames each over its target,
// creating only the directories may allows; after a failure, the files already renamed get
// their previous content back and the directories it created are removed (API.md N11).
func (w *writer) writeAll(changes []change) error {
	for i, c := range changes {
		if err := w.stage(c); err != nil {
			w.removeStages(changes[:i+1])
			w.removeDirs()
			return displayError(c.display, err)
		}
	}
	for i, c := range changes {
		if err := w.replace(c); err != nil {
			w.removeStages(changes[i:])
			err = errors.Join(displayError(c.display, err), w.restore(changes[:i]))
			w.removeDirs()
			return err
		}
	}
	return w.deleteAside(changes)
}

// deleteAside deletes the files set aside by a write whose renames all succeeded; a failure says so, since the outputs are written, and the next build sweeps what is left (CODEGEN.md §2.9).
func (w *writer) deleteAside(changes []change) error {
	var errs []error
	for _, c := range changes {
		if !c.remove {
			continue
		}
		if err := w.fsys.Remove(w.stageOf(c.abs)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf(fmtAsideLeft, c.display, err))
		}
	}
	return errors.Join(errs...)
}

// replace puts a change's staged file in its place, sets a file to delete aside (CODEGEN.md §2.9), or gives a file or a directory its declared spelling.
func (w *writer) replace(c change) error {
	switch {
	case c.dir: // one rename, nothing set aside: a killed build never hides the directory (DECISIONS 377)
		return w.fsys.Rename(c.respell, c.abs)
	case c.remove:
		return w.fsys.Rename(c.abs, w.stageOf(c.abs))
	case c.respell != "":
		return w.respell(c)
	case c.late:
		if err := w.stageFile(c.abs, c.data, true); err != nil {
			return err
		}
	}
	return w.fsys.Rename(w.stageOf(c.abs), c.abs)
}

// respell sets the file aside from its spelling on disk under the run's staging name and renames
// it in under abs, then stages new content and renames it over: the two spellings' staging names
// are one name on such a file system, so nothing was staged before (DECISIONS 357).
func (w *writer) respell(c change) error {
	aside := w.stageOf(c.respell)
	if err := w.fsys.Rename(c.respell, aside); err != nil {
		return err
	}
	if err := w.fsys.Rename(aside, c.abs); err != nil {
		return errors.Join(err, w.fsys.Rename(aside, c.respell))
	}
	if bytes.Equal(c.data, c.old) {
		return nil
	}
	err := w.stageFile(c.abs, c.data, true)
	if err == nil {
		err = w.fsys.Rename(w.stageOf(c.abs), c.abs)
	}
	if err != nil {
		return errors.Join(err, w.unspell(c))
	}
	return nil
}

// stage writes a change's staged file, its directory created, an existing file's mode kept; a
// respelled file is staged when it is replaced, a renamed directory never.
func (w *writer) stage(c change) error {
	if c.remove || c.respell != "" || c.late {
		return nil
	}
	if err := w.mkdirs(project.DirOf(c.abs)); err != nil {
		return err
	}
	return w.stageFile(c.abs, c.data, c.existed)
}

// stageFile writes data under abs's staging name, with the mode of the file abs when it exists.
func (w *writer) stageFile(abs string, data []byte, existed bool) error {
	tmp := w.stageOf(abs)
	if err := WriteStaged(w.fsys, tmp, data, nil); err != nil {
		return err
	}
	if !existed {
		return nil
	}
	return keepMode(w.fsys, tmp, abs)
}

// keepMode gives tmp the permissions of the file abs, where the file system sets them (handoff 2026-10-07 A2).
func keepMode(fsys WriteFS, tmp, abs string) error {
	perm, err := permOf(fsys, abs)
	if err != nil || perm == nil {
		return err
	}
	return SetPerm(fsys, tmp, *perm)
}

// permOf is the permissions of the file abs where fsys sets them, nil elsewhere.
func permOf(fsys WriteFS, abs string) (*fs.FileMode, error) {
	if _, ok := fsys.(ModeFS); !ok {
		return nil, nil
	}
	info, err := fsys.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	perm := info.Mode().Perm()
	return &perm, nil
}

// SetPerm gives name the permissions perm where fsys sets them; elsewhere a file keeps the mode
// its write gave it.
func SetPerm(fsys project.FS, name string, perm fs.FileMode) error {
	if m, ok := fsys.(ModeFS); ok {
		return m.Chmod(name, perm)
	}
	return nil
}

// mkdirs makes dir's missing directories top-down, recording each; a forbidden one fails first (CODEGEN.md §2.4).
func (w *writer) mkdirs(dir string) error {
	var missing []string
	for d := dir; d != project.DirOf(d); d = project.DirOf(d) {
		if _, err := w.fsys.Stat(d); !errors.Is(err, fs.ErrNotExist) {
			break
		}
		if w.may != nil && !w.may(d) {
			return &fs.PathError{Op: opMkdir, Path: d, Err: errRootMissing}
		}
		missing = append(missing, d)
	}
	for _, d := range slices.Backward(missing) {
		err := w.mkdir(d)
		if errors.Is(err, fs.ErrExist) {
			if info, serr := w.fsys.Stat(d); serr == nil && info.IsDir() {
				continue // made meanwhile by another process: not ours to remove
			}
		}
		if err != nil {
			return err
		}
		w.created = append(w.created, d)
	}
	return nil
}

// dirMaker is a file system that creates one directory whose parent exists.
type dirMaker interface {
	Mkdir(name string) error
}

// mkdir creates the directory d, its parent existing: Mkdir where the file system has it, so
// that a parent gone meanwhile is never made again; else MkdirAll.
func (w *writer) mkdir(d string) error {
	if m, ok := w.fsys.(dirMaker); ok {
		return m.Mkdir(d)
	}
	return w.fsys.MkdirAll(d)
}

// removeDirs removes the directories the write created, deepest first.
func (w *writer) removeDirs() {
	slices.SortFunc(w.created, func(a, b string) int { return cmp.Or(cmp.Compare(len(b), len(a)), cmp.Compare(a, b)) })
	for _, d := range slices.Compact(w.created) {
		_ = w.fsys.Remove(d) // one another writer filled meanwhile stays
	}
}

// removeStages removes the files staged for changes not renamed in: this run's names only, and
// never a respelled file's, whose staging name may hold the file itself set aside, nor a
// renamed directory's, which has none.
func (w *writer) removeStages(changes []change) {
	for _, c := range changes {
		if !c.remove && c.respell == "" {
			_ = w.fsys.Remove(w.stageOf(c.abs)) // a staged file that was never written is no failure
		}
	}
}

// restore puts back the previous content of files already replaced, or removes a new one; each
// failure names its display path, not the staged one (DECISIONS 201).
func (w *writer) restore(done []change) error {
	var errs []error
	for _, c := range slices.Backward(done) { // the last first: a file written twice gets its first content back last
		if err := w.restoreOne(c); err != nil {
			errs = append(errs, displayError(c.display, err))
		}
	}
	return errors.Join(errs...)
}

// restoreOne restores one file already replaced, or removes one that did not exist before; a
// respelled file or directory gets its old spelling back too.
func (w *writer) restoreOne(c change) error {
	switch {
	case c.dir:
		return w.fsys.Rename(c.abs, c.respell)
	case c.remove:
		return w.fsys.Rename(w.stageOf(c.abs), c.abs)
	case !c.existed:
		return w.fsys.Remove(c.abs)
	case c.respell != "":
		return w.unspell(c)
	}
	if err := w.stageFile(c.abs, c.old, true); err != nil {
		return err
	}
	return w.fsys.Rename(w.stageOf(c.abs), c.abs)
}

// unspell puts a respelled file back under its spelling on disk, with its previous content.
func (w *writer) unspell(c change) error {
	tmp := w.stageOf(c.respell)
	if err := WriteStaged(w.fsys, tmp, c.old, nil); err != nil {
		return err
	}
	if err := keepMode(w.fsys, tmp, c.abs); err != nil {
		return err
	}
	if err := w.fsys.Remove(c.abs); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return w.fsys.Rename(tmp, c.respell)
}

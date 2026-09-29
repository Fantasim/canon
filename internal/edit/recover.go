package edit

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/build"
)

// Process is who commits or recovers: the host and pid a journal records, and whether a pid of
// this host still runs. Recover leaves a live writer's journal alone; Alive nil counts every
// writer dead (log-2026-09-29 M4 U4c-r).
type Process struct {
	Host  string
	PID   int
	Alive func(pid int) bool
}

// running says whether j's writer runs on this host now.
func (p Process) running(j *journal) bool {
	return j.Host == p.Host && j.PID > 0 && p.Alive != nil && p.Alive(j.PID)
}

// Recover rolls back this host's unfinished edits (API.md O5); journals are untrusted input, see ADR-0010.
func Recover(s Site, logger *slog.Logger) error {
	names, err := journals(s.FS, s.Layout.Dir())
	if err != nil || len(names) == 0 {
		return err
	}
	var errs []error
	rolled, changes := 0, 0
	for _, name := range names {
		n, err := recoverOne(s, name)
		if n > 0 {
			rolled++
			changes += n
		}
		if err != nil {
			errs = append(errs, fmt.Errorf(fmtFileErr, path.Base(name), journalRefusal(ErrJournal, err)))
		}
	}
	if rolled > 0 && logger != nil {
		logger.Warn(msgRecovered, slog.Int(attrJournals, rolled), slog.Int(attrChanges, changes))
	}
	return errors.Join(errs...)
}

// journals are the journal files under dir, in byte order; none when there is no journal directory.
func journals(fsys build.WriteFS, dir string) ([]string, error) {
	jdir := path.Join(dir, journalDir)
	entries, err := fsys.ReadDir(jdir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf(fmtFileErr, journalDir, err)
	}
	var out []string
	for _, e := range entries {
		if e.Type().IsRegular() && isJournal(e.Name()) {
			out = append(out, path.Join(jdir, e.Name()))
		}
	}
	slices.Sort(out)
	return out, nil
}

// recoverOne rolls back the edit of the journal file name and removes it, unless its writer
// still runs or it was written on another machine; it returns how many entries it changed.
func recoverOne(s Site, name string) (int, error) {
	data, err := s.FS.ReadFile(name)
	if err != nil {
		return 0, err
	}
	var j journal
	if json.Unmarshal(data, &j) != nil {
		return 0, removeJournal(s.FS, name) // cut short before any file changed: WriteFile is atomic
	}
	if s.Self.running(&j) {
		return 0, nil
	}
	if j.Host != s.Self.Host {
		return 0, &fault{reasonForeign, j.Host}
	}
	u, err := j.resolve(s.Layout)
	if err != nil {
		return 0, err
	}
	n, err := u.run(s.FS)
	if err != nil {
		return n, err
	}
	return n, removeJournal(s.FS, name)
}

func removeJournal(fsys build.WriteFS, name string) error {
	if err := fsys.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// undo is what a rollback puts back, placed: files to return to their state before the commit,
// the stages to clear, the directories the commit created and those it removed.
type undo struct {
	files   []undoFile
	stages  []string
	created []spot
	removed []undoDir
}

type undoFile struct {
	spot
	journalFile
}

type undoDir struct {
	spot
	mode *uint32
}

// spots are every place the undo touches.
func (u *undo) spots() []spot {
	out := slices.Clone(u.created)
	for _, f := range u.files {
		out = append(out, f.spot)
	}
	for _, d := range u.removed {
		out = append(out, d.spot)
	}
	return out
}

// undoer runs an undo, counting what it changes and the directories whose entries changed.
type undoer struct {
	fsys    build.WriteFS
	touched []string
	changed int
}

// run rolls back, all or none, decided before any write: no place through a symbolic link, no
// file in neither its old nor its new state, no created directory holding what is not the
// edit's; the touched directories are synced last (API.md N11; log-2026-09-29 M4 U4c-r3).
func (u *undo) run(fsys build.WriteFS) (int, error) {
	if err := straight(fsys, u.spots()); err != nil {
		return 0, err
	}
	back, err := u.pending(fsys)
	if err != nil {
		return 0, err
	}
	if err := u.clearable(fsys); err != nil {
		return 0, err
	}
	for _, s := range u.stages {
		_ = fsys.Remove(stageName(s)) // a stage never written is no failure
	}
	r := &undoer{fsys: fsys}
	if err := r.recreate(u.removed); err != nil {
		return r.changed, err
	}
	if err := r.restore(back); err != nil {
		return r.changed, err
	}
	if err := r.clear(u.created); err != nil {
		return r.changed, err
	}
	return r.changed, syncDirs(fsys, r.touched)
}

// pending are the files still in their new state; one in neither state refuses the rollback.
func (u *undo) pending(fsys build.WriteFS) ([]undoFile, error) {
	var back []undoFile
	var changed []string
	for _, f := range u.files {
		now, err := fsys.ReadFile(f.abs)
		present := err == nil
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf(fmtFileErr, f.Path, err)
		}
		switch {
		case present == f.Existed && bytes.Equal(now, f.Old):
		case f.New == newAbsent && !present, present && f.New == digest(now):
			back = append(back, f)
		default:
			changed = append(changed, f.Path)
		}
	}
	if len(changed) > 0 {
		return nil, &fault{reasonChanged, strings.Join(changed, listSep)}
	}
	return back, nil
}

// clearable refuses the rollback if a created directory holds anything but other created
// directories, the edit's new files, their stages and what writing a stage left, named after it.
func (u *undo) clearable(fsys build.WriteFS) error {
	own := u.own()
	var strays []string
	for _, d := range u.created {
		entries, err := fsys.ReadDir(d.abs)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf(fmtFileErr, d.abs, err)
		}
		for _, e := range entries {
			if name := path.Join(d.abs, e.Name()); !ownEntry(own, name, e) {
				strays = append(strays, name)
			}
		}
	}
	if len(strays) > 0 {
		slices.Sort(strays)
		return &fault{reasonStray, strings.Join(strays, listSep)}
	}
	return nil
}

// own are the names the edit may have left in a directory it created: those directories, its
// new files and its stages.
func (u *undo) own() map[string]bool {
	own := map[string]bool{}
	for _, d := range u.created {
		own[d.abs] = true
	}
	for _, f := range u.files {
		if !f.Existed {
			own[f.abs] = true
		}
	}
	for _, s := range u.stages {
		own[stageName(s)] = true
	}
	return own
}

// ownEntry says whether an entry of a created directory is the edit's: one of own, a directory
// only if own is one, or a hidden leftover whose name holds an own stage's.
func ownEntry(own map[string]bool, name string, e fs.DirEntry) bool {
	switch {
	case e.Type()&fs.ModeSymlink != 0:
		return false
	case own[name]:
		return true
	case e.IsDir() || e.Name()[0] != hiddenMark:
		return false
	}
	for _, stem := range stagesIn(own, path.Dir(name)) {
		if strings.Contains(e.Name(), stem) {
			return true
		}
	}
	return false
}

// stagesIn are the names of the stages among own that lie right in dir.
func stagesIn(own map[string]bool, dir string) []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(own)) {
		if path.Dir(name) == dir && strings.HasSuffix(name, stageSuffix) {
			out = append(out, path.Base(name))
		}
	}
	return out
}

// recreate makes again, parents first, each removed directory not there, with its permissions.
func (r *undoer) recreate(dirs []undoDir) error {
	ordered := slices.Clone(dirs)
	slices.SortFunc(ordered, func(a, b undoDir) int { return deepestFirst(b.abs, a.abs) })
	for _, d := range ordered {
		there, err := r.exists(d.abs)
		if err != nil {
			return err
		}
		if there {
			continue
		}
		if err := r.fsys.MkdirAll(d.abs); err != nil {
			return err
		}
		if err := chmod(r.fsys, d.abs, d.mode); err != nil {
			return err
		}
		r.touch(d.abs)
	}
	return nil
}

// restore gives each file its old content and permissions, or removes it if it did not exist.
func (r *undoer) restore(files []undoFile) error {
	for _, f := range files {
		if err := r.restoreOne(f); err != nil {
			return fmt.Errorf(fmtFileErr, f.Path, err)
		}
		r.touch(f.abs)
	}
	return nil
}

func (r *undoer) restoreOne(f undoFile) error {
	if !f.Existed {
		if err := r.fsys.Remove(f.abs); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := r.fsys.MkdirAll(path.Dir(f.abs)); err != nil {
		return err
	}
	return replace(r.fsys, f.abs, f.Old, f.Mode)
}

// clear removes, deepest first, each directory the commit created with all it holds, which
// clearable found to be the edit's (log-2026-09-29 M4 U4c-r, U4c-r3).
func (r *undoer) clear(dirs []spot) error {
	ordered := slices.Clone(dirs)
	slices.SortFunc(ordered, func(a, b spot) int { return deepestFirst(a.abs, b.abs) })
	for _, d := range ordered {
		there, err := r.exists(d.abs)
		if err != nil {
			return err
		}
		if !there {
			continue
		}
		if err := removeAll(r.fsys, d.abs); err != nil {
			return err
		}
		r.touch(d.abs)
	}
	return nil
}

func (r *undoer) exists(name string) (bool, error) {
	_, err := r.fsys.Stat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// touch counts a change to name and keeps its directory to sync.
func (r *undoer) touch(name string) {
	r.changed++
	r.touched = append(r.touched, path.Dir(name))
}

// removeAll removes dir and everything in it, never descending a symbolic link: a link is
// removed itself (log-2026-09-29 M4 U4c-r3).
func removeAll(fsys build.WriteFS, dir string) error {
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return err
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return cmp.Compare(a.Name(), b.Name()) })
	for _, e := range entries {
		child := path.Join(dir, e.Name())
		if e.IsDir() && e.Type()&fs.ModeSymlink == 0 {
			err = removeAll(fsys, child)
		} else {
			err = fsys.Remove(child)
		}
		if err != nil {
			return err
		}
	}
	return fsys.Remove(dir)
}

// dirSyncer is a file system that makes a directory's entries durable; U8's OS FS has it.
type dirSyncer interface {
	SyncDir(dir string) error
}

// syncDirs makes each directory's entries durable where the FS can; one gone since needs none.
func syncDirs(fsys build.WriteFS, dirs []string) error {
	s, ok := fsys.(dirSyncer)
	if !ok {
		return nil
	}
	ordered := slices.Clone(dirs)
	slices.Sort(ordered)
	for _, d := range slices.Compact(ordered) {
		if err := s.SyncDir(d); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf(fmtFileErr, d, err)
		}
	}
	return nil
}

package edit

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
)

// Process is who commits or recovers: the host and pid a journal records, and whether a pid of
// this host still runs. Recover leaves a live writer's journal alone; Alive nil counts every
// writer dead (log-2026-09-29 M4 U4c-r).
type Process struct {
	Host  string
	PID   int
	Alive func(pid int) bool
}

// running says whether j's writer runs on this host now: the test of a journal recording no lock.
func (p Process) running(j *journal) bool {
	return j.Host == p.Host && j.PID > 0 && p.Alive != nil && p.Alive(j.PID)
}

// Recover rolls back this host's unfinished edits (API.md O5, N11); journals are untrusted input, see ADR-0010.
func Recover(s Site, logger *slog.Logger) error {
	names, err := journals(s.FS, s.Layout.Dir())
	if err != nil || len(names) == 0 {
		return err
	}
	var t tally
	for _, name := range names {
		r, err := recoverOne(s, name)
		t.add(name, r, err)
	}
	t.log(logger)
	return errors.Join(t.errs...)
}

// tally counts the journals a recovery rolled back, the entries it changed, and the journals it
// refused, each named (API.md O5).
type tally struct {
	rolled, changes int
	errs            []error
	removedOnly     bool // Settle: only a journal removed counts
}

// add counts one journal: r nil when it was left or skipped; a decodable journal removed is an
// edit rolled back, whatever it changed, and so is, for Recover, one it changed files of.
func (t *tally) add(name string, r *undoer, err error) {
	if r != nil && (err == nil || !t.removedOnly && r.changed > 0) {
		t.rolled++
		t.changes += r.changed
	}
	if err != nil {
		t.errs = append(t.errs, fmt.Errorf(fmtFileErr, path.Base(name), journalRefusal(ErrJournal, err)))
	}
}

// log is O5's one Warn line, when a journal was rolled back.
func (t *tally) log(logger *slog.Logger) {
	if t.rolled > 0 && logger != nil {
		logger.Warn(msgRecovered, slog.Int(attrJournals, t.rolled), slog.Int(attrChanges, t.changes))
	}
}

// journals are the journal files under dir, in byte order; none when there is no journal directory.
func journals(fsys build.WriteFS, dir string) ([]string, error) {
	jdir := project.Join(dir, JournalDir)
	entries, err := fsys.ReadDir(jdir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf(fmtFileErr, JournalDir, err)
	}
	var out []string
	for _, e := range entries {
		if e.Type().IsRegular() && isJournal(e.Name()) {
			out = append(out, project.Join(jdir, e.Name()))
		}
	}
	slices.Sort(out)
	return out, nil
}

// recoverOne rolls back the edit of the journal file name and removes it, unless its writer
// still runs, it was written on another machine or this site cannot take its locks; nil when it
// rolled nothing back: gone, torn, left (API.md N11; DECISIONS 375).
func recoverOne(s Site, name string) (*undoer, error) {
	l, err := loadJournal(s.FS, name)
	switch {
	case err != nil:
		return nil, err
	case l.gone:
		return nil, nil
	case l.torn:
		return nil, removeJournal(s.FS, name) // cut short before any file changed: WriteFile is atomic
	case l.bad:
		return nil, &fault{reasonLocks, strings.Join(l.j.Locks, listSep)}
	case l.j.Host != s.Self.Host:
		return nil, &fault{reasonForeign, l.j.Host}
	case l.locked:
		return recoverLocked(s, name, l.j)
	case s.Self.running(l.j):
		return nil, nil
	}
	return rollBack(s, name, l.j)
}

// recoverLocked rolls back a journal recording locks only holding them all, taken without waiting,
// all or none; one held, or no Locker, leaves it (DECISIONS 375: J2a, J3).
func recoverLocked(s Site, name string, j *journal) (r *undoer, err error) {
	if s.Locks == nil {
		return nil, nil
	}
	release, ok, err := s.Locks.TryLock(j.Locks)
	if err != nil || !ok {
		return nil, err
	}
	defer func() { err = errors.Join(err, release()) }()
	return underLocks(s, name, j)
}

// underLocks reads the journal again under its locks: gone, or its locks or host changed, it is
// left; otherwise it is rolled back (DECISIONS 375: J3).
func underLocks(s Site, name string, j *journal) (*undoer, error) {
	l, err := loadJournal(s.FS, name)
	if err != nil || l.gone || !l.locked || l.j.Host != j.Host || !slices.Equal(l.j.Locks, j.Locks) {
		return nil, err
	}
	return rollBack(s, name, l.j)
}

// rollBack checks the journal j of the file name, undoes its edit and removes it; the undoer is
// what it changed, nil when the journal was refused before any write.
func rollBack(s Site, name string, j *journal) (*undoer, error) {
	u, err := j.resolve(s.Layout, stageToken(name))
	if err != nil {
		return nil, err
	}
	r, err := u.run(s.FS)
	if err != nil {
		return r, err
	}
	return r, removeJournal(s.FS, name)
}

// clearStages removes the edit's stages, 0.1.5's fixed ones, and what the OS WriteFile left
// writing one, exactly those forms (DECISIONS 351); one never written is no failure.
func (u *undo) clearStages(fsys build.WriteFS) {
	bases := map[string][]string{}
	for _, s := range u.stages {
		for _, stage := range []string{stageName(s, u.token), fixedStageName(s)} {
			_ = fsys.Remove(stage)
			bases[project.DirOf(stage)] = append(bases[project.DirOf(stage)], path.Base(stage))
		}
	}
	for _, dir := range slices.Sorted(maps.Keys(bases)) {
		clearLeftovers(fsys, dir, bases[dir])
	}
}

// clearLeftovers removes each file of dir that the OS WriteFile left writing one of bases, as
// build names such a leftover (build.StagedOf), nothing else; a leftover kept is no failure.
func clearLeftovers(fsys build.WriteFS, dir string, bases []string) {
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		left := func(base string) bool { return build.StagedOf(e.Name(), base) }
		if !e.IsDir() && slices.ContainsFunc(bases, left) {
			_ = fsys.Remove(project.Join(dir, e.Name()))
		}
	}
}

// removeJournal removes the journal name and what writing it left beside it (API.md §2.2).
func removeJournal(fsys build.WriteFS, name string) error {
	clearLeftovers(fsys, project.DirOf(name), []string{path.Base(name)})
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
	token   string // the stages' <j>, from the journal's name (DECISIONS 351)
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

// undoer runs an undo, counting what it changes, the files it restores or removes, and the
// directories whose entries changed.
type undoer struct {
	fsys    build.WriteFS
	token   string
	touched []string
	files   []string
	changed int
}

// run rolls back, all or none, decided before any write: no place through a symbolic link, no
// file in neither its old nor its new state, no created directory holding what is not the
// edit's; the touched directories are synced last (API.md N11; log-2026-09-29 M4 U4c-r3).
func (u *undo) run(fsys build.WriteFS) (*undoer, error) {
	r := &undoer{fsys: fsys, token: u.token}
	if err := straight(fsys, u.spots()); err != nil {
		return r, err
	}
	back, err := u.pending(fsys)
	if err != nil {
		return r, err
	}
	if err := u.clearable(fsys); err != nil {
		return r, err
	}
	u.clearStages(fsys)
	if err := r.recreate(u.removed); err != nil {
		return r, err
	}
	if err := r.restore(back); err != nil {
		return r, err
	}
	if err := r.clear(u.created); err != nil {
		return r, err
	}
	return r, syncDirs(fsys, r.touched)
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
			if name := project.Join(d.abs, e.Name()); !ownEntry(own, name, e) {
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
		own[stageName(s, u.token)], own[fixedStageName(s)] = true, true
	}
	return own
}

// ownEntry says whether an entry of a created directory is the edit's: one of own, a directory
// only if own is one, or what the OS WriteFile left writing one of own's stages (DECISIONS 351).
func ownEntry(own map[string]bool, name string, e fs.DirEntry) bool {
	switch {
	case e.Type()&fs.ModeSymlink != 0:
		return false
	case own[name]:
		return true
	case e.IsDir():
		return false
	}
	return writeLeftover(own, name)
}

// writeLeftover reports name a staging name the OS WriteFile gives one of own's stages in name's
// directory, exactly as build names it (DECISIONS 351): what writing that stage left, nothing else.
func writeLeftover(own map[string]bool, name string) bool {
	dir, base := project.DirOf(name), path.Base(name)
	for stage := range own { //canon:unordered any stage that matches decides
		if project.DirOf(stage) == dir && strings.HasSuffix(stage, stageSuffix) && build.StagedOf(base, path.Base(stage)) {
			return true
		}
	}
	return false
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
		r.files = append(r.files, f.abs)
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
	if err := r.fsys.MkdirAll(project.DirOf(f.abs)); err != nil {
		return err
	}
	return build.ReplaceStaged(r.fsys, f.abs, stageName(f.abs, r.token), f.Old, modeOf(f.Mode))
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
	r.touched = append(r.touched, project.DirOf(name))
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
		child := project.Join(dir, e.Name())
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

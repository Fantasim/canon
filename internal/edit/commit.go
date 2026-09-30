package edit

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
)

// Layout places an edit's files: Dir is the project directory, which holds .canon/journal, Abs
// a display path's file on disk (false for none), Display an absolute name's display path.
type Layout interface {
	Dir() string
	Abs(file string) (string, bool)
	Display(abs string) string
}

var _ Layout = (*build.Project)(nil)

// Site is where a commit writes or a recovery rolls back, and the process doing it.
type Site struct {
	FS     build.WriteFS
	Layout Layout
	Self   Process
}

// commitFile is one file a commit writes (data) or removes, or a directory it removes once empty.
type commitFile struct {
	abs, display string
	from         string // a renamed file's old place, whose permissions it keeps (API.md N8)
	data         []byte
	remove, dir  bool
	old          []byte
	existed      bool
	mode         *uint32 // the permissions its stage gets, or a removed directory's
	jf           journalFile
}

// commit is one Commit under way: its files in byte order of abs, the directories it creates
// (journaled) and made (staging did), the targets it has renamed or removed so far, and the
// journal that undoes it.
type commit struct {
	fsys    build.WriteFS
	at      Layout
	dir     string
	name    string // the journal file
	modes   bool   // the FS sets permissions
	targets []commitFile
	created []string
	made    []string
	done    []int
	j       journal
}

// Commit writes changes, planned on the snapshot whose new revision is rev, all or none (API.md
// N6, N8-N12, S11); the journal is only as safe as WriteFile is atomic and durable and SyncDir,
// where the FS has it, syncs. A cancellation after the first rename is ignored.
func Commit(ctx context.Context, s Site, rev string, changes []Change) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(changes) == 0 {
		return nil
	}
	c, err := plan(s, rev, changes)
	if err != nil {
		return err
	}
	if err := c.idle(); err != nil {
		return err
	}
	if err := c.check(); err != nil {
		return err
	}
	if err := c.vet(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := c.writeJournal(); err != nil {
		return err
	}
	if err := c.stageAll(ctx); err != nil {
		return c.rollback(err)
	}
	if err := c.apply(); err != nil {
		return c.rollback(err)
	}
	return nil
}

// plan places every file of changes, their new contents with N12's line ends, in byte order.
// Two changes of one file are refused; so is a case-only rename where names fold case, by N9.
func plan(s Site, rev string, changes []Change) (*commit, error) {
	name, err := journalName(s.Layout.Dir(), rev)
	if err != nil {
		return nil, err
	}
	_, modes := s.FS.(build.ModeFS)
	j := journal{Version: journalVersion, Revision: rev, Host: s.Self.Host, PID: s.Self.PID}
	c := &commit{fsys: s.FS, at: s.Layout, dir: s.Layout.Dir(), name: name, modes: modes, j: j}
	for _, ch := range changes {
		if int(ch.Kind) >= len(expanders) {
			return nil, fmt.Errorf(fmtFileErr, ch.Path, ErrChanges)
		}
		for _, t := range expanders[ch.Kind](ch) {
			if err := place(s.Layout, &t); err != nil {
				return nil, err
			}
			c.targets = append(c.targets, t)
		}
	}
	slices.SortFunc(c.targets, func(a, b commitFile) int { return cmp.Compare(a.abs, b.abs) })
	for i := 1; i < len(c.targets); i++ {
		if c.targets[i].abs == c.targets[i-1].abs {
			return nil, fmt.Errorf(fmtFileErr, c.targets[i].display, ErrChanges)
		}
	}
	return c, nil
}

// place resolves a target's display paths and gives its new content N12's line ends.
func place(at Layout, t *commitFile) error {
	abs, ok := at.Abs(t.display)
	if !ok {
		return fmt.Errorf(fmtFileErr, t.display, ErrChanges)
	}
	t.abs = abs
	if t.from != "" {
		if t.from, ok = at.Abs(t.from); !ok {
			return fmt.Errorf(fmtFileErr, t.display, ErrChanges)
		}
	}
	if !t.remove {
		t.data = textEnds(t.data)
	}
	return nil
}

func modifiedFiles(c Change) []commitFile {
	return []commitFile{{display: c.Path, data: c.After, old: c.Before, existed: true}}
}

func createdFiles(c Change) []commitFile {
	return []commitFile{{display: c.Path, data: c.After}}
}

func deletedFiles(c Change) []commitFile {
	return []commitFile{{display: c.Path, remove: true, old: c.Before, existed: true}}
}

// renamedFiles are a renamed file's old path, removed, and its new one, created (API.md N8).
func renamedFiles(c Change) []commitFile {
	return []commitFile{
		{display: c.OldPath, remove: true, old: c.Before, existed: true},
		{display: c.Path, from: c.OldPath, data: c.After},
	}
}

func removedDirFiles(c Change) []commitFile {
	return []commitFile{{display: c.Path, remove: true, dir: true}}
}

// textEnds is data with '\n' line ends and exactly one final '\n' (API.md N12).
func textEnds(data []byte) []byte {
	text := bytes.TrimRight(bytes.ReplaceAll(data, []byte(crlfEnd), []byte(lineEnd)), lineEnd)
	return append(text, lineEnd...)
}

// vet refuses a commit whose journal Recover would refuse: a place through a symbolic link or
// hidden is ErrUnwritable, the project's state (log-2026-09-29 M4 U5b-r); any other is ErrChanges.
func (c *commit) vet() error {
	u, err := c.j.resolve(c.at)
	if err == nil {
		err = straight(c.fsys, u.spots())
	}
	var f *fault
	if errors.As(err, &f) && (f.reason == reasonLink || f.reason == reasonHidden) {
		return journalRefusal(ErrUnwritable, err)
	}
	return journalRefusal(ErrChanges, err)
}

// idle refuses a commit while any journal is there: an edit unfinished, or still running, whose
// rollback would undo this one (log-2026-09-29 M4 U4c-r).
func (c *commit) idle() error {
	names, err := journals(c.fsys, c.dir)
	if err != nil {
		return err
	}
	if len(names) > 0 {
		return fmt.Errorf(fmtFileErr, path.Base(names[0]), ErrJournal)
	}
	return nil
}

// check re-reads every file and compares it with the snapshot the edit was planned on
// (API.md N9), then records the journal.
func (c *commit) check() error {
	var stale []string
	for i := range c.targets {
		t := &c.targets[i]
		if t.dir {
			continue
		}
		now, err := c.fsys.ReadFile(t.abs)
		present := err == nil
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf(fmtFileErr, t.display, err)
		}
		if present != t.existed || !bytes.Equal(now, t.old) {
			stale = append(stale, t.display)
		}
	}
	if len(stale) > 0 {
		slices.Sort(stale)
		return &StaleError{Files: slices.Compact(stale)}
	}
	return c.record()
}

// record fills the journal: each file as it is and the digest of what it becomes, the
// directories new files need that do not exist yet, and the directories to remove.
func (c *commit) record() error {
	for i := range c.targets {
		t := &c.targets[i]
		record := c.recordFile
		if t.dir {
			record = c.recordDir
		}
		if err := record(t); err != nil {
			return err
		}
	}
	slices.SortFunc(c.j.Files, func(a, b journalFile) int { return cmp.Compare(a.Path, b.Path) })
	slices.Sort(c.created)
	c.created = slices.Compact(c.created)
	for _, d := range c.created {
		c.j.Created = append(c.j.Created, relIn(c.dir, d))
	}
	slices.SortFunc(c.j.Removed, func(a, b removedDir) int { return cmp.Compare(a.Path, b.Path) })
	return nil
}

func (c *commit) recordFile(t *commitFile) error {
	t.jf = journalFile{Path: relIn(c.dir, t.abs), Existed: t.existed, Old: t.old, New: newAbsent}
	if !t.remove {
		t.jf.New = digest(t.data)
	}
	var err error
	switch {
	case t.existed:
		t.mode, err = c.perm(t.abs, t.display)
		t.jf.Mode = rwOnly(t.mode)
	case t.from != "":
		t.mode, err = c.perm(t.from, t.display)
	}
	if err != nil {
		return err
	}
	if !t.existed {
		missing, err := c.missing(project.DirOf(t.abs))
		if err != nil {
			return fmt.Errorf(fmtFileErr, t.display, err)
		}
		c.created = append(c.created, missing...)
	}
	c.j.Files = append(c.j.Files, t.jf)
	return nil
}

// recordDir journals a directory to remove, with its permissions, if it is one now.
func (c *commit) recordDir(t *commitFile) error {
	info, err := c.fsys.Stat(t.abs)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf(fmtFileErr, t.display, err)
	}
	if !info.IsDir() {
		return nil
	}
	t.existed = true
	if c.modes {
		t.mode = permOf(info)
	}
	c.j.Removed = append(c.j.Removed, removedDir{Path: relIn(c.dir, t.abs), Mode: t.mode})
	return nil
}

// perm is abs's permissions where the FS sets them, nil elsewhere (log-2026-09-29 M4 U4c-r).
func (c *commit) perm(abs, display string) (*uint32, error) {
	if !c.modes {
		return nil, nil
	}
	info, err := c.fsys.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf(fmtFileErr, display, err)
	}
	return permOf(info), nil
}

// missing is dir and each parent of it that does not exist.
func (c *commit) missing(dir string) ([]string, error) {
	var out []string
	for d := dir; d != project.DirOf(d); d = project.DirOf(d) {
		_, err := c.fsys.Stat(d)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// writeJournal writes the journal and syncs its directory, and each directory made for it,
// before any file changes (API.md N10; log-2026-09-29 M4 U4c-r).
func (c *commit) writeJournal() error {
	data, err := c.j.encode()
	if err != nil {
		return err
	}
	jdir := project.DirOf(c.name)
	made, err := c.missing(jdir)
	if err != nil {
		return err
	}
	if err := c.fsys.MkdirAll(jdir); err != nil {
		return err
	}
	if err := c.fsys.WriteFile(c.name, data); err != nil {
		_ = c.fsys.Remove(c.name) // what a failed write left, if anything: no file has changed
		return err
	}
	syncs := []string{jdir}
	for _, d := range made {
		syncs = append(syncs, project.DirOf(d))
	}
	if err := syncDirs(c.fsys, syncs); err != nil {
		return c.rollback(err)
	}
	return nil
}

// stageAll writes every new content beside its target, stopping at a cancellation: nothing is
// renamed yet (API.md N10, S11).
func (c *commit) stageAll(ctx context.Context) error {
	for _, t := range c.targets {
		if t.remove {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		fresh, err := c.missing(project.DirOf(t.abs))
		if err == nil {
			err = c.fsys.MkdirAll(project.DirOf(t.abs))
		}
		if err != nil {
			return fmt.Errorf(fmtFileErr, t.display, err)
		}
		c.made = append(c.made, fresh...)
		if err := stage(c.fsys, t.abs, t.data, t.mode); err != nil {
			return fmt.Errorf(fmtFileErr, t.display, err)
		}
	}
	return ctx.Err()
}

// apply renames every stage over its target, removes the deleted files, then the emptied
// directories, syncs every directory it touched, then removes the journal: from the first
// rename on, a cancellation no longer stops it (API.md N6, N10, S11).
func (c *commit) apply() error {
	for i, t := range c.targets {
		if t.remove {
			continue
		}
		c.done = append(c.done, i) // before the rename: an undone rename is left alone, being old
		if err := c.fsys.Rename(stageName(t.abs), t.abs); err != nil {
			return fmt.Errorf(fmtFileErr, t.display, err)
		}
	}
	for i, t := range c.targets {
		if !t.remove || t.dir {
			continue
		}
		c.done = append(c.done, i)
		if err := c.fsys.Remove(t.abs); err != nil {
			return fmt.Errorf(fmtFileErr, t.display, err)
		}
	}
	if err := c.dropDirs(); err != nil {
		return err
	}
	if err := syncDirs(c.fsys, c.touched()); err != nil {
		return err
	}
	return c.fsys.Remove(c.name)
}

// dropDirs removes, deepest first, each journaled directory that is empty now; one that is
// not stays, and that is no failure (API.md N6).
func (c *commit) dropDirs() error {
	var dirs []int
	for i, t := range c.targets {
		if t.dir && t.existed {
			dirs = append(dirs, i)
		}
	}
	slices.SortFunc(dirs, func(a, b int) int { return deepestFirst(c.targets[a].abs, c.targets[b].abs) })
	for _, i := range dirs {
		t := c.targets[i]
		entries, err := c.fsys.ReadDir(t.abs)
		if errors.Is(err, fs.ErrNotExist) || err == nil && len(entries) > 0 {
			continue
		}
		c.done = append(c.done, i)
		if err == nil {
			err = c.fsys.Remove(t.abs)
		}
		if err != nil {
			return fmt.Errorf(fmtFileErr, t.display, err)
		}
	}
	return nil
}

// touched are the directories whose entries the commit changed, to sync before the journal goes.
func (c *commit) touched() []string {
	var dirs []string
	for _, t := range c.targets {
		dirs = append(dirs, project.DirOf(t.abs))
	}
	for _, d := range c.created {
		dirs = append(dirs, project.DirOf(d))
	}
	return dirs
}

// spot places one of the commit's own names; vet found every one confined.
func (c *commit) spot(abs string) spot {
	s, _ := confine(c.at, relIn(c.dir, abs))
	return s
}

// rollback puts back what this commit changed, and only that, then removes the journal; a
// rollback that cannot finish keeps the journal for Recover (API.md N11; log-2026-09-29 M4 U4c-r).
func (c *commit) rollback(cause error) error {
	u := &undo{}
	for _, d := range c.made {
		u.created = append(u.created, c.spot(d))
	}
	for _, i := range c.done {
		t := c.targets[i]
		if t.dir {
			u.removed = append(u.removed, undoDir{spot: c.spot(t.abs), mode: t.mode})
			continue
		}
		jf := t.jf
		if t.existed {
			jf.Mode = t.mode // this process's own rollback keeps every permission bit
		}
		u.files = append(u.files, undoFile{spot: c.spot(t.abs), journalFile: jf})
	}
	for _, t := range c.targets {
		if !t.remove {
			u.stages = append(u.stages, t.abs)
		}
	}
	if _, err := u.run(c.fsys); err != nil {
		return errors.Join(cause, journalRefusal(ErrJournal, err))
	}
	if err := c.fsys.Remove(c.name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errors.Join(cause, err)
	}
	return cause
}

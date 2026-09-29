package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/fantasim/canonlang/internal/project"
)

// osFiles is a file system that may say it is the operating system's, whose changes the OS
// notifies; an embedder's wrapper of the OS's files opts in with an OSBacked method (W12).
type osFiles interface {
	OSBacked() bool
}

func osBacked(fsys project.FS) bool {
	o, ok := fsys.(osFiles)
	return ok && o.OSBacked()
}

// notifier is what tells the hub of changes to the OS's files: fsnotify, or a test's.
type notifier interface {
	add(name string) error
	remove(name string) error
	close() error
	events() <-chan fsnotify.Event
	errs() <-chan error
}

type fsNotifier struct{ w *fsnotify.Watcher }

func openNotifier() (notifier, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf(fmtWrap, errWatch, err)
	}
	return fsNotifier{w}, nil
}

func (n fsNotifier) add(name string) error         { return watchErr(n.w.Add(name)) }
func (n fsNotifier) remove(name string) error      { return watchErr(n.w.Remove(name)) }
func (n fsNotifier) close() error                  { return watchErr(n.w.Close()) }
func (n fsNotifier) events() <-chan fsnotify.Event { return n.w.Events }
func (n fsNotifier) errs() <-chan error            { return n.w.Errors }

func watchErr(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(fmtWrap, errWatch, err)
}

// watchOS starts the OS's notifications on every directory the snapshot read in or listed;
// when they cannot start, the hub polls instead (API.md W12).
func (h *hub) watchOS(open func() (notifier, error)) {
	n, err := open()
	if err != nil {
		h.fallBack(err)
		return
	}
	h.os, h.dirs, h.covered = n, map[string]bool{}, map[string]string{}
	h.resync(h.clk.Now())
}

// fallBack stops the OS's notifications, which failed, and says so once: the hub polls the
// snapshot from now on (API.md S1).
func (h *hub) fallBack(err error) {
	h.log.Warn(msgPolling, logError, err)
	h.closeOS()
}

func (h *hub) closeOS() {
	if h.os != nil {
		_ = h.os.close()
		h.os = nil
	}
}

// notified notes a change the OS reported; a directory removed or renamed lost its watch, which
// the next resync adds again, and a missing directory under the one notified is tried again.
func (h *hub) notified(ev fsnotify.Event) {
	name := filepath.ToSlash(ev.Name)
	if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
		delete(h.dirs, name)
	}
	now := h.clk.Now()
	h.note(now)
	if !h.coveredBy(name) && !h.coveredBy(path.Dir(name)) {
		return
	}
	if err := h.retry(now); err != nil {
		h.fallBack(err)
	}
}

// coveredBy reports dir watched in place of a missing directory below it.
func (h *hub) coveredBy(dir string) bool {
	return slices.Contains(slices.Collect(maps.Values(h.covered)), dir)
}

// resync follows the directories the current snapshot reads, once per snapshot state, and tries
// again every missing one; a failure makes the hub poll instead.
func (h *hub) resync(now time.Time) {
	s, _, err := h.p.current()
	if err != nil {
		return
	}
	if gen := s.fs.generation(); s.fs != h.seen || gen != h.seenGn {
		h.seen, h.seenGn = s.fs, gen
		err = h.follow(now, s.fs.watchDirs())
	}
	if err == nil {
		err = h.retry(now)
	}
	if err != nil {
		h.fallBack(err)
	}
}

// follow watches every directory of want, or the nearest one above it that exists, and no other;
// a directory added is a change the notifications may have missed, noted at now.
func (h *hub) follow(now time.Time, want []string) error {
	kept, covered := map[string]bool{}, map[string]string{}
	for _, d := range want {
		got, added, err := h.add(d)
		if err != nil {
			return err
		}
		kept[got] = true
		if got != d {
			covered[d] = got
		}
		if added {
			h.note(now)
		}
	}
	h.covered = covered
	for _, d := range slices.Sorted(maps.Keys(h.dirs)) {
		if !kept[d] {
			_ = h.os.remove(filepath.FromSlash(d))
			delete(h.dirs, d)
		}
	}
	return nil
}

// retry tries every missing directory again, watched from now on if it now exists.
func (h *hub) retry(now time.Time) error {
	for _, d := range slices.Sorted(maps.Keys(h.covered)) {
		got, added, err := h.add(d)
		if err != nil {
			return err
		}
		if got == d {
			delete(h.covered, d)
		}
		if added {
			h.note(now)
		}
	}
	return nil
}

// add watches d, or the nearest directory above it that exists, whose change shows d appearing;
// it returns the directory watched and whether it is new.
func (h *hub) add(d string) (string, bool, error) {
	for {
		if h.dirs[d] {
			return d, false, nil
		}
		err := h.os.add(filepath.FromSlash(d))
		if err == nil {
			h.dirs[d] = true
			return d, true, nil
		}
		if up := path.Dir(d); errors.Is(err, fs.ErrNotExist) && up != d {
			d = up
			continue
		}
		return d, false, err
	}
}

// watchDirs is every directory an entry of s is in, or is when it is a listing, and every place
// a link leads to: what the OS's notifications cover so that a change to what s read shows.
func (s *snapFS) watchDirs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	dirs := map[string]bool{s.root: true}
	//canon:unordered each entry adds its directory to a set, sorted below
	for n, e := range s.ents {
		d := path.Dir(n.abs)
		switch {
		case e.err != nil:
		case n.kind == kindDir:
			d = n.abs
		case n.kind == kindLink:
			dirs[string(e.data)] = true
		}
		dirs[d] = true
	}
	return slices.Sorted(maps.Keys(dirs))
}

// generation counts the entries s has read so far, so a resync knows when it read more.
func (s *snapFS) generation() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gen
}

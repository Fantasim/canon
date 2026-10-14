package edit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"path"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/writelock"
)

// Locker takes the writer locks of the packages a journal records (DECISIONS 375).
type Locker interface {
	// TryLock takes every package exclusive, all or none, never waiting.
	TryLock(pkgs []string) (release func() error, ok bool, err error)
}

// loaded is a journal file read: gone since it was listed, torn (it does not decode), or decoded,
// locked when it records a valid lock list, bad when its lock list refuses it.
type loaded struct {
	j                       *journal
	gone, torn, locked, bad bool
}

// loadJournal reads and decodes the journal file name.
func loadJournal(fsys build.WriteFS, name string) (loaded, error) {
	data, err := fsys.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return loaded{gone: true}, nil
	}
	if err != nil {
		return loaded{}, err
	}
	return decodeJournal(data), nil
}

// decodeJournal is a journal's JSON decoded, torn when it does not decode.
func decodeJournal(data []byte) loaded {
	var j journal
	var probe struct {
		Locks json.RawMessage `json:"locks"`
	}
	if json.Unmarshal(data, &j) != nil || json.Unmarshal(data, &probe) != nil {
		return loaded{torn: true}
	}
	listed := probe.Locks != nil && (j.Version == journalVersion || j.Version == journalVersionPID)
	bad := listed && (j.Version == journalVersionPID || !validLocks(j.Locks)) // API.md N11
	return loaded{j: &j, locked: listed && !bad, bad: bad}
}

// validLocks says whether locks is a non-empty list of sorted, unique package names.
func validLocks(locks []string) bool {
	for i, p := range locks {
		if writelock.Valid(p) != nil || i > 0 && locks[i-1] >= p {
			return false
		}
	}
	return len(locks) > 0
}

// heldOf is the lock list a commit's journal records: the packages s holds exclusive, sorted and
// unique, none where s takes no lock; a Locker holding none is refused (DECISIONS 375: J2, J12).
func heldOf(s Site) ([]string, error) {
	if s.Locks == nil {
		return nil, nil
	}
	if len(s.Held) == 0 {
		return nil, errNoHeld
	}
	held := slices.Compact(slices.Sorted(slices.Values(s.Held)))
	for _, p := range held {
		if err := writelock.Valid(p); err != nil {
			return nil, err
		}
	}
	return held, nil
}

// lockJournals are the lock lists of this host's journals recording locks, in name order.
func lockJournals(s Site) ([][]string, error) {
	names, err := journals(s.FS, s.Layout.Dir())
	if err != nil {
		return nil, err
	}
	var out [][]string
	for _, name := range names {
		l, err := loadJournal(s.FS, name)
		if err != nil {
			return nil, err
		}
		if l.locked && l.j.Host == s.Self.Host {
			out = append(out, l.j.Locks)
		}
	}
	return out, nil
}

// Meeting is every package of the transitive closure of this host's journals whose locks meet
// holding, the packages held in either mode; each is to be held exclusive (DECISIONS 375: J4).
func Meeting(s Site, holding []string) ([]string, error) {
	if s.Locks == nil {
		return nil, nil
	}
	lists, err := lockJournals(s)
	if err != nil {
		return nil, err
	}
	return closure(lists, holding), nil
}

// closure is the union of the lists reached from holding, a list reaching every package of it.
func closure(lists [][]string, holding []string) []string {
	reach, need := map[string]bool{}, map[string]bool{}
	for _, p := range holding {
		reach[p] = true
	}
	taken := make([]bool, len(lists))
	for grown := true; grown; {
		grown = false
		for i, locks := range lists {
			if taken[i] || !slices.ContainsFunc(locks, func(p string) bool { return reach[p] }) {
				continue
			}
			taken[i], grown = true, true
			for _, p := range locks {
				reach[p], need[p] = true, true
			}
		}
	}
	return slices.Sorted(maps.Keys(need))
}

// Settle rolls back, in name order, this host's journals all of whose locks s holds exclusive
// (Held), and removes them; changed are the files it restored or removed (API.md N11; DECISIONS 375: J4).
func Settle(s Site, logger *slog.Logger) (changed []string, err error) {
	if s.Locks == nil {
		return nil, nil
	}
	names, err := journals(s.FS, s.Layout.Dir())
	if err != nil {
		return nil, err
	}
	t := tally{removedOnly: true}
	for _, name := range names {
		r, err := settleOne(s, name)
		t.add(name, r, err)
		if r != nil {
			for _, f := range r.files {
				changed = append(changed, s.Layout.Display(f))
			}
		}
	}
	t.log(logger)
	slices.Sort(changed)
	return slices.Compact(changed), errors.Join(t.errs...)
}

// settleOne rolls back the journal name if it is this host's and s holds all its locks; nil when
// it is left: gone, torn, recording no lock, another host's, or not covered.
func settleOne(s Site, name string) (*undoer, error) {
	l, err := loadJournal(s.FS, name)
	if err != nil || !l.locked || l.j.Host != s.Self.Host || !holdsAll(s.Held, l.j.Locks) {
		return nil, err
	}
	return rollBack(s, name, l.j)
}

// blocked refuses a commit on the first journal that concerns it: an edit unfinished, or still
// running, whose rollback could undo this one (API.md N11; DECISIONS 375: J6).
func (c *commit) blocked() error {
	names, err := journals(c.fsys, c.dir)
	if err != nil {
		return err
	}
	for _, name := range names {
		refused, err := c.blocks(name)
		if err != nil {
			return err
		}
		if refused {
			return fmt.Errorf(fmtFileErr, path.Base(name), ErrJournal)
		}
	}
	return nil
}

// blocks says whether the journal name refuses the commit: any journal when it takes no lock;
// otherwise one at its own name, one that records no lock or is another host's, or one whose
// locks meet its own, Settle having left it. One gone since it was listed does not.
func (c *commit) blocks(name string) (bool, error) {
	if name == c.name || len(c.j.Locks) == 0 {
		return true, nil
	}
	l, err := loadJournal(c.fsys, name)
	if err != nil || l.gone {
		return false, err
	}
	return !l.locked || l.j.Host != c.j.Host || Meets(l.j.Locks, c.j.Locks), nil
}

// holdsAll says whether every package of locks is in held.
func holdsAll(held, locks []string) bool {
	for _, p := range locks {
		if !slices.Contains(held, p) {
			return false
		}
	}
	return true
}

// Meets says whether a and b share a package.
func Meets(a, b []string) bool {
	return slices.ContainsFunc(a, func(p string) bool { return slices.Contains(b, p) })
}

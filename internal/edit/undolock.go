package edit

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/lock"
)

// overLocks puts into over each canon.lock as the edit writes it (API.md E20), so a dry run of
// its Undo is checked against the lock facts the edit leaves (E22, E23).
func (a *applier) overLocks(over *overlayFS) error {
	if len(a.locked) == 0 || a.facts == nil {
		return nil
	}
	locks, err := a.snap.a.LocksWith(lockIDs(a.locked))
	if err != nil {
		return err
	}
	for _, l := range locks {
		over.files[l.Abs] = l.Content
	}
	return nil
}

// lockBroken reports a lock finding (E6001, E6002) the last dry run holds and the base does not:
// a lock fact the Undo would undo (API.md E23).
func (c *undoCheck) lockBroken() bool {
	if c.a.facts == nil {
		return false
	}
	var pkgs []string
	for _, r := range c.roots {
		pkgs = append(pkgs, r.Package)
	}
	for _, pkg := range slices.Compact(slices.Sorted(slices.Values(pkgs))) {
		before := lockFindings(c.a.base, pkg)
		for _, f := range lockFindings(c.last, pkg) {
			if !slices.Contains(before, f) {
				return true
			}
		}
	}
	return false
}

// lockFindings are the lock findings of pkg in s, each as its code, its file, and its value path,
// or for none the text it is reported at, a lock line or a source item (API.md E23).
func lockFindings(s *Snapshot, pkg string) []string {
	bag := s.a.Bag(pkg)
	if bag == nil {
		return nil
	}
	files := s.a.Files()
	var out []string
	for _, f := range bag.Findings() {
		if f.Code != diag.E6001.Def().Code && f.Code != diag.E6002.Def().Code {
			continue
		}
		at := f.Path
		if text := files.Content(f.Span.File); at == "" && int(f.Span.End) <= len(text) && f.Span.Start <= f.Span.End {
			at = string(text[f.Span.Start:f.Span.End])
		}
		out = append(out, strings.Join([]string{string(f.Code), files.Path(f.Span.File), at}, findingSep))
	}
	return out
}

// lockIDs are locked as build names them.
func lockIDs(locked []Locked) []build.LockID {
	ids := make([]build.LockID, len(locked))
	for i, l := range locked {
		ids[i] = build.LockID{Name: l.Name, Key: l.Key}
	}
	return ids
}

// skipped are the ids of locked whose lines E20 skipped, an AllowErrors conflict: neither the lock
// as read nor as the edit writes it holds them, so they were never locked (API.md E23).
func (a *applier) skipped(locked []Locked) ([]Locked, error) {
	if !a.allowErrors || len(locked) == 0 {
		return nil, nil
	}
	locks, err := a.snap.a.LocksWith(lockIDs(locked))
	if err != nil {
		return nil, err
	}
	var facts []lock.Fact
	for _, l := range locks {
		f, _ := lock.Parse(0, l.Content, l.Package, diag.NewBag(a.snap.a.Files(), l.Package))
		facts = append(facts, f.Facts()...)
	}
	var out []Locked
	for _, id := range locked {
		written := slices.ContainsFunc(facts, func(f lock.Fact) bool {
			return f.Kind != lock.KindField && f.Name == id.Name && f.Holder == id.Key
		})
		if !written && !a.base.a.LockHolds(build.LockID{Name: id.Name, Key: id.Key}) {
			out = append(out, id)
		}
	}
	return out, nil
}

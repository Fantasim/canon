package build

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/lock"
	"github.com/fantasim/canonlang/internal/types"
)

// LockID is an id an edit adds or retires: its collection as canon.lock names it, and its key.
type LockID struct {
	Name, Key string
}

// LocksWith is each selected package's canon.lock as read with the current facts of ids added, an
// id once its entry or member and every @stable field evaluated; a lock they add no line to is
// left out, as is every lock under a layer (API.md E20, B1a; log-2026-09-29 M4 U5b-r).
func (a *Analysis) LocksWith(ids []LockID) ([]Lock, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(ids) == 0 || len(a.r.p.opt.Layers) > 0 {
		return nil, nil
	}
	var out []Lock
	for _, st := range a.r.locks {
		l, err := st.with(ids, a.r.stableFields)
		if err != nil {
			return nil, err
		}
		if l != nil {
			out = append(out, *l)
		}
	}
	return out, nil
}

// with is st's lock as read with the current facts of each whole, conflict-free id of ids added;
// nil when that changes nothing or the lock does not parse whole.
func (st *lockState) with(ids []LockID, stable func(string) int) (*Lock, error) {
	if !st.whole {
		return nil, nil
	}
	current := lock.New(st.pkg)
	if _, err := current.Update(st.sources); err != nil {
		return nil, fmt.Errorf(fmtPackage, st.pkg, err)
	}
	file := st.fresh()
	before := lineSet(file.Format())
	if err := addAll(file, candidates(current.Facts(), ids, stable, file.Facts())); err != nil {
		return nil, fmt.Errorf(fmtPackage, st.pkg, err)
	}
	content := file.Format()
	if bytes.Equal(content, st.raw) || st.raw == nil && len(file.Facts()) == 0 {
		return nil, nil
	}
	out := &Lock{Package: st.pkg, Path: st.path, Abs: st.abs, Content: content, Status: StatusWritten}
	for _, line := range strings.SplitAfter(string(content), lineEnd) {
		if line != "" && !before[line] {
			out.Lines = append(out.Lines, strings.TrimSuffix(line, lineEnd))
		}
	}
	if len(out.Lines) == 0 { // no id of this package added: its lock, canonical or not, is not touched (E20, LOCK.md §2.4)
		return nil, nil
	}
	return out, nil
}

// candidates: whole ids, each new fact unique (LOCK.md §1 E3102, §4.4; log-2026-09-29 M4 U5b-r3, U7b, B8-r).
func candidates(current []lock.Fact, ids []LockID, stable func(string) int, held []lock.Fact) [][]lock.Fact {
	var out [][]lock.Fact
	clash := func(f lock.Fact) bool { return conflicts(held, f) || !holds(held, f) && conflicts(current, f) }
	for _, id := range ids {
		own := slices.DeleteFunc(slices.Clone(current), func(f lock.Fact) bool { return f.Name != id.Name || f.Holder != id.Key })
		if whole(own, stable(id.Name)) && !slices.ContainsFunc(own, clash) {
			out = append(out, own)
		}
	}
	return out
}

// addAll adds to file the facts of each candidate.
func addAll(file *lock.File, cands [][]lock.Fact) error {
	for _, own := range cands {
		for _, f := range own {
			if _, err := file.Add(f); err != nil {
				return err
			}
		}
	}
	return nil
}

// conflicts reports a fact a tool never writes beside facts: in its collection, its holder with
// another value there, or its value with another holder (E6002 against a lock, E3102 against the
// sources).
func conflicts(held []lock.Fact, f lock.Fact) bool {
	if f.Kind == lock.KindTable {
		return false
	}
	return slices.ContainsFunc(held, func(h lock.Fact) bool {
		return h.Kind == f.Kind && h.Name == f.Name && h.Field == f.Field && (h.Holder == f.Holder) != (h.Value == f.Value)
	})
}

// holds reports held already having f's value for f's holder, retired or not.
func holds(held []lock.Fact, f lock.Fact) bool {
	return slices.ContainsFunc(held, func(h lock.Fact) bool {
		return h.Kind == f.Kind && h.Name == f.Name && h.Field == f.Field && h.Holder == f.Holder && h.Value == f.Value
	})
}

// LockHolds reports an id the selected packages' locks, as read, already hold (API.md E20;
// log-2026-09-29 M4 U5b-r3).
func (a *Analysis) LockHolds(id LockID) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.ContainsFunc(a.r.locks, func(st *lockState) bool {
		return st.file != nil && slices.ContainsFunc(st.file.Facts(), func(f lock.Fact) bool {
			return f.Kind != lock.KindField && f.Name == id.Name && f.Holder == id.Key
		})
	})
}

// whole reports an id's facts that evaluated: its entry's or member's, and one per @stable field.
func whole(facts []lock.Fact, fields int) bool {
	own := slices.IndexFunc(facts, func(f lock.Fact) bool { return f.Kind != lock.KindField })
	return own >= 0 && len(facts) == fields+1
}

// stableFields is how many @stable fields the entries of the stable table named name have; 0
// for another collection.
func (r *run) stableFields(name string) int {
	for _, cp := range r.prog.Packages {
		for _, obj := range cp.Decls {
			if obj.Kind() == check.ObjLet && cp.Path+qnameSep+obj.Name() == name {
				return stableCount(obj.Type())
			}
		}
	}
	return 0
}

// stableCount is the number of @stable fields of a table type's element record.
func stableCount(t types.Type) int {
	tt, ok := t.Base().(*types.TableType)
	if !ok {
		return 0
	}
	rt, ok := tt.Elem.Base().(*types.RecordType)
	if !ok {
		return 0
	}
	n := 0
	for _, f := range rt.Fields {
		if f.Stable {
			n++
		}
	}
	return n
}

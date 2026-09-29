package build

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// API.md P8 (log-2026-09-29 M4 U13-r): every example's keyed lists and tables hold identified records.
func TestKeyedHaveIdentitiesExamples(t *testing.T) {
	if testing.Short() {
		t.Skip("every example, analyzed")
	}
	_, cold := examplesAnalyzer(t).pair(t)
	keyedHaveIdentities(t, "examples", cold)
	t.Logf("%d values walked", len(cold.settled))
}

// identityWalk walks settled values as rules names their parts, each node once.
type identityWalk struct {
	t    *testing.T
	name string
	seen map[value.Value]bool
}

// keyedHaveIdentities fails when a record sits in a keyed list or a table of a settled value
// without an identity: rules builds its path index lazily on it (rules/paths.go).
func keyedHaveIdentities(t *testing.T, name string, a *Analysis) {
	t.Helper()
	w := &identityWalk{t: t, name: name, seen: map[value.Value]bool{}}
	declared := verify.DeclaredTypes(a.Program())
	for root, v := range a.settled { //canon:unordered each value is walked whole
		w.walk(v, declared[root])
	}
}

func (w *identityWalk) walk(v value.Value, dt types.Type) {
	if v == nil || w.seen[v] {
		return
	}
	w.seen[v] = true
	switch x := v.(type) {
	case *value.Record:
		for i, f := range verify.Fields(x.T) {
			if i < len(x.Fields) {
				w.walk(x.Fields[i], f.Type)
			}
		}
	case *value.List:
		w.list(x, dt)
	case *value.Map:
		for i, k := range x.Keys {
			w.walk(k, nil)
			if i < len(x.Vals) {
				w.walk(x.Vals[i], nil)
			}
		}
	case *value.Table:
		for _, e := range x.Entries {
			w.identified(e)
			w.walk(e, nil)
		}
	case *value.Pair:
		w.walk(x.A, nil)
		w.walk(x.B, nil)
	}
}

// list walks a list's elements, each record of a keyed one identified.
func (w *identityWalk) list(l *value.List, dt types.Type) {
	lt, declared := verify.Declared(dt).(*types.ListType)
	var et types.Type
	if declared {
		et = lt.Elem
	} else {
		lt, _ = verify.Declared(l.T).(*types.ListType)
	}
	for _, e := range l.Elems {
		if rec, ok := e.(*value.Record); ok && lt != nil && lt.KeyedBy != nil {
			w.identified(rec)
		}
		w.walk(e, et)
	}
}

func (w *identityWalk) identified(rec *value.Record) {
	if rec != nil && rec.Ident == nil {
		w.t.Errorf("%s: a record in a keyed list or a table has no identity: %s", w.name, rec.CanonText())
	}
}

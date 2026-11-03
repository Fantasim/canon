package check_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
)

const (
	flatTypes = "dep/types.canon"
	flatUse   = "dep/use.canon"
	flatSteps = 200
	// flatEdit is the value of the let `size` with its step number, which each edit changes; the anchor stops at the next declaration.
	flatEdit = "len() + %d\n\n/// A sum"
	// flatKeptField is the checker field memoizing KeptDepMap.
	flatKeptField = "kept"
)

// flatWorld declares dependent maps in one file and reads them in two lets of another; the edits
// change the value of `size` alone (a Recheck takes an annotated let changed only in its value).
var flatWorld = map[string]string{
	flatTypes: `package dep

/// A kind.
enum K { num, str }

/// An event.
record Ev {
  /// Its kind.
  k: K
}

/// An item.
record Item {
  /// Its count.
  n: Int = 1
}

/// Items.
let items: table Item = {
  one {}
}

/// What an event holds.
type P(e: Ev) = match e.k {
  num => String
  str => ref items
}

/// A holder.
record R {
  /// Its event.
  ev: Ev
  /// Per item.
  byItem: {i in items: P(ev)} = {}
  /// Counts.
  counts: {i in items: Int} = {}
}

/// A holder.
let h: R = { ev: { k: num } }
`,
	flatUse: `package dep

/// A size.
let size: Int = h.counts.len() + 0

/// A sum.
let sum: Int = h.byItem.len() + 0
`,
}

// infoLens are the lengths of the maps of a program's Info, by field name.
func infoLens(prog *check.Program) map[string]int {
	lens := map[string]int{}
	v := reflect.ValueOf(*prog.Info)
	for i := range v.NumField() {
		if f := v.Field(i); f.Kind() == reflect.Map {
			lens[v.Type().Field(i).Name] = f.Len()
		}
	}
	return lens
}

// IMPLEMENTATION-PLAN §7.6 NFR-02, ADR-0026 §15.9: every checker map and slice and every Info map keep one length across Rechecks.
func TestRecheckLineageKeepsCheckerFlat(t *testing.T) {
	w := newWorld(t, flatWorld)
	prog, s, _ := w.session()
	checkerAtStart, infoAtStart := check.CheckerLens(s), infoLens(prog)
	if checkerAtStart[flatKeptField] == 0 {
		t.Fatal("the world reads no dependent map: the memo is not exercised")
	}
	for n := 1; n <= flatSteps; n++ {
		next, ns, _, ok := w.recheck(s, w.edit(flatUse, fmt.Sprintf(flatEdit, n-1), fmt.Sprintf(flatEdit, n)))
		if !ok {
			t.Fatalf("step %d: Recheck refused", n)
		}
		if got := check.CheckerLens(ns); !reflect.DeepEqual(got, checkerAtStart) {
			t.Fatalf("step %d: checker lengths %v, want %v", n, got, checkerAtStart)
		}
		if got := infoLens(next); !reflect.DeepEqual(got, infoAtStart) {
			t.Fatalf("step %d: Info map lengths %v, want %v", n, got, infoAtStart)
		}
		s = ns
	}
}

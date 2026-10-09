package eval

import (
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
)

// The evaluator's file table is built once and shared: every capture bag of every expect reads
// the same map, so a bag the verify memo keeps pins no table of its own (no spec rule: a memory
// bound of canon test).
func TestFileTableSharedAcrossCalls(t *testing.T) {
	f, prog := parseIndexed(t)
	ev := New(prog, nil, check.Bags{}, Options{})
	first, again := ev.files(), ev.files()
	if reflect.ValueOf(first).UnsafePointer() != reflect.ValueOf(again).UnsafePointer() {
		t.Fatalf("two calls built two file tables, want one shared")
	}
	if got := first.Path(f.Src.ID); got != f.Src.Path {
		t.Errorf("the table resolves the file to %q, want %q", got, f.Src.Path)
	}
	bare := New(nil, nil, check.Bags{}, Options{})
	if bare.files().Path(f.Src.ID) != "" {
		t.Errorf("an evaluator without a program resolves a file")
	}
	bare.index.add(f, f.Src.Path)
	if got := bare.files().Path(f.Src.ID); got != f.Src.Path {
		t.Errorf("a file indexed late resolves to %q, want %q", got, f.Src.Path)
	}
}

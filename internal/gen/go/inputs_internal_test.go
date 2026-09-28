package gogen

import (
	"errors"
	"path/filepath"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// TestE8302Message: ir.InputGetterFailure, the one renderer gen/go and gen/cpp share, renders testdata/findings/E8302_1.txtar's findings.txt exactly (ERRORS.md §1.6).
func TestE8302Message(t *testing.T) {
	paths, err := filepath.Glob("testdata/findings/E8302_*.txtar")
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob testdata/findings/E8302_*.txtar: %v", err)
	}
	for _, p := range paths {
		a, err := txtar.ParseFile(p)
		if err != nil {
			t.Fatal(err)
		}
		want := findingsOf(a)
		code, msg := ir.InputGetterFailure("a.Gen.apiKey")
		got := "panic: " + code + ": " + msg + "\n"
		if got != want {
			t.Errorf("%s: ir.InputGetterFailure = %q, want %q", p, got, want)
		}
	}
}

// CODEGEN.md §5.12: a reason ir names no text for (not a valid list) is a plan defect, ErrMalformed, never a failure line.
func TestInputReasonWithoutText(t *testing.T) {
	g := &gen{}
	if got := g.inputReason(ir.InputNotValid, ir.TypeRef{Kind: types.List}); got != "" || !errors.Is(g.err, ErrMalformed) {
		t.Errorf("inputReason = %q, err %v; want \"\" and ErrMalformed", got, g.err)
	}
	g = &gen{}
	if got := g.inputReason(ir.InputNotSet, ir.TypeRef{Kind: types.Int}); got == "" || g.err != nil {
		t.Errorf("inputReason(InputNotSet) = %q, err %v; want its text and no error", got, g.err)
	}
}

// findingsOf is the "findings.txt" file of a: "" when the archive has none.
func findingsOf(a *txtar.Archive) string {
	for _, f := range a.Files {
		if f.Name == "findings.txt" {
			return string(f.Data)
		}
	}
	return ""
}

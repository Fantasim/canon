package canon_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// depWrittenSrc is p: F's dependent x takes Float for k = a. Its default, a spread or a layer line
// passing an Int literal along keeps it kept as written (f, f4, f5, f6's y and x); a computed Int
// (G's default k.index, n, id(1)) is not, by a default, a spread or a layer line (g, w2, w3, w4).
const depWrittenSrc = "package p\n\n" +
	"enum K {\n  a\n  b\n}\n\n" +
	"type PF(k: K) = match k {\n  a => Float\n  b => Int\n}\n\n" +
	"fn id(i: Int) -> Int {\n  return i\n}\n\n" +
	"let n: Int = 1\n\n" +
	"record F {\n  k: K = a\n  x: PF(k) = 1\n}\n\n" +
	"record F2 {\n  k: K = a\n  y: PF(k) = 1\n  x: PF(k) = y\n}\n\n" +
	"record G {\n  k: K = a\n  x: PF(k) = k.index\n}\n\n" +
	"let f: F = {}\n\n" +
	"let f3: F = { x: 1 }\n\n" +
	"let f4: F = { ...f3 }\n\n" +
	"let f5: F = { x: 1 }\n\n" +
	"let f6: F2 = {}\n\n" +
	"let g: G = {}\n\n" +
	"let w1: F = { x: n }\n\n" +
	"let w2: F = { ...w1 }\n\n" +
	"let w3: F = { x: 1 }\n\n" +
	"let w4: F = { x: id(1) }\n"

// depWrittenLayer is dev: an Int literal into f5's x, a computed Int into w3's.
const depWrittenLayer = "amend f5 {\n  x: 2\n}\n\namend w3 {\n  x: n\n}\n"

// TYPES.md §11.4: an Int literal a default, spread or layer passes to a dependent field converts to its Float branch; a computed Int is E3802.
func TestDependentLiteralConvertsThroughCopies(t *testing.T) {
	p := openLayered(t, depWrittenSrc, depWrittenLayer, true)
	var e3802 []string
	for _, f := range mustCheck(t, p, "p").Findings {
		if f.Code == string(diag.E3802.Def().Code) {
			e3802 = append(e3802, f.Path)
		}
	}
	slices.Sort(e3802)
	if want := []string{"g.x", "w1.x", "w3.x", "w4.x"}; !slices.Equal(e3802, want) { // w2's is w1's, at its origin (EVALUATION.md §13)
		t.Errorf("TYPES.md §11.4: %s at %q, want %q", diag.E3802.Def().Code, e3802, want)
	}
	for _, path := range []string{"p:g.x", "p:w1.x", "p:w2.x", "p:w3.x", "p:w4.x"} {
		if got, ok := mustValue(t, p, path).Float(); ok {
			t.Errorf("TYPES.md §11.4: %s = Float %v, want the computed Int kept", path, got)
		}
	}
	for _, c := range []struct {
		path string
		want float64
	}{{"p:f.x", 1}, {"p:f3.x", 1}, {"p:f4.x", 1}, {"p:f5.x", 2}, {"p:f6.y", 1}, {"p:f6.x", 1}} {
		if got, ok := mustValue(t, p, c.path).Float(); !ok || got != c.want {
			t.Errorf("TYPES.md §11.4: %s = %v (Float %v), want %v", c.path, got, ok, c.want)
		}
	}
}

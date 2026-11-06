package canon_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// brokenFnHead is p: pk picks tt's y, uu is another list of S, and fx (body to fill, from line 17)
// returns a ref of tt; the layer gives pk's pick fx()'s result.
const (
	brokenFnHead = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\nrecord P {\n  pick: ref tt\n}\n\n" +
		"let tt: [S] keyed by name = [{ name: \"x\" }, { name: \"y\" }]\n\nlet uu: [S] keyed by name = [{ name: \"x\" }]\n\n" +
		"fn fx() -> ref tt {\n  %s\n}\n\nlet pk: P = { pick: %s }\n"
	brokenFnLayer = "package p\nlayer dev\n\namend pk {\n  pick: fx()\n}\n"
	layeredPick   = "tt.y" // pk's own pick, which the layer replaces
	basePick      = "fx()" // pk's pick reading fx itself
)

// brokenFnBodies are fx bodies holding a static E3002, which breaks fx (TYPES.md §1), and its line.
var brokenFnBodies = []struct {
	name, body string
	line       int
}{
	{"a record of another type", "return P { pick: tt.x }", 17},
	{"an untyped record literal", "return { name: \"x\" }", 17},
	{"a ref of another collection", "let r: ref uu = uu.x\n  return r", 18},
}

// e3002At is fx's E3002 at line of the main file, as causeOf and errorPlaces print it.
func e3002At(line int) []string {
	return []string{fmt.Sprintf("%s %s:%d", diag.E3002.Def().Code, srcMain, line)}
}

// errorPlaces is the error findings of fs, as codes and places.
func errorPlaces(fs []canon.Finding) []string {
	var out []string
	for _, f := range fs {
		if f.Severity == canon.SeverityError {
			out = append(out, fmt.Sprintf("%s %s:%d", f.Code, f.File, f.Line))
		}
	}
	return out
}

// brokenFnOpen opens p with fx's body, pk's pick and the dev layer, active when on.
func brokenFnOpen(t *testing.T, body, pick string, on bool) *canon.Project {
	t.Helper()
	files := map[string]string{srcMain: formatted(t, srcMain, fmt.Sprintf(brokenFnHead, body, pick)), sharedLayer: brokenFnLayer}
	var layers []string
	if on {
		layers = []string{"dev"}
	}
	p, _ := sharedOpen(t, files, layers)
	return p
}

// pickOf is pk's pick: its own when the layer gives it fx's result, else fx's.
func pickOf(on bool) string {
	if on {
		return layeredPick
	}
	return basePick
}

// TYPES.md §1, API.md R6 R7 R15, DECISIONS 514: pk is poisoned, fx's finding its cause, layer or not.
func TestLayerCallingBrokenFnPoisons(t *testing.T) {
	ctx := context.Background()
	for _, row := range brokenFnBodies {
		want := e3002At(row.line)
		for _, on := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s layer %v", row.name, on), func(t *testing.T) {
				p := brokenFnOpen(t, row.body, pickOf(on), on)
				_, err := p.Value(ctx, "p:pk")
				if got := causeOf(t, "Value", err); !slices.Equal(got, want) {
					t.Errorf("API.md R6, DECISIONS 514: Value's cause is %q, want fx's %q", got, want)
				}
				_, err = p.Refs(ctx, "p:tt[x]")
				if got := causeOf(t, "Refs", err); !slices.Equal(got, want) {
					t.Errorf("API.md R7: Refs's cause is %q, want fx's %q", got, want)
				}
				err = p.Graph(ctx, func(canon.GraphItem) error { return nil })
				if got := causeOf(t, "Graph", err); !slices.Equal(got, want) {
					t.Errorf("API.md R15: Graph's cause is %q, want fx's %q", got, want)
				}
			})
		}
	}
}

// EVALUATION.md §9.1, DECISIONS 472 514: the inactive layer is evaluated singly; fx's is the only error.
func TestInactiveLayerCallingBrokenFnChecks(t *testing.T) {
	ctx := context.Background()
	for _, row := range brokenFnBodies {
		t.Run(row.name, func(t *testing.T) {
			res, err := brokenFnOpen(t, row.body, layeredPick, false).Check(ctx)
			if err != nil {
				t.Fatalf("DECISIONS 514: Check fails %v, want fx's finding", err)
			}
			if got, want := errorPlaces(res.Findings), e3002At(row.line); !slices.Equal(got, want) {
				t.Errorf("EVALUATION.md §9.1: Check's errors are %q, want %q", got, want)
			}
		})
	}
}

// API.md E18 E19, DECISIONS 463 514: an edit's re-check meets the broken layer as a poisoned pk.
func TestEditUnderLayerCallingBrokenFn(t *testing.T) {
	ctx := context.Background()
	for _, row := range brokenFnBodies {
		for _, on := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s layer %v", row.name, on), func(t *testing.T) {
				p := brokenFnOpen(t, row.body, layeredPick, on)
				res, err := p.Edit(ctx, canon.Edit{AllowErrors: true, Ops: []canon.Op{canon.Set("p:tt[y].w", canon.Int(3))}})
				if err != nil {
					t.Fatalf("API.md E19, DECISIONS 514: the edit fails %v, want it written over fx's error", err)
				}
				if got, want := errorPlaces(res.Findings), e3002At(row.line); !res.Applied || !slices.Equal(got, want) {
					t.Errorf("API.md E18: applied %v with errors %q, want written with %q", res.Applied, got, want)
				}
			})
		}
	}
}

// EVALUATION.md §4.3 §7.1, DECISIONS 185 514: a soft membership finding keeps the unconverted record.
func TestLayerFnResultE3503Kept(t *testing.T) {
	ctx := context.Background()
	for _, on := range []bool{true, false} {
		t.Run(fmt.Sprintf("layer %v", on), func(t *testing.T) {
			p := brokenFnOpen(t, "return S { name: \"x\" }", pickOf(on), on)
			v, err := p.Value(ctx, "p:pk.pick")
			if err != nil {
				t.Fatalf("DECISIONS 185: Value fails %v, want the kept record", err)
			}
			if got := string(v.JSON()); got != `{"name":"x","w":0}` {
				t.Errorf("DECISIONS 185: pk.pick is %s, want the unconverted record", got)
			}
			if res, err := p.Refs(ctx, "p:tt[x]"); err != nil || len(res.Refs) != 0 {
				t.Errorf("API.md R7: Refs gives %v, %v; want no ref of tt's x", res, err)
			}
		})
	}
}

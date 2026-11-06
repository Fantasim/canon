package canon_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// layerPoisonSrc is p: pk picks one of tt's entries, and fx returns a ref into uu, another list.
const layerPoisonSrc = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\nrecord P {\n  pick: ref tt\n}\n\n" +
	"let tt: [S] keyed by name = [{ name: \"x\" }, { name: \"y\" }]\n\nlet uu: [S] keyed by name = [{ name: \"x\" }]\n\n" +
	"fn fx() -> ref uu {\n  return uu.x\n}\n\nlet pk: P = { pick: tt.y }\n"

// API.md R6, R7, R15 (DECISIONS 238, 503): an active layer's line giving pk's pick a ref of the
// wrong collection poisons pk: Refs and Graph fail ErrNoValue naming root p:pk with the findings
// Value gives for it, never none (a partial list never looks complete); inactive, Refs lists none.
func TestRefsFailureCarriesLayerFindings(t *testing.T) {
	files := map[string]string{srcMain: formatted(t, srcMain, layerPoisonSrc), sharedLayer: "package p\nlayer dev\n\namend pk {\n  pick: fx()\n}\n"}
	ctx := context.Background()
	p, _ := sharedOpen(t, files, []string{"dev"})
	_, err := p.Value(ctx, "p:pk")
	want := causeOf(t, "Value", err)
	if !slices.Equal(want, []string{fmt.Sprintf("%s %s:5", diag.E3002.Def().Code, sharedLayer)}) {
		t.Fatalf("API.md R6: Value gives the findings %q, want the layer line's type mismatch", want)
	}
	_, err = p.Refs(ctx, "p:tt[x]")
	if got := causeOf(t, "Refs", err); !slices.Equal(got, want) {
		t.Errorf("API.md R7, §15: Refs fails with the findings %q, want Value's %q", got, want)
	}
	err = p.Graph(ctx, func(canon.GraphItem) error { return nil })
	if got := causeOf(t, "Graph", err); !slices.Equal(got, want) {
		t.Errorf("API.md R15, §15: Graph fails with the findings %q, want Value's %q", got, want)
	}
	off, _ := sharedOpen(t, files, nil)
	if res, err := off.Refs(ctx, "p:tt[x]"); err != nil || len(res.Refs) != 0 {
		t.Errorf("API.md R7: with the layer inactive Refs gives %v, %v; want no ref", res, err)
	}
}

// causeOf is err, a PathError of ErrNoValue naming root p:pk, as its findings' codes and places.
func causeOf(t *testing.T, call string, err error) []string {
	t.Helper()
	var pe *canon.PathError
	if !errors.As(err, &pe) || !errors.Is(err, canon.ErrNoValue) || pe.Detail != "root p:pk" {
		t.Fatalf("API.md R7, §15: %s gives %v, want ErrNoValue naming root p:pk", call, err)
	}
	var out []string
	for _, f := range pe.Findings {
		out = append(out, fmt.Sprintf("%s %s:%d", f.Code, f.File, f.Line))
	}
	return out
}

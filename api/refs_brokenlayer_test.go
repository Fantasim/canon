package canon_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

const (
	// brokenTargetSrc is p: aa is broken (line 16), pk's map has no "zz", which the layer's line 9 amends.
	brokenTargetSrc = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\nrecord H {\n  m: {String: S}\n}\n\n" +
		"fn mk() -> {String: S} {\n  return { \"a\": { name: \"a\" } }\n}\n\nlet aa: S = { name: 1 }\n\nlet pk: H = { m: mk() }\n"
	brokenTargetLayer = "package p\nlayer dev\n\namend aa {\n  w: 1\n}\n\namend pk {\n  m[\"zz\"].w: 2\n}\n"
	// siblingSrc is p: k is broken (line 9); bb and pk are sound.
	siblingSrc = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\nfn k() -> Int {\n  return \"x\"\n}\n\n" +
		"let bb: S = { name: \"b\" }\n\nlet pk: S = { name: \"c\" }\n"
	pkW3 = "\n\namend pk {\n  w: 3\n}\n"
)

// placeOf is code at file:line, as causeOf and errorPlaces print a finding.
func placeOf(code diag.Code, file string, line int) string {
	return fmt.Sprintf("%s %s:%d", code, file, line)
}

// sortedErrors is the error findings of p's Check, as places, sorted.
func sortedErrors(t *testing.T, ctx context.Context, files map[string]string, layers []string) []string {
	t.Helper()
	p, _ := sharedOpen(t, files, layers)
	res, err := p.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return slices.Sorted(slices.Values(errorPlaces(res.Findings)))
}

// TYPES.md §1, DECISIONS 514: a layer amending a broken let is not broken; its other lines apply and report.
func TestLayerAmendingBrokenLetApplies(t *testing.T) {
	ctx := context.Background()
	files := map[string]string{srcMain: formatted(t, srcMain, brokenTargetSrc), sharedLayer: brokenTargetLayer}
	e1905 := placeOf(diag.E1905.Def().Code, sharedLayer, 9)
	want := []string{e1905, placeOf(diag.E3002.Def().Code, srcMain, 16)}
	for _, layers := range [][]string{{"dev"}, nil} {
		if got := sortedErrors(t, ctx, files, layers); !slices.Equal(got, want) {
			t.Errorf("EVALUATION.md §9.3, layers %v: Check's errors are %q, want %q", layers, got, want)
		}
	}
	p, _ := sharedOpen(t, files, []string{"dev"})
	_, err := p.Value(ctx, "p:pk")
	if got := causeOf(t, "Value", err); !slices.Equal(got, []string{e1905}) {
		t.Errorf("API.md R6: pk's cause is %q, want its own line's %q", got, e1905)
	}
	off, _ := sharedOpen(t, files, nil)
	if _, err := off.Value(ctx, "p:pk"); err != nil {
		t.Errorf("EVALUATION.md §9.3: without the layer pk gives %v, want its value", err)
	}
}

// API.md R6, DECISIONS 514: a let a broken active layer amends takes the layer's errors, else what it names, as its cause.
func TestBrokenLayerSiblingCause(t *testing.T) {
	ctx := context.Background()
	rows := []struct {
		name, line string
		want       string
	}{
		{"a sibling line calling a broken fn", "w: k()", placeOf(diag.E3002.Def().Code, srcMain, 9)},
		{"a sibling line in error", "w: \"x\"", placeOf(diag.E3002.Def().Code, sharedLayer, 5)},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			files := map[string]string{srcMain: formatted(t, srcMain, siblingSrc), sharedLayer: "package p\nlayer dev\n\namend bb {\n  " + row.line + "\n}" + pkW3}
			p, _ := sharedOpen(t, files, []string{"dev"})
			_, err := p.Value(ctx, "p:pk")
			if got := causeOf(t, "Value", err); !slices.Equal(got, []string{row.want}) {
				t.Errorf("API.md R6: pk's cause is %q, want %q", got, row.want)
			}
			off, _ := sharedOpen(t, files, nil)
			if _, err := off.Value(ctx, "p:pk"); err != nil {
				t.Errorf("EVALUATION.md §9.3: without the layer pk gives %v, want its value", err)
			}
		})
	}
}

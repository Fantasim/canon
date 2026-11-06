package canon_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// twoLetsSrc is p: bb spreads aa's second, dd spreads bb, ee holds a copy of dd as its entry e; rw
// reads x's w through dd, rr refers to x through dd's let path, rw2 reads x's w through ee's entry.
const twoLetsSrc = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\n" +
	"record A {\n  id: String\n  oo: [S] keyed by name\n}\n\n" +
	"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ name: \"x\" }] },\n" +
	"  { id: \"second\", oo: [{ name: \"x\" }, { name: \"y\" }] },\n]\n\n" +
	"let bb: A = { ...aa.second, id: \"third\" }\n\nlet dd: A = { ...bb }\n\n" +
	"let rw: Int = dd.oo.x.w\n\nlet rr: ref dd.oo = dd.oo.x\n\n" +
	"let ee: [A] keyed by id = [{ ...dd, id: \"e\" }]\n\nlet rw2: Int = ee.e.oo.x.w\n"

// API.md R7, R11, E12 (DECISIONS 489, 503): with lines amending dd and bb (bb's x made bb's own), second's
// x keeps its base reads through dd and ee, active or not: rw's, rw2's and rr's `code` refs and rr's
// computed `value` ref, with its edge; the rename is refused `computed` for rr.
func TestRefsBaseReadsThroughTwoAmendedLets(t *testing.T) {
	bodies := []struct{ name, body string }{
		{"dd then bb", "amend dd {\n  oo.y.w: 4\n}\n\namend bb {\n  oo.x.w: 5\n}\n"},
		{"bb then dd", "amend bb {\n  oo.x.w: 5\n}\n\namend dd {\n  oo.y.w: 4\n}\n"},
		{"bb alone", "amend bb {\n  oo.x.w: 5\n}\n"},
		{"dd alone", "amend dd {\n  oo.y.w: 4\n}\n"},
	}
	src := formatted(t, srcMain, twoLetsSrc)
	at := func(kind, let string) string {
		return fmt.Sprintf("%s %s:%d", kind, srcMain, lineOf(src, "let "+let+":"))
	}
	base := []string{at("code", "rw"), at("value", "rr"), at("code", "rr"), at("code", "rw2")}
	for _, b := range bodies {
		layer := "package p\nlayer dev\n\n" + b.body
		var want []string // the layer file sorts before p's (R8)
		if i := strings.Index(layer, "oo.x.w"); i >= 0 {
			want = append(want, fmt.Sprintf("layer %s:%d", sharedLayer, strings.Count(layer[:i], "\n")+1))
		}
		want = append(want, base...)
		for _, layers := range activeLayers {
			t.Run(fmt.Sprintf("%s active %v", b.name, layers != nil), func(t *testing.T) {
				files := map[string]string{srcMain: src, sharedLayer: layer}
				sharedRow{l: sharedLevel{path: secondX}, layers: layers, before: files, want: strings.Join(want, ", "), refused: canon.ReasonComputed, to: "z"}.run(t)
				p, _ := sharedOpen(t, files, layers)
				_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Rename(secondX, canon.Key("z"))}})
				var ne *canon.NotEditableError
				if !errors.As(err, &ne) || ne.Detail != "p:rr" {
					t.Errorf("API.md E12 (DECISIONS 503): the refusal is %v, want Detail p:rr", err)
				}
				if got := edgesTo(t, p, secondX)[secondX]; !slices.Equal(got, []string{edgeRR}) {
					t.Errorf("API.md R11 (DECISIONS 489): the edges to %s are %q, want %q", secondX, got, edgeRR)
				}
			})
		}
	}
}

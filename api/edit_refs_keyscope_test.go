package canon_test

import (
	"fmt"
	"strings"
	"testing"
)

// shadowSrc is items, a let shield naming item axe, and many, whose one holds elements keyed {s}
// and axe; r1 reads `ts[shield]`, which is the let: axe's n, 7.
const shadowSrc = "package p\n\nrecord I {\n  id: String\n}\n\nrecord T {\n  k: ref items\n  n: Int = 0\n}\n\n" +
	"record O {\n  id: String\n  ts: [T] keyed by k\n}\n\n" +
	"let items: [I] keyed by id = [{ id: \"{i}\" }, { id: \"axe\" }, { id: \"bow\" }]\n\n" +
	"let shield: ref items = axe\n\n" +
	"let many: [O] keyed by id = [{ id: \"one\", ts: [{ k: \"{o}\", n: 1 }, { k: \"axe\", n: 7 }] }]\n\n" +
	"let r1: Int = many.one.ts[shield].n\n"

// API.md R7, E11 (DECISIONS 318, 487): `[shield]` read as the let shield is no ref of items[shield]
// or of the element keyed shield, in an expression and an amend line, active or not; the rename
// leaves both tokens and r1's value as they were.
func TestRefsKeyTokenInScope(t *testing.T) {
	rows := []struct {
		path, to, renamed string
	}{
		{"p:items[shield]", "spear", "{i} {o}"},
		{"p:many[one].ts[shield]", "bow", "{o}"},
	}
	for _, active := range []bool{true, false} {
		var layers []string
		want := map[string]string{"p:r1": "7"}
		if active {
			layers, want["p:r1"] = []string{"dev"}, "5"
		}
		for _, r := range rows {
			t.Run(fmt.Sprintf("active %v %s", active, r.path), func(t *testing.T) {
				files := func(key string) map[string]string {
					var pairs []string
					for _, m := range []string{"{i}", "{o}"} {
						k := "shield"
						if strings.Contains(r.renamed, m) {
							k = key
						}
						pairs = append(pairs, m, k)
					}
					layer := "package p\nlayer dev\n\namend many {\n  one.ts[shield].n: 5\n}\n"
					return map[string]string{srcMain: formatted(t, srcMain, strings.NewReplacer(pairs...).Replace(shadowSrc)), sharedLayer: formatted(t, sharedLayer, layer)}
				}
				before := files("shield")
				if r.path != "p:items[shield]" { // the element's Refs only: its Undo writes `k: shield` bare, the let (see the report)
					p, _ := sharedOpen(t, before, layers)
					if _, refs := refsOf(t, p, r.path); len(refs) != 0 {
						t.Errorf("DECISIONS 318: the refs of %s are %q, want none", r.path, refs)
					}
					return
				}
				refs := fmt.Sprintf("value %s:%d", srcMain, lineOf(before[srcMain], "let many:"))
				sharedRow{l: sharedLevel{path: r.path}, layers: layers, before: before, after: files(r.to), want: refs, to: r.to}.run(t)
				p, _ := sharedOpen(t, files(r.to), layers)
				checkValues(t, p, "DECISIONS 318", want)
			})
		}
	}
}

// copyKeyFieldSrc is segElemSrc with cc, a spread copy of many's one, whose layer line replaces its
// element keyed shield whole.
const copyKeyFieldSrc = segElemSrc + "\nlet cc: O = { ...many.one, id: \"cc\" }\n"

// API.md R7, E11, E22 (DECISIONS 491; one key, one token): a line replacing a copy's element whole
// states its key twice; the literal's key field, written bare as an unresolved name typed `ref items`
// or as a string, is the same key, so it is a `layer` ref of the element and the rename rewrites it.
func TestRefsLayerLineReplacedRefKeyField(t *testing.T) {
	forms := []struct{ name, field string }{{"bare", "{k}"}, {"string", "\"{k}\""}}
	want := fmt.Sprintf("layer %s:5, layer %s:5", sharedLayer, sharedLayer)
	for _, f := range forms {
		for _, active := range []bool{true, false} {
			var layers []string
			if active {
				layers = []string{"dev"}
			}
			t.Run(fmt.Sprintf("%s active %v", f.name, active), func(t *testing.T) {
				files := func(key string) map[string]string {
					src := strings.NewReplacer("{i}", "shield", "{o}", key, "{t}", "shield").Replace(copyKeyFieldSrc)
					line := "ts[" + key + "]: { k: " + strings.ReplaceAll(f.field, "{k}", key) + ", n: 5 }"
					layer := "package p\nlayer dev\n\namend cc {\n  " + line + "\n}\n"
					return map[string]string{srcMain: formatted(t, srcMain, src), sharedLayer: formatted(t, sharedLayer, layer)}
				}
				sharedRow{l: sharedLevel{path: "p:many[one].ts[shield]"}, layers: layers, before: files("shield"), after: files("bow"), want: want, to: "bow"}.run(t)
			})
		}
	}
}

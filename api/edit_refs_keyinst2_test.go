package canon_test

import (
	"fmt"
	"strings"
	"testing"
)

// segElemSrc is items, and many, whose entries one and two each hold, in ts, an element keyed by a
// ref to item {o} ({t} in two's).
const segElemSrc = "package p\n\nrecord I {\n  id: String\n}\n\nrecord T {\n  k: ref items\n  n: Int = 0\n}\n\n" +
	"record O {\n  id: String\n  ts: [T] keyed by k\n}\n\n" +
	"let items: [I] keyed by id = [{ id: \"{i}\" }, { id: \"bow\" }]\n\n" +
	"let many: [O] keyed by id = [{ id: \"one\", ts: [{ k: {o} }] }, { id: \"two\", ts: [{ k: {t} }] }]\n"

// API.md R7, E11, E22 (DECISIONS 484, 487, 491): an amend segment's `[shield]` on a list keyed by a
// `ref items` is a `layer` ref of the element the line's path reaches, by identity, never another
// instance's, and stays a ref of items[shield]; active or not, each rename rewrites it once or not.
func TestRefsLayerRefKeySegmentElement(t *testing.T) {
	rows := []struct {
		path, to, renamed string
		line              bool // the renamed key is the line's
		refs              string
	}{
		{"p:many[one].ts[shield]", "bow", "{o}", true, "layer {l}:5"},
		{"p:many[two].ts[shield]", "bow", "{t}", false, ""},
		{"p:items[shield]", "y", "{i} {o} {t}", true, "layer {l}:5, value {p}:19, value {p}:19"},
	}
	for _, active := range []bool{true, false} {
		var layers []string
		if active {
			layers = []string{"dev"}
		}
		for _, r := range rows {
			t.Run(fmt.Sprintf("active %v %s", active, r.path), func(t *testing.T) {
				files := func(key string) map[string]string {
					var pairs []string
					for _, m := range []string{"{i}", "{o}", "{t}"} {
						k := "shield"
						if strings.Contains(r.renamed, m) {
							k = key
						}
						pairs = append(pairs, m, k)
					}
					line := "shield"
					if r.line {
						line = key
					}
					src := strings.NewReplacer(pairs...).Replace(segElemSrc)
					layer := "package p\nlayer dev\n\namend many {\n  one.ts[" + line + "].n: 5\n}\n"
					return map[string]string{srcMain: formatted(t, srcMain, src), sharedLayer: formatted(t, sharedLayer, layer)}
				}
				want := strings.NewReplacer("{l}", sharedLayer, "{p}", srcMain).Replace(r.refs)
				sharedRow{l: sharedLevel{path: r.path}, layers: layers, before: files("shield"), after: files(r.to), want: want, to: r.to}.run(t)
			})
		}
	}
}

// perInstKeySrc is aa whose first and second each hold, in oo, an entry keyed {f} and {s}, and in ts
// an element keyed by a ref into their own oo, read by `[k]` and `.k`.
const perInstKeySrc = "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\nrecord T {\n  r: ref S\n  n: Int = 0\n}\n\n" +
	"record A {\n  id: String\n  oo: [S] keyed by k\n  ts: [T] keyed by r\n}\n\n" +
	"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"{f}\" }], ts: [{ r: \"{f}\" }] },\n" +
	"  { id: \"second\", oo: [{ k: \"{s}\" }], ts: [{ r: \"{s}\" }] },\n]\n\n" +
	"let r1: Int = aa.second.ts[{s}].n\n\nlet r2: Int = aa.second.ts.{s}.n\n\nlet r3: Int = aa.first.ts.{f}.n\n"

// API.md R7, E11, E22 (DECISIONS 310, 483, 488): `[k]` and `.k` on a list keyed by a ref into each
// instance's own oo are `code` refs of the entry the reached element's key names, by identity: the
// instance's own entry only; the rename rewrites them once, beside the 310 cascade.
func TestRefsPerInstanceRefKeyedRead(t *testing.T) {
	rows := []struct {
		of, mark string
		refs     []string
	}{
		{"first", "{f}", []string{"value first", "code r3"}},
		{"second", "{s}", []string{"value second", "code r1", "code r2"}},
	}
	for _, r := range rows {
		t.Run(r.of, func(t *testing.T) {
			files := func(key string) map[string]string {
				fill := strings.NewReplacer(r.mark, key, "{f}", "x", "{s}", "x")
				return map[string]string{srcMain: formatted(t, srcMain, fill.Replace(perInstKeySrc))}
			}
			src := files("x")[srcMain]
			var want []string
			for _, ref := range r.refs {
				kind, at, _ := strings.Cut(ref, " ")
				line := lineOf(src, "let "+at+":")
				if kind == "value" {
					line = lineOf(src, "  { id: \""+at+"\"")
				}
				want = append(want, fmt.Sprintf("%s %s:%d", kind, srcMain, line))
			}
			sharedRow{l: sharedLevel{path: "p:aa[" + r.of + "].oo[x]"}, before: files("x"), after: files("y"), want: strings.Join(want, ", ")}.run(t)
		})
	}
}

// defaultKeySrc is items and R, whose field defaults read its own ws, keyed by a ref to an item, by
// `.k` and `[k]`, and rr, which takes them.
const defaultKeySrc = "package p\n\nrecord I {\n  id: String\n}\n\nrecord T {\n  k: ref items\n  n: Int = 0\n}\n\n" +
	"record R {\n  ws: [T] keyed by k\n  u: Int = ws.{s}.n\n  v: Int = ws[{s}].n\n}\n\n" +
	"let items: [I] keyed by id = [{ id: \"{s}\" }, { id: \"bow\" }]\n\nlet rr: R = { ws: [{ k: {s} }] }\n"

// API.md R7, E11, E22 (DECISIONS 488, 491): a record field default's `ws.shield`, a key read on a
// list keyed by a `ref items` the checker records no key for, is a `code` ref of items[shield], as
// `ws[shield]` is; the rename rewrites both.
func TestRefsDefaultRefKeyedRead(t *testing.T) {
	files := func(key string) map[string]string {
		return map[string]string{srcMain: formatted(t, srcMain, strings.ReplaceAll(defaultKeySrc, "{s}", key))}
	}
	src := files("shield")[srcMain]
	want := fmt.Sprintf("code %s:%d, code %s:%d, value %s:%d", srcMain, lineOf(src, "  u:"), srcMain, lineOf(src, "  v:"), srcMain, lineOf(src, "let rr:"))
	sharedRow{l: sharedLevel{path: "p:items[shield]"}, before: files("shield"), after: files("y"), want: want}.run(t)
}

// API.md R7, E11, E22 (DECISIONS 484, 491; one key, one token): a layer line replacing a copy's entry
// whole states the renamed key twice, its segment and its literal's key field: both are `layer`
// refs of second's x, active or not, and the rename rewrites both.
func TestRefsLayerLineReplacedKeyField(t *testing.T) {
	shapes := []struct{ name, bb, pre string }{
		{"spread copy", "let bb: A = { ...aa.second, id: \"bb\" }", "oo"},
		{"collection-valued let", "let bb: [A] keyed by id = aa", "second.oo"},
	}
	want := fmt.Sprintf("layer %s:5, layer %s:5", sharedLayer, sharedLayer)
	for _, sh := range shapes {
		files := func(key string) map[string]string {
			layer := "package p\nlayer dev\n\namend bb {\n  " + sh.pre + "." + key + ": { k: \"" + key + "\", w: 5 }\n}\n"
			return map[string]string{srcMain: formatted(t, srcMain, strings.ReplaceAll(carriedSrc(sh.bb), "{k}", key)), sharedLayer: formatted(t, sharedLayer, layer)}
		}
		for _, active := range []bool{true, false} {
			var layers []string
			if active {
				layers = []string{"dev"}
			}
			t.Run(fmt.Sprintf("%s active %v", sh.name, active), func(t *testing.T) {
				sharedRow{l: sharedLevel{path: "p:aa[second].oo[x]"}, layers: layers, before: files("x"), after: files("y"), want: want}.run(t)
			})
		}
	}
}

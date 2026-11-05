package canon_test

import (
	"fmt"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// mapKeyInstSrc is aa whose first and second each hold, in oo, an entry keyed {f} and {s}, and a
// map m keyed by a ref into their own oo.
const mapKeyInstSrc = "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\n" +
	"record A {\n  id: String\n  oo: [S] keyed by k\n  m: {ref S: Int}\n}\n\n" +
	"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"{f}\" }], m: {} },\n" +
	"  { id: \"second\", oo: [{ k: \"{s}\" }], m: {} },\n]\n"

// API.md R7, E11, E22 (DECISIONS 483, 484, 487): an amend key typed `ref S`, S's collection each
// instance's own oo, names the entry of the instance the line's path reaches: second.m's [x] is a
// `layer` ref of second's x and none of first's, active or not; the rename rewrites only its own.
func TestRefsLayerRefKeyPerInstance(t *testing.T) {
	marks := map[string]string{"first": "{f}", "second": "{s}"}
	for _, active := range []bool{true, false} {
		var layers []string
		if active {
			layers = []string{"dev"}
		}
		for _, lines := range [][]string{{"first"}, {"second"}, {"first", "second"}} {
			for _, of := range []string{"first", "second"} {
				name := fmt.Sprintf("active %v %s rename %s", active, strings.Join(lines, "+"), of)
				t.Run(name, func(t *testing.T) {
					var want []string
					files := func(key string) map[string]string {
						fill := strings.NewReplacer(marks[of], key, "{f}", "x", "{s}", "x")
						layer := "package p\nlayer dev\n\namend aa {\n"
						for i, who := range lines {
							layer += "  " + who + ".m[" + marks[who] + "]: " + fmt.Sprint(i+2) + "\n"
						}
						return map[string]string{srcMain: formatted(t, srcMain, fill.Replace(mapKeyInstSrc)), sharedLayer: formatted(t, sharedLayer, fill.Replace(layer+"}\n"))}
					}
					for i, who := range lines {
						if who == of {
							want = append(want, fmt.Sprintf("layer %s:%d", sharedLayer, i+5))
						}
					}
					sharedRow{l: sharedLevel{path: "p:aa[" + of + "].oo[x]"}, layers: layers, before: files("x"), after: files("y"), want: strings.Join(want, ", ")}.run(t)
				})
			}
		}
	}
}

// refKeyedSrc is items, many, whose entries one and two each hold, in ts, an element keyed by a ref
// to item {o} ({t} in two's), o, whose optional os holds one keyed by {w}, and reads of them.
const refKeyedSrc = "package p\n\nrecord I {\n  id: String\n}\n\nrecord T {\n  k: ref items\n  n: Int = 0\n}\n\n" +
	"record O {\n  id: String\n  ts: [T] keyed by k\n}\n\nrecord W {\n  os: ([T] keyed by k)?\n}\n\n" +
	"let items: [I] keyed by id = [{ id: \"{i}\" }, { id: \"bow\" }]\n\n" +
	"let many: [O] keyed by id = [{ id: \"one\", ts: [{ k: {o} }] }, { id: \"two\", ts: [{ k: {t} }] }]\n\n" +
	"let o: W = { os: [{ k: {w} }] }\n\n" +
	"let r1: Int = many.one.ts[{o1}].n\n\nlet r2: Int = many.one.ts.{o2}.n\n\n" +
	"let r3: Int = o.os![{w1}].n\n\nlet r4: Int = many.two.ts[{t1}].n\n"

// API.md R7, E11, E22 (DECISIONS 316, 473, 477, 488): an expression's `[shield]` on a list keyed by
// a `ref items` is a `code` ref of the element its receiver reaches statically, through an entry
// step or `!`, as `.shield` is; both stay refs of items[shield], each listed once under each target.
func TestRefsExprRefKeyedElement(t *testing.T) {
	rows := []struct {
		path, to, renamed string
		refs              []string
	}{
		{"p:many[one].ts[shield]", "bow", "{o} {o1} {o2}", []string{"code r1", "code r2"}},
		{"p:o.os[shield]", "bow", "{w} {w1}", []string{"code r3"}},
		{"p:items[shield]", "y", "{i} {o} {t} {w} {o1} {o2} {w1} {t1}", []string{"value many", "value many", "value o", "code r1", "code r2", "code r3", "code r4"}},
	}
	for _, r := range rows {
		t.Run(r.path, func(t *testing.T) {
			files := func(key string) map[string]string {
				var pairs []string
				for _, m := range []string{"{i}", "{o}", "{t}", "{w}", "{o1}", "{o2}", "{w1}", "{t1}"} {
					k := "shield"
					if strings.Contains(r.renamed, m) {
						k = key
					}
					pairs = append(pairs, m, k)
				}
				return map[string]string{srcMain: formatted(t, srcMain, strings.NewReplacer(pairs...).Replace(refKeyedSrc))}
			}
			sharedRow{l: sharedLevel{path: r.path}, before: files("shield"), after: files(r.to), want: letRefs(files("shield")[srcMain], r.refs), to: r.to}.run(t)
		})
	}
}

// letRefs is each "kind let" of refs as the kind, p's file and the line declaring that let.
func letRefs(src string, refs []string) string {
	var out []string
	for _, r := range refs {
		kind, let, _ := strings.Cut(r, " ")
		out = append(out, fmt.Sprintf("%s %s:%d", kind, srcMain, lineOf(src, "let "+let+":")))
	}
	return strings.Join(out, ", ")
}

// optLetSrc is xs, an optional keyed list, and ot, an optional table, each holding {x} and z, read
// through `!` and `?.`.
const optLetSrc = "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\nrecord U {\n  w: Int = 0\n}\n\n" +
	"let xs: ([S] keyed by k)? = [{ k: \"{xs}\" }, { k: \"z\" }]\n\n" +
	"let ot: (table U)? = { {ot} { w: 1 }, z { w: 2 } }\n\n" +
	"let r1: Int = xs!.{xs}.w\n\nlet r2: Int? = xs?.{xs}?.w\n\nlet r3: Int = ot!.{ot}.w\n\nlet r4: Int = xs!.z.w\n"

// API.md R7, E11, E22 (DECISIONS 477, 487): `x!` holds the collection x holds, so `xs!.x`,
// `xs?.x` and `ot!.x` on an optional let are `code` refs of its entry, rewritten by the rename.
func TestRefsOptionalLetRoot(t *testing.T) {
	rows := []struct {
		path, mark string
		refs       []string
	}{
		{"p:xs[x]", "{xs}", []string{"code r1", "code r2"}},
		{"p:ot[x]", "{ot}", []string{"code r3"}},
	}
	for _, r := range rows {
		t.Run(r.path, func(t *testing.T) {
			files := func(key string) map[string]string {
				fill := strings.NewReplacer(r.mark, key, "{xs}", "x", "{ot}", "x")
				return map[string]string{srcMain: formatted(t, srcMain, fill.Replace(optLetSrc))}
			}
			sharedRow{l: sharedLevel{path: r.path}, before: files("x"), after: files("y"), want: letRefs(files("x")[srcMain], r.refs)}.run(t)
		})
	}
}

// carriedSrc is aa, whose second's oo holds an entry keyed {k}, and bb, copying second's entries:
// a spread copy of second, or a collection-valued let of aa.
func carriedSrc(bb string) string {
	return "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\nrecord A {\n  id: String\n  oo: [S] keyed by k\n}\n\n" +
		"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\" }] },\n  { id: \"second\", oo: [{ k: \"{k}\" }] },\n]\n\n" + bb + "\n"
}

// API.md R7, E11, E12, E22 (DECISIONS 483, 484, 486, 489, 491): a line amending
// a copy's entry carries the source entry's key, so its `.x` is a `layer` ref of second's x, active
// or not, listed once and rewritten once by its rename; the copy's own x refuses `computed`.
func TestRefsLayerKeyCarriedByCopy(t *testing.T) {
	shapes := []struct{ name, bb, line, own string }{
		{"spread copy", "let bb: A = { ...aa.second, id: \"bb\" }", "oo.{k}.w: 1", "p:bb.oo[x]"},
		{"collection-valued let", "let bb: [A] keyed by id = aa", "second.oo.{k}.w: 1", "p:bb[second].oo[x]"},
	}
	want := fmt.Sprintf("layer %s:5", sharedLayer)
	for _, sh := range shapes {
		files := func(key string) map[string]string {
			fill := strings.NewReplacer("{k}", key)
			layer := "package p\nlayer dev\n\namend bb {\n  " + sh.line + "\n}\n"
			return map[string]string{srcMain: formatted(t, srcMain, fill.Replace(carriedSrc(sh.bb))), sharedLayer: formatted(t, sharedLayer, fill.Replace(layer))}
		}
		for _, active := range []bool{true, false} {
			var layers []string
			if active {
				layers = []string{"dev"}
			}
			t.Run(fmt.Sprintf("%s active %v source", sh.name, active), func(t *testing.T) {
				sharedRow{l: sharedLevel{path: "p:aa[second].oo[x]"}, layers: layers, before: files("x"), after: files("y"), want: want}.run(t)
			})
			t.Run(fmt.Sprintf("%s active %v copy", sh.name, active), func(t *testing.T) {
				sharedRow{l: sharedLevel{path: sh.own}, layers: layers, before: files("x"), want: want, refused: canon.ReasonComputed}.run(t)
			})
		}
	}
}

package canon_test

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// shadowHead is items, a keyed list whose keys are dynamic, the lists and records holding refs into
// it, and {extra}: the declarations a row adds, the name in scope among them.
const shadowHead = "package p\n\n{imports}record I {\n  id: String\n}\n\nrecord T {\n  k: ref items\n  n: Int = 0\n}\n\n" +
	"record O {\n  id: String\n  ts: [T] keyed by k\n}\n\nrecord P {\n  pick: ref items\n}\n\n" +
	"let items: [I] keyed by id = [{ id: \"shield\" }, { id: \"axe\" }, { id: \"{i}\" }, { id: \"mace\" }]\n\n" +
	"let many: [O] keyed by id = [{ id: \"one\", ts: [{ k: {s}, n: 1 }, { k: axe, n: 7 }, { k: {b}, n: 2 }] }]\n\n" +
	"let pp: P = { pick: {p} }\n\nlet pps: table P = {\n  a { pick: {a} }\n{e}}\n{extra}"

// shadowRow is one edit of shadowHead's project: the marks before and after, the declarations it
// adds, its layers, and the values it reads back after the edit.
type shadowRow struct {
	name          string
	head          string            // the source when not shadowHead
	more          map[string]string // the other files of the project
	extra         string
	before, after map[string]string // mark → text
	undone        map[string]string // the marks the Undo leaves, when not before's: a string stays a string (E11, E22)
	layer, edit   string            // a layer file's body, and the edit layer
	ops           []canon.Op
	want, kept    map[string]string // the values after the edit, and after its Undo
}

// shadowMarks fill shadowHead's marks absent from a row's map.
var shadowMarks = map[string]string{"{s}": "\"shield\"", "{b}": "bow", "{p}": "bow", "{e}": "", "{i}": "bow", "{a}": "bow", "{f}": "bow", "{g}": "bow", "{l}": "bow", "{imports}": "", "{mk}": "bow", "{sk}": "bow", "{st}": "bow", "{mv}": "bow"}

// files are shadowHead with marks, and the row's layer file.
func (r shadowRow) files(t *testing.T, marks map[string]string) map[string]string {
	t.Helper()
	fill := func(text string) string {
		for _, m := range slices.Sorted(maps.Keys(shadowMarks)) {
			v, ok := marks[m]
			if !ok {
				v = shadowMarks[m]
			}
			text = strings.ReplaceAll(text, m, v)
		}
		return text
	}
	out := map[string]string{srcMain: formatted(t, srcMain, fill(strings.ReplaceAll(cmp.Or(r.head, shadowHead), "{extra}", r.extra)))}
	if r.layer != "" {
		out[sharedLayer] = fill(r.layer)
	}
	return out
}

// run applies r's ops, checks the files and values it leaves, then that its Undo gives back the bytes.
func (r shadowRow) run(t *testing.T) {
	law := maps.Clone(editLaw)
	before := r.files(t, r.before)
	maps.Copy(law, before)
	maps.Copy(law, r.more)
	opts := project(law)
	if r.edit != "" {
		opts.Layers, opts.EditLayer = []string{r.edit}, r.edit
	}
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	m, _ := opts.FS.(*memFS)
	ctx := context.Background()
	res, err := p.Edit(ctx, canon.Edit{Ops: r.ops})
	if err != nil {
		t.Fatalf("API.md E19: the edit is refused: %v", err)
	}
	after := r.files(t, r.after)
	if r.edit != "" {
		after[sharedLayer] = r.after["layer"]
	}
	for _, name := range slices.Sorted(maps.Keys(after)) {
		if got := read(t, m, name); got != after[name] {
			t.Errorf("DECISIONS 318, 378: %s after the edit:\n%s\nwant:\n%s", name, got, after[name])
		}
	}
	checkValues(t, p, "TYPES.md §4.1 (DECISIONS 318)", r.want)
	if back, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		var findings []canon.Finding
		if back != nil {
			findings = back.Findings
		}
		t.Fatalf("API.md E22: the Undo: %v %v", err, findings)
	}
	undone := before
	if r.undone != nil {
		undone = r.files(t, r.undone)
	}
	for _, name := range slices.Sorted(maps.Keys(before)) {
		if got := read(t, m, name); got != undone[name] {
			t.Errorf("API.md E22: %s after the Undo:\n%s\nwant:\n%s", name, got, undone[name])
		}
	}
	checkValues(t, p, "API.md E22", r.kept)
}

// API.md M7, E11, E22 (TYPES.md 4.1; DECISIONS 318, 366, 378): a Set, an AddEntry, a layer
// write, a rename and an Undo print a dynamic-keyed ref's word key as a string where a let, parameter
// or field of that word is in scope at the position written, else bare.
func TestEditQuotesShadowedRefKey(t *testing.T) {
	letShield, letLance := "\nlet shield: ref items = axe\n", "\nlet lance: ref items = axe\n"
	mapM := "\nrecord M {\n  m: {ref items: Int}\n}\n\nlet mm: M = { m: { {mk}: 1 } }\n"
	staticSt := "\nrecord S2 {\n  n: Int = 0\n}\n\nlet st: table S2 = {\n  {sk} { n: 2 }\n  axe { n: 7 }\n}\n\nlet sr: Int = st[{st}].n\n\nlet sh: Bool = st.hasKey({st})\n"
	rows := []shadowRow{
		{name: "set shadowed by a let", extra: letShield, after: map[string]string{"{p}": "\"shield\""},
			ops: []canon.Op{canon.Set("p:pp.pick", canon.Key("shield"))}, want: map[string]string{"p:pp.pick": "shield"}},
		{name: "set unshadowed", after: map[string]string{"{p}": "shield"},
			ops: []canon.Op{canon.Set("p:pp.pick", canon.Key("shield"))}, want: map[string]string{"p:pp.pick": "shield"}},
		{name: "add entry shadowed by a let", extra: letShield, after: map[string]string{"{e}": "  b { pick: \"shield\" }\n"},
			ops: []canon.Op{canon.AddEntry("p:pps", canon.Key("b"), canon.Obj{"pick": canon.Key("shield")})}, want: map[string]string{"p:pps.b.pick": "shield"}},
		{name: "layer write shadowed by a let", extra: letShield, edit: "dev", layer: "package p\nlayer dev\n",
			after: map[string]string{"layer": "package p\nlayer dev\n\namend pp { pick: \"shield\" }\n"},
			ops:   []canon.Op{canon.Set("p:pp.pick", canon.Key("shield"))}, want: map[string]string{"p:pp.pick": "shield"}},
		{name: "rename's tokens shadowed by a let", extra: "\nlet lance: ref items = axe\n",
			after:  map[string]string{"{i}": "lance", "{a}": "\"lance\"", "{b}": "\"lance\"", "{p}": "\"lance\""},
			undone: map[string]string{"{a}": "\"bow\"", "{b}": "\"bow\"", "{p}": "\"bow\""},
			ops:    []canon.Op{canon.Rename("p:items[bow]", canon.Key("lance"))}, want: map[string]string{"p:pp.pick": "lance"}, kept: map[string]string{"p:pp.pick": "bow"}},
		{name: "rename's tokens shadowed by a parameter", extra: "\nfn f(lance: ref items) -> Int {\n  return many.one.ts[{f}].n\n}\n\nlet rf: Int = f(axe)\n",
			after:  map[string]string{"{i}": "lance", "{a}": "lance", "{b}": "lance", "{p}": "lance", "{f}": "\"lance\""},
			undone: map[string]string{"{f}": "\"bow\""},
			ops:    []canon.Op{canon.Rename("p:items[bow]", canon.Key("lance"))}, want: map[string]string{"p:rf": "2"}, kept: map[string]string{"p:rf": "2"}},
		{name: "rename's tokens shadowed by a field", extra: "\nrecord R {\n  lance: ref items = axe\n  ws: [T] keyed by k\n  u: Int = ws[{f}].n\n}\n\nlet rr: R = { ws: [{ k: {g}, n: 2 }, { k: axe, n: 7 }] }\n",
			after:  map[string]string{"{i}": "lance", "{a}": "lance", "{b}": "lance", "{p}": "lance", "{f}": "\"lance\"", "{g}": "lance"},
			undone: map[string]string{"{f}": "\"bow\""},
			ops:    []canon.Op{canon.Rename("p:items[bow]", canon.Key("lance"))}, want: map[string]string{"p:rr.u": "2"}, kept: map[string]string{"p:rr.u": "2"}},
		{name: "rename unshadowed", after: map[string]string{"{i}": "lance", "{a}": "lance", "{b}": "lance", "{p}": "lance"},
			ops: []canon.Op{canon.Rename("p:items[bow]", canon.Key("lance"))}, want: map[string]string{"p:pp.pick": "lance"}},
		{name: "rename's amend segment shadowed by a let", extra: "\nlet lance: ref items = axe\n", layer: "package p\nlayer dev\n\namend many {\n  one.ts[{l}].n: 5\n}\n",
			after:  map[string]string{"{i}": "lance", "{a}": "\"lance\"", "{b}": "\"lance\"", "{p}": "\"lance\"", "{l}": "\"lance\""},
			undone: map[string]string{"{a}": "\"bow\"", "{b}": "\"bow\"", "{p}": "\"bow\"", "{l}": "\"bow\""},
			ops:    []canon.Op{canon.Rename("p:items[bow]", canon.Key("lance"))}, want: map[string]string{"p:pp.pick": "lance"}},
		{name: "layer write beside a line keyed by a let", extra: letShield, edit: "dev", layer: "package p\nlayer dev\n\namend many {\n  one.ts[shield].n: 5\n}\n",
			after: map[string]string{"layer": "package p\nlayer dev\n\namend many {\n  one.ts[shield].n: 5\n  one.ts[\"shield\"].n: 9\n}\n"},
			ops:   []canon.Op{canon.Set("p:many[one].ts[shield].n", canon.Int(9))}, want: map[string]string{"p:many[one].ts[shield].n": "9", "p:many[one].ts[axe].n": "5"}},
		{name: "rename's map key shadowed by a let", extra: letLance + mapM,
			after:  map[string]string{"{i}": "lance", "{a}": "\"lance\"", "{b}": "\"lance\"", "{p}": "\"lance\"", "{mk}": "\"lance\""},
			undone: map[string]string{"{a}": "\"bow\"", "{b}": "\"bow\"", "{p}": "\"bow\"", "{mk}": "\"bow\""},
			ops:    []canon.Op{canon.Rename("p:items[bow]", canon.Key("lance"))}, want: map[string]string{"p:mm.m[lance]": "1"}, kept: map[string]string{"p:mm.m[bow]": "1"}},
		{name: "a map entry's rename shadowed by a let", extra: "\nlet mace: ref items = axe\n" + mapM,
			after: map[string]string{"{mk}": "\"mace\""}, undone: map[string]string{"{mk}": "\"bow\""},
			ops: []canon.Op{canon.Rename("p:mm.m[bow]", canon.Key("mace"))}, want: map[string]string{"p:mm.m[mace]": "1"}, kept: map[string]string{"p:mm.m[bow]": "1"}},
		{name: "rename's static table keys shadowed by a let", extra: letLance + staticSt,
			after:  map[string]string{"{sk}": "lance", "{st}": "\"lance\""},
			undone: map[string]string{"{st}": "\"bow\""},
			ops:    []canon.Op{canon.Rename("p:st.bow", canon.Key("lance"))}, want: map[string]string{"p:sr": "2", "p:sh": "true"}, kept: map[string]string{"p:sr": "2", "p:sh": "true"}},
		{name: "rename shadowed by a type", extra: "\nrecord Lance {\n  x: Int = 0\n}\n",
			after:  map[string]string{"{i}": "Lance", "{a}": "\"Lance\"", "{b}": "\"Lance\"", "{p}": "\"Lance\""},
			undone: map[string]string{"{a}": "\"bow\"", "{b}": "\"bow\"", "{p}": "\"bow\""},
			ops:    []canon.Op{canon.Rename("p:items[bow]", canon.Key("Lance"))}, want: map[string]string{"p:pp.pick": "Lance"}},
		{name: "rename shadowed by an imported name", more: map[string]string{"q/q.canon": "package q\n\nlet lance: Int = 1\n"},
			before: map[string]string{"{imports}": "import q { lance }\n\n"},
			after:  map[string]string{"{imports}": "import q { lance }\n\n", "{i}": "lance", "{a}": "\"lance\"", "{b}": "\"lance\"", "{p}": "\"lance\""},
			undone: map[string]string{"{imports}": "import q { lance }\n\n", "{a}": "\"bow\"", "{b}": "\"bow\"", "{p}": "\"bow\""},
			ops:    []canon.Op{canon.Rename("p:items[bow]", canon.Key("lance"))}, want: map[string]string{"p:pp.pick": "lance"}},
		{name: "set of an enum-leaf ref shadowed by a let", head: enumHead, before: map[string]string{"{p}": "a"}, after: map[string]string{"{p}": "\"b\""},
			ops: []canon.Op{canon.Set("p:pe.pick", canon.Member("b"))}, want: map[string]string{"p:pe.pick": "b"}},
		{name: "rename's map key and its ref value shadowed by a let", extra: letLance + "\nrecord M2 {\n  r: {ref items: ref items}\n}\n\nlet mr: M2 = { r: { {mk}: {mv} } }\n",
			after:  map[string]string{"{i}": "lance", "{a}": "\"lance\"", "{b}": "\"lance\"", "{p}": "\"lance\"", "{mk}": "\"lance\"", "{mv}": "\"lance\""},
			undone: map[string]string{"{a}": "\"bow\"", "{b}": "\"bow\"", "{p}": "\"bow\"", "{mk}": "\"bow\"", "{mv}": "\"bow\""},
			ops:    []canon.Op{canon.Rename("p:items[bow]", canon.Key("lance"))}, want: map[string]string{"p:mr.r[lance]": "lance"}},
		{name: "rename's map key and a ref in its list shadowed by a let", extra: letLance + "\nrecord M3 {\n  r: {ref items: [ref items]}\n}\n\nlet ml: M3 = { r: { {mk}: [{mv}, axe] } }\n",
			after:  map[string]string{"{i}": "lance", "{a}": "\"lance\"", "{b}": "\"lance\"", "{p}": "\"lance\"", "{mk}": "\"lance\"", "{mv}": "\"lance\""},
			undone: map[string]string{"{a}": "\"bow\"", "{b}": "\"bow\"", "{p}": "\"bow\"", "{mk}": "\"bow\"", "{mv}": "\"bow\""},
			ops:    []canon.Op{canon.Rename("p:items[bow]", canon.Key("lance"))}, want: map[string]string{"p:ml.r[lance][0]": "lance"}},
		{name: "rename's map key before a comment shadowed by a let", extra: letLance + "\nrecord M {\n  m: {ref items: Int}\n}\n\nlet mm: M = { m: { {mk}: /* mid */ 1 } }\n",
			after:  map[string]string{"{i}": "lance", "{a}": "\"lance\"", "{b}": "\"lance\"", "{p}": "\"lance\"", "{mk}": "\"lance\"", "{mv}": "\"lance\""},
			undone: map[string]string{"{a}": "\"bow\"", "{b}": "\"bow\"", "{p}": "\"bow\"", "{mk}": "\"bow\"", "{mv}": "\"bow\""},
			ops:    []canon.Op{canon.Rename("p:items[bow]", canon.Key("lance"))}, want: map[string]string{"p:mm.m[lance]": "1"}},
		{name: "a rename's Undo shadowed by a let", extra: letShield, after: map[string]string{"{s}": "mace"},
			ops: []canon.Op{canon.Rename("p:many[one].ts[shield]", canon.Key("mace"))}, want: map[string]string{"p:many[one].ts[mace].n": "1"}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) { r.run(t) })
	}
}

// enumHead is ens, keyed by a member of Q, b a let naming one of its elements, and pe, whose pick is
// a ref into ens written {p}.
const enumHead = "package p\n\nenum Q {\n  a\n  b\n  c\n}\n\nrecord E {\n  q: Q\n}\n\nrecord PE {\n  pick: ref ens\n}\n\n" +
	"let ens: [E] keyed by q = [{ q: a }, { q: b }, { q: c }]\n\nlet b: ref ens = c\n\nlet pe: PE = { pick: {p} }\n{extra}"

// enumLeafSrc is ens, keyed by a member of Q, and hs, whose one holds rs keyed by a ref to ens, its
// element keyed {k}; hc is a spread copy of one.
const enumLeafSrc = "package p\n\nenum Q {\n  a\n  b\n  c\n}\n\nrecord E {\n  q: Q\n}\n\n" +
	"record R {\n  k: ref ens\n  n: Int = 0\n}\n\nrecord H {\n  id: String\n  rs: [R] keyed by k\n}\n\n" +
	"let ens: [E] keyed by q = [{ q: a }, { q: b }, { q: c }]\n\n" +
	"let hs: [H] keyed by id = [{ id: \"one\", rs: [{ k: {k}, n: 1 }] }]\n\nlet hc: H = { ...hs.one, id: \"hc\" }\n"

// API.md R7, E11, E22 (DECISIONS 490, 491, 494): a line replacing a copy's element whole, its key a
// ref whose chain ends at an enum, states the key twice; the literal's key field written bare is the
// member the checker resolves it to, the same key, so it is listed and the rename rewrites both.
func TestRefsLayerLineReplacedEnumKeyField(t *testing.T) {
	want := sharedLayer + ":5"
	for _, active := range []bool{true, false} {
		var layers []string
		if active {
			layers = []string{"dev"}
		}
		t.Run(fmt.Sprintf("active %v", active), func(t *testing.T) {
			files := func(key string) map[string]string {
				layer := "package p\nlayer dev\n\namend hc {\n  rs[" + key + "]: { k: " + key + ", n: 5 }\n}\n"
				return map[string]string{srcMain: formatted(t, srcMain, strings.ReplaceAll(enumLeafSrc, "{k}", key)), sharedLayer: formatted(t, sharedLayer, layer)}
			}
			sharedRow{l: sharedLevel{path: "p:hs[one].rs[b]"}, layers: layers, before: files("b"), after: files("c"),
				want: "layer " + want + ", layer " + want, key: canon.Member("c")}.run(t)
		})
	}
}

// literalReadSrc is aa, whose first and second each hold an entry keyed x in their own oo, al a let of
// second's oo, and reads of second's x by a string key, through the field and through al.
const literalReadSrc = "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\nrecord A {\n  id: String\n  oo: [S] keyed by k\n}\n\n" +
	"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\" }] },\n  { id: \"second\", oo: [{ k: \"{k}\" }] },\n]\n\n" +
	"let al: [S] keyed by k = aa.second.oo\n\nlet r1: Int = aa.second.oo[\"{k}\"].w\n\nlet r2: Int = al[\"{k}\"].w\n\nlet r3: Int = aa.first.oo[\"x\"].w\n"

// API.md R7, E11, E22 (DECISIONS 477, 483): a string key reading an element of a field's keyed list
// from a receiver reaching it statically, through the field or a let of it, is a `code` ref of that
// instance's element only; the rename rewrites it.
func TestRefsLiteralKeyOnFieldList(t *testing.T) {
	files := func(key string) map[string]string {
		return map[string]string{srcMain: formatted(t, srcMain, strings.ReplaceAll(literalReadSrc, "{k}", key))}
	}
	src := files("x")[srcMain]
	want := fmt.Sprintf("code %s:%d, code %s:%d", srcMain, lineOf(src, "let r1:"), srcMain, lineOf(src, "let r2:"))
	sharedRow{l: sharedLevel{path: "p:aa[second].oo[x]"}, before: files("x"), after: files("y"), want: want}.run(t)
}

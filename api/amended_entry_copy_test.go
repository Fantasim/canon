package canon_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// amendedEntrySrc is p: e holds h's entry x outside h's uu; r's defaults alt, al and am hold, in a
// field, a plain list and a map, its own qs' entry q. pe, pr and pm compare each copy with its
// source, ws reads every w.
const amendedEntrySrc = "package p\n\nrecord T {\n  n: String\n  w: Int = 0\n}\n\n" +
	"record U {\n  n: String\n  w: Int = 0\n  tt: [T] keyed by n = [{ n: \"t\" }]\n}\n\n" +
	"record H {\n  id: String\n  uu: [U] keyed by n\n}\n\n" +
	"record R {\n  qs: [U] keyed by n = [{ n: \"q\" }]\n  alt: U = qs.q\n  al: [U] = [qs.q]\n  am: {String: U} = { \"a\": qs.q }\n}\n\n" +
	"let h: H = { id: \"h\", uu: [{ n: \"x\" }] }\n\nlet e: U = h.uu.x\n\nlet r: R = {}\n\n" +
	"let pe: [Bool] = [e == h.uu.x, h.uu.contains(e), e.tt.t == h.uu.x.tt.t]\n\n" +
	"let pr: [Bool] = [r.alt == r.qs.q, r.qs.contains(r.alt), r.alt.tt.t == r.qs.q.tt.t, r.al[0] == r.qs.q, r.qs.contains(r.al[0])]\n\n" +
	"let pm: [Bool] = [r.am[\"a\"] == r.qs.q, r.qs.contains(r.am[\"a\"])]\n\n" +
	"let ws: [Int] = [e.w, e.tt.t.w, h.uu.x.w, h.uu.x.tt.t.w, r.alt.w, r.alt.tt.t.w, r.qs.q.w, r.qs.q.tt.t.w, r.al[0].w]\n"

// The probes with every copy its source's, and ws with every w 0.
const (
	amendedEntryPE = "[true, true, true]"
	amendedEntryPR = "[true, true, true, true, true]"
	amendedEntryWS = "[0, 0, 0, 0, 0, 0, 0, 0, 0]"
)

// amendedEntryLayers are the layer files of the matrix, by name, each one line.
var amendedEntryLayers = map[string]string{
	"ew": "amend e {\n  w: 5\n}\n", "et": "amend e {\n  tt.t.w: 6\n}\n",
	"rw": "amend r {\n  alt.w: 5\n}\n", "rt": "amend r {\n  alt.tt.t.w: 6\n}\n",
	"rl": "amend r {\n  al[0].w: 7\n}\n", "rlw": "amend r {\n  al[0]: { n: \"q\", w: 3 }\n}\n",
	"hw": "amend h {\n  uu.x.w: 5\n}\n", "rm": "amend r {\n  am[\"a\"].w: 4\n}\n",
}

// amendedEntryCase is a set of layers, the active ones in order, and what pe, pr and ws then
// give; own are element nodes the amendments make a copy's own, base the nodes that stay.
type amendedEntryCase struct {
	name       string
	active     []string
	pe, pr, ws string
	own        []string
}

// amendedEntryBase are the element nodes every build has, at each source's place (DECISIONS 478).
var amendedEntryBase = []string{"p:h.uu[x]", "p:h.uu[x].tt[t]", "p:r.qs[q]", "p:r.qs[q].tt[t]"}

// amendedEntryCases is the matrix: each layer alone, stacked, and none active.
var amendedEntryCases = []amendedEntryCase{
	{"inactive", nil, amendedEntryPE, amendedEntryPR, amendedEntryWS, nil},
	{"let copy amended", []string{"ew"}, "[false, false, true]", amendedEntryPR, "[5, 0, 0, 0, 0, 0, 0, 0, 0]", nil},
	{"let copy amended below", []string{"et"}, "[false, false, false]", amendedEntryPR, "[0, 6, 0, 0, 0, 0, 0, 0, 0]", []string{"p:e.tt[t]"}},
	{"field copy amended", []string{"rw"}, amendedEntryPE, "[false, false, true, true, true]", "[0, 0, 0, 0, 5, 0, 0, 0, 0]", nil},
	{"field copy amended below", []string{"rt"}, amendedEntryPE, "[false, false, false, true, true]", "[0, 0, 0, 0, 0, 6, 0, 0, 0]", []string{"p:r.alt.tt[t]"}},
	{"list copy amended", []string{"rl"}, amendedEntryPE, "[true, true, true, false, false]", "[0, 0, 0, 0, 0, 0, 0, 0, 7]", nil},
	{"list copy replaced", []string{"rlw"}, amendedEntryPE, "[true, true, true, false, false]", "[0, 0, 0, 0, 0, 0, 0, 0, 3]", []string{"p:r.al[0].tt[t]"}},
	{"source amended", []string{"hw"}, amendedEntryPE, amendedEntryPR, "[5, 0, 5, 0, 0, 0, 0, 0, 0]", nil},
	{"let stacked", []string{"ew", "et"}, "[false, false, false]", amendedEntryPR, "[5, 6, 0, 0, 0, 0, 0, 0, 0]", []string{"p:e.tt[t]"}},
	{"let stacked reversed", []string{"et", "ew"}, "[false, false, false]", amendedEntryPR, "[5, 6, 0, 0, 0, 0, 0, 0, 0]", []string{"p:e.tt[t]"}},
	{"field stacked", []string{"rw", "rt"}, amendedEntryPE, "[false, false, false, true, true]", "[0, 0, 0, 0, 5, 6, 0, 0, 0]", []string{"p:r.alt.tt[t]"}},
	{"source then copy", []string{"hw", "ew"}, amendedEntryPE, amendedEntryPR, "[5, 0, 5, 0, 0, 0, 0, 0, 0]", nil},
}

// openAmendedEntry opens amendedEntrySrc with every layer of amendedEntryLayers, those of active on, and more beside.
func openAmendedEntry(t *testing.T, active []string, more map[string]string) *canon.Project {
	t.Helper()
	law := maps.Clone(editLaw)
	maps.Copy(law, more)
	law[srcMain] = amendedEntrySrc
	//canon:unordered each layer is stored under its own file
	for name, body := range amendedEntryLayers {
		law["p/"+name+".layer.canon"] = "package p\nlayer " + name + "\n\n" + body
	}
	opts := project(law)
	opts.Layers = active
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// checkAmendedEntry fails unless p, built with c's layers, gives c's probes and c's element nodes,
// those at each source's place included.
func checkAmendedEntry(t *testing.T, p *canon.Project, c amendedEntryCase) {
	t.Helper()
	checkErrors(t, p)
	checkValues(t, p, "EVALUATION.md §4.2; DECISIONS 510", map[string]string{"p:pe": c.pe, "p:pr": c.pr, "p:ws": c.ws})
	nodes := elementNodes(t, p)
	for _, n := range append(slices.Clone(amendedEntryBase), c.own...) {
		if !slices.Contains(nodes, n) {
			t.Errorf("EVALUATION.md §4.2; DECISIONS 478, 486: element nodes %v lack %s", nodes, n)
		}
	}
	for _, n := range nodes {
		if !slices.Contains(amendedEntryBase, n) && !slices.Contains(c.own, n) {
			t.Errorf("EVALUATION.md §4.2; DECISIONS 478, 486: element node %s, want only %v and %v", n, amendedEntryBase, c.own)
		}
	}
}

// EVALUATION.md 4.2; DECISIONS 486, 510 (ruling of 2026-11-06): an amended copy of an entry held outside
// its collection is a record of its own, unequal to the entry, and owns what a line amends below it,
// at its place in Graph; with no layer active, or only its source amended, it is the entry.
func TestAmendedEntryCopyIsItsOwn(t *testing.T) {
	for _, c := range amendedEntryCases {
		t.Run(c.name, func(t *testing.T) {
			checkAmendedEntry(t, openAmendedEntry(t, c.active, nil), c)
		})
	}
}

// valueOrigin is the origin of the value at path in p, and those it came through (originText).
func valueOrigin(t *testing.T, p *canon.Project, path string) string {
	t.Helper()
	v, err := p.Value(context.Background(), path)
	if err != nil {
		t.Fatalf("API.md R1: Value of %s: %v", path, err)
	}
	return originText(&v.Origin)
}

// EVALUATION.md 13; DECISIONS 486, 510 (ruling of 2026-11-06): in an amended copy of an entry, the
// amended field is the layer's, every other field keeps the entry's provenance.
func TestAmendedEntryCopyProvenance(t *testing.T) {
	p := openAmendedEntry(t, []string{"ew", "rw"}, nil)
	for _, c := range []struct{ copied, source string }{{"p:e.n", "p:h.uu[x].n"}, {"p:r.alt.n", "p:r.qs[q].n"}} {
		if got, want := valueOrigin(t, p, c.copied), valueOrigin(t, p, c.source); got != want {
			t.Errorf("EVALUATION.md §13: origin of %s = %s, want %s's %s", c.copied, got, c.source, want)
		}
	}
	for _, c := range []struct{ path, layer string }{{"p:e.w", "ew"}, {"p:r.alt.w", "rw"}} {
		if got := valueOrigin(t, p, c.path); !strings.HasPrefix(got, string(canon.OriginLayer)+" ") || !strings.Contains(got, fmt.Sprintf("layer %q", c.layer)) {
			t.Errorf("EVALUATION.md §13: origin of %s = %s, want layer %s", c.path, got, c.layer)
		}
	}
}

// API.md W7; EVALUATION.md 4.2; DECISIONS 486, 501, 510 (ruling of 2026-11-06): a build after an edit
// of another file, which replays p's file, gives every case of the matrix a cold build gives.
func TestAmendedEntryCopyWarm(t *testing.T) {
	for _, c := range amendedEntryCases {
		t.Run(c.name, func(t *testing.T) {
			p := openAmendedEntry(t, c.active, map[string]string{"p/n.canon": "package p\n\nlet n: Int = 1\n"})
			for step := 2; step <= 3; step++ {
				if _, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("p:n", canon.Int(int64(step)))}}); err != nil {
					t.Fatalf("API.md E19: %v", err)
				}
				checkAmendedEntry(t, p, c)
			}
		})
	}
}

// probeText is valueText of path, "" for none.
func probeText(t *testing.T, p *canon.Project, path string) string {
	t.Helper()
	if path == "" {
		return ""
	}
	return valueText(t, p, path)
}

// checkCopyAfter is checkDefaultCopy of c after a line of r at path, active when on. A line at or
// below c's copy, an entry held outside its collection, makes it a record of its own: its probe's
// last comparison, the copy == its entry, is false (EVALUATION.md 4.2; DECISIONS 510).
func checkCopyAfter(t *testing.T, p *canon.Project, c defaultCopyShape, path string, on bool) {
	t.Helper()
	if !on || !c.entry || !strings.HasPrefix("p:r."+path, c.copy+".") {
		checkDefaultCopy(t, p, c)
		return
	}
	checkValues(t, p, "EVALUATION.md §4.2; DECISIONS 510", map[string]string{c.probe: "[true, true, true, true, false]"})
	c.probe = ""
	checkDefaultCopy(t, p, c)
}

// EVALUATION.md 4.2; DECISIONS 486, 510 (ruling of 2026-11-06): a map's value amended below, a copy of
// an entry held outside its collection, is a record of its own; inactive, it is the entry.
func TestAmendedEntryCopyInMap(t *testing.T) {
	for _, c := range []struct {
		active []string
		want   string
	}{{nil, "[true, true]"}, {[]string{"rm"}, "[false, false]"}} {
		p := openAmendedEntry(t, c.active, nil)
		checkErrors(t, p)
		checkValues(t, p, "EVALUATION.md §4.2; DECISIONS 510", map[string]string{"p:pm": c.want})
	}
}

// assignedSlotSrc is p: f puts a new record, pl h's entry y, in the slot of a plain list that held
// h's entry x; pv compares each with h's entries.
const assignedSlotSrc = "package p\n\nrecord U {\n  n: String\n  w: Int = 0\n}\n\nrecord H {\n  uu: [U] keyed by n\n}\n\n" +
	"fn f(u: U) -> [U] {\n  var xs: [U] = [u]\n  xs[0] = { n: \"x\", w: 3 }\n  return xs\n}\n\n" +
	"fn pl(hh: H) -> [U] {\n  var xs: [U] = [hh.uu.x]\n  xs[0] = hh.uu.y\n  return xs\n}\n\n" +
	"let h: H = { uu: [{ n: \"x\" }, { n: \"y\" }] }\n\nlet pv: [Bool] = [f(h.uu.x)[0] == h.uu.x, h.uu.contains(f(h.uu.x)[0]), pl(h)[0] == h.uu.y]\n"

// EVALUATION.md 4.1, 4.2; DECISIONS 510 (ruling of 2026-11-06): a value assigned to a plain list's
// element keeps its own identity, a record none and an entry its own, never the replaced one's.
func TestAssignedPlainSlotKeepsOwnIdentity(t *testing.T) {
	p := openLayered(t, assignedSlotSrc, "", false)
	checkErrors(t, p)
	checkValues(t, p, "EVALUATION.md §4.2; DECISIONS 510", map[string]string{"p:pv": "[false, false, true]"})
}

// keyedSlotSrc is p: g assigns a literal to its keyed var's slot x twice, k assigns i's entry x to
// a copy of h's uu; pk compares each slot with the entries and reads the second literal by key x.
const keyedSlotSrc = "package p\n\nrecord U {\n  n: String\n  w: Int = 0\n}\n\nrecord H {\n  uu: [U] keyed by n\n}\n\n" +
	"fn g(hh: H) -> [U] keyed by n {\n  var xs: [U] keyed by n = hh.uu\n  xs[\"x\"] = { n: \"x\", w: 3 }\n  xs[\"x\"] = { n: \"x\" }\n  return xs\n}\n\n" +
	"fn k(hh: H, ii: H) -> [U] keyed by n {\n  var xs: [U] keyed by n = hh.uu\n  xs[\"x\"] = ii.uu.x\n  return xs\n}\n\n" +
	"let h: H = { uu: [{ n: \"x\" }, { n: \"y\" }] }\n\nlet i: H = { uu: [{ n: \"x\" }] }\n\n" +
	"let pk: [Bool] = [g(h).x == h.uu.x, h.uu.contains(g(h).x), g(h).x.w == 0, g(h).y == h.uu.y, k(h, i).x == i.uu.x, k(h, i).x == h.uu.x, k(h, i).y == h.uu.y]\n"

// EVALUATION.md 4.1, 4.2; TYPES.md 6.3; DECISIONS 510 (ruling of 2026-11-06): a record assigned to a keyed var's
// slot takes no identity from the element it replaces: a literal gets one in a collection of its
// own, by its key, unequal even to an equal entry; an entry keeps its own; the rest keep theirs.
func TestAssignedKeyedSlotIdentity(t *testing.T) {
	p := openLayered(t, keyedSlotSrc, "", false)
	checkErrors(t, p)
	checkValues(t, p, "EVALUATION.md §4.2; TYPES.md §6.3", map[string]string{"p:pk": "[false, false, true, true, true, false, true]"})
}

// optionalCopySrc is p: r's default ao holds its own qs' entry q through an optional field.
const optionalCopySrc = "package p\n\nrecord U {\n  n: String\n  w: Int = 0\n}\n\n" +
	"record R {\n  qs: [U] keyed by n = [{ n: \"q\" }]\n  ao: U? = qs.q\n}\n\nlet r: R = {}\n\n" +
	"let po: [Bool] = [r.ao! == r.qs.q, r.qs.contains(r.ao!)]\n"

// foreignCopySrc is p: e holds the entry x of package q's h, outside h's uu.
const foreignCopySrc = "package p\n\nimport q\n\nlet e: q.U = q.h.uu.x\n\nlet pf: [Bool] = [e == q.h.uu.x, q.h.uu.contains(e)]\n"

// foreignCopyQ is package q: h's uu holds the entry x.
const foreignCopyQ = "package q\n\nrecord U {\n  n: String\n  w: Int = 0\n}\n\nrecord H {\n  uu: [U] keyed by n\n}\n\n" +
	"let h: H = { uu: [{ n: \"x\" }] }\n"

// EVALUATION.md 4.2; DECISIONS 486, 510 (ruling of 2026-11-06): an amended copy of an entry held in an
// optional field, or of another package's entry, is a record of its own; inactive, it is the entry.
func TestAmendedEntryCopyOptionalAndForeign(t *testing.T) {
	want := map[bool]string{false: "[true, true]", true: "[false, false]"}
	for _, c := range []struct {
		src, body, probe string
		more             map[string]string
	}{
		{optionalCopySrc, "amend r {\n  ao.w: 5\n}\n", "p:po", nil},
		{foreignCopySrc, "amend e {\n  w: 5\n}\n", "p:pf", map[string]string{"q/q.canon": foreignCopyQ}},
	} {
		for _, on := range []bool{false, true} {
			p := openLayeredWith(t, c.src, c.more, c.body, on)
			checkErrors(t, p)
			checkValues(t, p, "EVALUATION.md §4.2; DECISIONS 510", map[string]string{c.probe: want[on]})
		}
	}
}

// heldListSrc is p: mm holds h's uu as a map value, a list element, an optional field and a field,
// ks and ol hold it at a let and an optional let; pc compares the x each holds with h's, wc reads
// each x's w.
const heldListSrc = "package p\n\nrecord U {\n  n: String\n  w: Int = 0\n}\n\nrecord H {\n  uu: [U] keyed by n\n}\n\n" +
	"record M {\n  mk: {String: [U] keyed by n}\n  lk: [[U] keyed by n]\n  ok: ([U] keyed by n)?\n  fk: [U] keyed by n\n}\n\n" +
	"let h: H = { uu: [{ n: \"x\" }] }\n\nlet mm: M = { mk: { \"a\": h.uu }, lk: [h.uu], ok: h.uu, fk: h.uu }\n\nlet ks: [U] keyed by n = h.uu\n\n" +
	"let ol: ([U] keyed by n)? = h.uu\n\n" +
	"let pc: [Bool] = [mm.mk[\"a\"].x == h.uu.x, h.uu.contains(mm.mk[\"a\"].x), mm.lk[0].x == h.uu.x, h.uu.contains(mm.lk[0].x), " +
	"mm.ok!.x == h.uu.x, h.uu.contains(mm.ok!.x), mm.fk.x == h.uu.x, ks.x == h.uu.x, ol!.x == h.uu.x, h.uu.contains(ol!.x)]\n\n" +
	"let wc: [Int] = [mm.mk[\"a\"].x.w, mm.lk[0].x.w, mm.ok!.x.w, mm.fk.x.w, ks.x.w, ol!.x.w, h.uu.x.w]\n"

// heldListPC is pc with no line active: the field and the let hold their own entries, the map
// value, list element, optional field and optional let h's (TYPES.md 6.3; DECISIONS 411).
const heldListPC = "[true, true, true, true, true, true, false, false, true, true]"

// EVALUATION.md 4.2; DECISIONS 510 (ruling of 2026-11-06): an entry amended in a keyed list no field
// or let owns (a map's value, a list's element, an optional field or let) is a record of its own,
// with a node at its place; in a field's or a let's keyed list it stays that collection's.
func TestAmendedHeldKeyedCopy(t *testing.T) {
	for _, c := range []struct{ line, pc, wc, own string }{
		{"amend mm {\n  mk[\"a\"].x.w: 5\n}\n", "[false, false, true, true, true, true, false, false, true, true]", "[5, 0, 0, 0, 0, 0, 0]", "p:mm.mk[a][x]"},
		{"amend mm {\n  lk[0].x.w: 5\n}\n", "[true, true, false, false, true, true, false, false, true, true]", "[0, 5, 0, 0, 0, 0, 0]", "p:mm.lk[0][x]"},
		{"amend mm {\n  ok.x.w: 5\n}\n", "[true, true, true, true, false, false, false, false, true, true]", "[0, 0, 5, 0, 0, 0, 0]", "p:mm.ok[x]"},
		{"amend mm {\n  fk.x.w: 5\n}\n", heldListPC, "[0, 0, 0, 5, 0, 0, 0]", ""},
		{"amend ks {\n  x.w: 5\n}\n", heldListPC, "[0, 0, 0, 0, 5, 0, 0]", ""},
		{"amend ol {\n  x.w: 5\n}\n", "[true, true, true, true, true, true, false, false, false, false]", "[0, 0, 0, 0, 0, 5, 0]", "p:ol[x]"},
	} {
		checkValues(t, openLayered(t, heldListSrc, c.line, false), "DECISIONS 510", map[string]string{"p:pc": heldListPC, "p:wc": "[0, 0, 0, 0, 0, 0, 0]"})
		p := openLayered(t, heldListSrc, c.line, true)
		checkErrors(t, p)
		checkValues(t, p, "EVALUATION.md §4.2; DECISIONS 510", map[string]string{"p:pc": c.pc, "p:wc": c.wc})
		want := []string{"p:h.uu[x]", "p:ks[x]", "p:mm.fk[x]"}
		if c.own != "" {
			want = append(want, c.own)
		}
		if got := elementNodes(t, p); !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))) {
			t.Errorf("DECISIONS 478, 510: element nodes %v, want %v", got, want)
		}
	}
}

package check_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// refLeafBase holds lists keyed by a ref, and one level below each: `rs` (and hr's, ha's; ge's one level)
// keyed by `ref ens`, an enum leaf; `ks` (and mk's) by `ref js`, an Int leaf; `cs` by `ref ks`,
// two links; `us` (and vu's) by `ref tab`, a table's String leaf (DECISIONS 456).
const refLeafBase = `package a

local enum Q {
  a
  b
}

local record E {
  kind: Q
  v: Int = 0
}

local record R {
  r: ref ens
  w: Int = 0
}

local record H {
  rs: [R] keyed by r
}

local record G {
  es: [E] keyed by kind
}

local record J {
  n: Int
}

local record K {
  j: ref js
  w: Int = 0
}

local record M {
  ks: [K] keyed by j
}

local record C {
  c: ref ks
}

local record T {
  w: Int = 0
}

local record U {
  t: ref tab
  w: Int = 0
}

local record V {
  us: [U] keyed by t
}

local let ens: [E] keyed by kind = [{ kind: a, v: 1 }, { kind: b, v: 2 }]

local let rs: [R] keyed by r = [{ r: a, w: 7 }]

local let hr: H = { rs: [{ r: a, w: 8 }, { r: "b", w: 9 }] }

local let ha: H = { rs: [{ r: a }] }

local let ge: G = { es: [{ kind: a }] }

local let js: [J] keyed by n = [{ n: 9 }]

local let ks: [K] keyed by j = [{ j: 9, w: 3 }]

local let mk: M = { ks: [{ j: 9 }] }

local let cs: [C] keyed by c = [{ c: 9 }]

local let tab: table T = {
  y {}
}

local let us: [U] keyed by t = [{ t: y }]

local let vu: V = { us: [{ t: y }] }

local let q: Q = a

local let oq: Q? = a

local let n: Int = 9

local let s: String = "y"

local let re: ref ens = a

local let rj: ref js = 9

local let rk: ref ks = 9

fn pick() -> Q {
  return Q.a
}

fn num() -> Int {
  return 9
}
`

// coded is a catalogue code (diag.E3002 and the rest).
type coded interface{ Def() *diag.Def }

// refLeafCase is one form, written at one level or through a chain (src, and the layer dev's
// amendments when it has any), and the errors a build gives.
type refLeafCase struct {
	name, src, layer string
	want             []coded
}

// TYPES.md §3.3, §3.5, §4.1, §4.2, §9.1, §10.3, STDLIB.md §5, DECISIONS 317, 366, 456, 490.
var refLeafCases = []refLeafCase{
	{"§4.1 one level: ref ens written as a member name", "local let x: ref ens = a", "", nil},
	{"§4.1 chain: ref rs written as a member name (456)", "local let x: ref rs = a", "", nil},
	{"§4.1 one level: ref ens written as a string (366)", "local let x: ref ens = \"a\"", "", nil},
	{"§4.1 chain: ref rs written as a string (366, 456)", "local let x: ref rs = \"a\"", "", nil},
	{"§10.3 one level: a qualified member is no ref", "local let x: ref ens = Q.a", "", []coded{diag.E3002}},
	{"§10.3 chain: a qualified member is no ref (456)", "local let x: ref rs = Q.a", "", []coded{diag.E3002}},
	{"§10.3 a field typed ref ens written Q.a, one level", "local let x: R = { r: Q.a }", "", []coded{diag.E3002}},
	{"§3.5 one level: an enum-keyed list has no .k", "local let x: E = ens.a", "", []coded{diag.E3003}},
	{"§3.5 chain: a list keyed by a ref to an enum leaf has no .k (456)", "local let x: R = hr.rs.a", "", []coded{diag.E3003}},
	{"STDLIB §5 one level: xs[Q.a]", "local let x: E = ens[Q.a]", "", nil},
	{"STDLIB §5 chain: xs[Q.a] (456)", "local let x: R = rs[Q.a]", "", nil},
	{"STDLIB §5 chain through a field: xs[Q.a] (456)", "local let x: R = hr.rs[Q.a]", "", nil},
	{"STDLIB §5 chain through a field: xs[a] (456)", "local let x: R = hr.rs[a]", "", nil},
	{"STDLIB §5 one level: get, find and hasKey of Q.a",
		"local let x: Bool = ens.get(Q.a) != none and ens.find(Q.a) != none and ens.hasKey(Q.a)", "", nil},
	{"STDLIB §5 chain: get, find and hasKey of Q.a (456)",
		"local let x: Bool = rs.get(Q.a) != none and hr.rs.find(Q.a) != none and hr.rs.hasKey(Q.a)", "", nil},
	{"STDLIB §5 one level: a value of the key type", "local let x: E = ens[q]", "", nil},
	{"STDLIB §5 chain: a value of the leaf's type (456)", "local let x: R = rs[q]", "", nil},
	{"STDLIB §5 chain: a call of the leaf's type (456)", "local let x: Bool = rs[pick()].w == 7 and ks[num()].w == 3", "", nil},
	{"STDLIB §5 chain: a value of an Int leaf's type (456)", "local let x: Bool = ks[n].w == 3 and ks.hasKey(n)", "", nil},
	{"STDLIB §5 chain: a value of the key field's own type, unchanged", "local let x: R = rs[re]", "", nil},
	{"STDLIB §5 one level: a value of another type is refused", "local let x: E = ens[n]", "", []coded{diag.E3027}},
	{"STDLIB §5 chain: a value of neither the key's nor its leaf's type is refused (456)", "local let x: R = rs[n]", "", []coded{diag.E3027}},
	{"STDLIB §5 chain: an enum value for an Int leaf is refused (456)", "local let x: K = ks[q]", "", []coded{diag.E3027}},
	{"STDLIB §5 chain: a qualified member for an Int leaf is refused (456)", "local let x: K = ks[Q.a]", "", []coded{diag.E3002}},
	{"§5.1 one level: a context-dependent key", "local let x: Bool = js[4 + 5] == js[-(-9)] and js.hasKey(4 + 5)", "", nil},
	{"§5.1 chain: a context-dependent key is checked against the leaf (456)",
		"local let x: Bool = ks[4 + 5].w == 3 and ks[-(-9)].w == 3 and ks.hasKey(4 + 5)", "", nil},
	{"STDLIB §5 one level: a table's String key", "local let x: Bool = tab[s] == tab[tab.y.id] and tab.hasKey(s)", "", nil},
	{"STDLIB §5 chain: a String leaf through a table (456)",
		"local let x: Bool = us[s].w == 0 and us[tab.y.id] == us[s] and us.hasKey(s)", "", nil},
	{"317 one level: a table's String key where an element is expected is a key", "local let x: Bool = s in tab", "", []coded{diag.E3026}},
	{"317 chain: a String leaf's value where an element is expected is a key (456)", "local let x: Bool = s in us", "", []coded{diag.E3026}},
	{"STDLIB §5 two links: a ref of the key's own type and the leaf's Int",
		"local let x: Bool = cs[rk] == cs[n] and cs[9] == cs[rk]", "", nil},
	{"STDLIB §5 two links: a middle link's ref is a ref into another collection", "local let x: C = cs[rj]", "", []coded{diag.E3002}},
	{"317 one level: a key where an element is expected is a key",
		"local let x: Bool = Q.a in ens or ens.contains(q)", "", []coded{diag.E3026, diag.E3026}},
	{"317 chain: a leaf-typed key where an element is expected is a key (456)",
		"local let x: Bool = Q.a in rs or hr.rs.contains(q) or rs.indexOf(Q.a) != none", "", []coded{diag.E3026, diag.E3026, diag.E3026}},
	{"317 chain: an Int leaf's value where an element is expected is a key (456)", "local let x: Bool = n in ks", "", []coded{diag.E3026}},
	{"§3.3 one level: a member named like a let in scope is the member",
		"local let b: Q = a\n\nlocal let x: Int(2..=2) = ens[b].v\n\nlocal let y: Bool = ens.hasKey(b)", "", nil},
	{"§3.3 chain: a member named like a let in scope is the member (456)",
		"local let b: Q = a\n\nlocal let x: Int(9..=9) = hr.rs[b].w\n\nlocal let y: Bool = hr.rs.get(b) != none", "", nil},
	{"§3.3 chain: the member is looked up, never the let's value (456)", "local let b: Q = a\n\nlocal let x: R = rs[b]", "", []coded{diag.E4002}},
	{"§4.2 one level: a parameter named like a member is ambiguous",
		"fn f(b: Q) -> Int {\n  return ens[b].v\n}", "", []coded{diag.E2101}},
	{"§4.2 chain: a parameter named like a member is ambiguous (456)",
		"fn f(b: Q) -> Int {\n  return hr.rs[b].w\n}", "", []coded{diag.E2101}},
	{"§3.3 chain: a let of the key field's own type is that value, never the member (490)",
		"local let b: ref ens = \"a\"\n\nlocal let x: Int(7..=7) = rs[b].w", "", nil},
	{"§4.2 chain: a parameter of the key field's own type is no ambiguity (490)",
		"fn f(b: ref ens) -> Int {\n  return rs[b].w\n}\n\nlocal let x: Int(7..=7) = f(\"a\")", "", nil},
	{"§4.1 one level: a name in scope that is not a value of an Int key is refused (318, 490)",
		"local let x: Bool = js[q] == js[9] or js[pick] == js[9] or js.hasKey(q)", "", []coded{diag.E3027, diag.E3027, diag.E3027}},
	{"§4.1 chain: a name in scope that is not a value of an Int leaf is refused (318, 490)",
		"local let x: Bool = ks[q] == ks[9] or ks[Q] == ks[9]", "", []coded{diag.E3027, diag.E3027}},
	{"§4.1, §6.5 one level: an optional of the key type must be proved present (490)", "local let x: E = ens[oq]", "", []coded{diag.E3403}},
	{"§4.1, §6.5 chain: an optional of the leaf's type must be proved present (490)", "local let x: R = rs[oq]", "", []coded{diag.E3403}},
	{"§4.1, §6.5 one level: an optional Int key must be proved present (490)",
		"local let on: Int? = 9\n\nlocal let x: J = js[on]", "", []coded{diag.E3403}},
	{"§4.1, §6.5 chain: an optional Int leaf must be proved present (490)",
		"local let on: Int? = 9\n\nlocal let x: K = ks[on]", "", []coded{diag.E3403}},
	{"§4.1, §6.5 chain: a ref C? of the key field's own type must be proved present (303, 490)",
		"local let ore: ref ens? = a\n\nlocal let orj: ref js? = 9\n\nlocal let x: Bool = rs[ore] == rs[a] or ks[orj] == ks[9]", "",
		[]coded{diag.E3403, diag.E3403}},
	{"§6.5, STDLIB §5 one level: an optional ref into the receiver's own collection must be proved present (490)",
		"local let ort: ref tab? = y\n\nlocal let ojs: ref js? = 9\n\nlocal let x: Bool = tab[ort] == tab.y or js[ojs] == js[9]", "",
		[]coded{diag.E3403, diag.E3403}},
	{"§6.5, STDLIB §5 chain: an optional ref into the receiver's own collection must be proved present (490)",
		"local let ors: ref rs? = a\n\nlocal let ohr: ref hr.rs? = a\n\nlocal let x: Bool = rs[ors] == rs[a] or rs.hasKey(ors) or hr.rs[ohr] == hr.rs[a]", "",
		[]coded{diag.E3403, diag.E3403, diag.E3403}},
	{"STDLIB §5 a proved-present ref into the receiver's own collection is a key (490)",
		"local let ors: ref rs? = a\n\nlocal let ojs: ref js? = 9\n\nlocal let x: Bool = rs[ors!] == rs[a] and js.hasKey(ojs!)", "", nil},
	{"§4.1, §6.5 a proved-present optional is a key (490)", "local let x: Bool = ens[oq!] == ens[a] and rs[oq!] == rs[a]", "", nil},
	{"§3.3 step 1 first: a name naming a static key of the expected ref is that entry, the let unread",
		"local let y: Int = 3\n\nlocal let x: U = { t: y }", "", nil},
	{"§4.1 dynamic keys: a name in scope of another type is never the key (318)",
		"local let b: Q = a\n\nlocal let x: R = { r: b }", "", []coded{diag.E3027}},
}

// EVALUATION.md §9.2, TYPES.md §3.3, §4.1, §5.1, §6.5, DECISIONS 456, 490.
var refLeafAmendCases = []refLeafCase{
	{"STDLIB §5 a missing leaf-typed key fails its lookup", "local let x: R = rs[Q.b]", "", []coded{diag.E4002}},
	{"§9.2 one level: [Q.a]", "", "amend ge {\n  es[Q.a].v: 5\n}\n", nil},
	{"§9.2 chain: [Q.a] (456)", "", "amend hr {\n  rs[Q.a].w: 6\n}\n", nil},
	{"§9.2 chain: [q] (456)", "", "amend hr {\n  rs[q].w: 6\n}\n", nil},
	{"§9.2 one level: [pick()]", "", "amend ge {\n  es[pick()].v: 6\n}\n", nil},
	{"§9.2 chain: [pick()] and [num()] (456)", "", "amend hr {\n  rs[pick()].w: 6\n}\n\namend mk {\n  ks[num()].w: 6\n}\n", nil},
	{"§9.2 chain: a context-dependent key (456)", "", "amend mk {\n  ks[4 + 5].w: 6\n}\n", nil},
	{"§9.2 chain: a String leaf through a table (456)", "", "amend vu {\n  us[tab.y.id].w: 6\n}\n\namend vu {\n  us[s].w: 7\n}\n", nil},
	{"§9.2 chain: a missing [Q.b] reaches nothing (456)", "", "amend ge {\n  es[Q.b].v: 6\n}\n\namend hr {\n  rs[Q.a].w: 6\n}\n", []coded{diag.E1905}},
	{"§4.1 one level: a name of another type is refused as in an expression (318, 490)", "", "amend ge {\n  es[n].v: 6\n}\n", []coded{diag.E3027}},
	{"§4.1 chain: a name of neither the key's nor its leaf's type is refused as in an expression (318, 490)", "",
		"amend hr {\n  rs[n].w: 6\n}\n", []coded{diag.E3027}},
	{"§4.1 chain: a name of another type than an Int leaf is refused as in an expression (318, 490)", "",
		"amend mk {\n  ks[q].w: 6\n}\n", []coded{diag.E3027}},
	{"§4.1 chain: a String leaf's wrong-typed name is refused as in an expression (318, 490)", "",
		"amend vu {\n  us[n].w: 6\n}\n", []coded{diag.E3027}},
	{"§4.1 one level and chain: a name that is not a value is refused as in an expression (318, 490)", "",
		"amend ge {\n  es[pick].v: 6\n}\n\namend mk {\n  ks[pick].w: 6\n}\n", []coded{diag.E3027, diag.E3027}},
	{"§4.1, §6.5 chain: an optional Int leaf must be proved present (490)", "local let on: Int? = 9", "amend mk {\n  ks[on].w: 6\n}\n", []coded{diag.E3403}},
	{"§4.1, §6.5 chain: a ref C? of the key field's own type must be proved present (303, 490)",
		"local let ore: ref ens? = a", "amend hr {\n  rs[ore].w: 6\n}\n", []coded{diag.E3403}},
	{"§6.5 one level: an optional key must be proved present", "", "amend ge {\n  es[oq].v: 6\n}\n", []coded{diag.E3403}},
	{"§6.5 chain: an optional key must be proved present (456)", "", "amend hr {\n  rs[oq].w: 6\n}\n", []coded{diag.E3403}},
	{"§9.2 chain: a ref into the amended list itself is a key (490)",
		"local let rh: ref hr.rs = a\n\nlocal let x: Int(6..=6) = hr.rs[a].w", "amend hr {\n  rs[rh].w: 6\n}\n", nil},
	{"§6.5 chain: an optional ref into the amended list itself must be proved present (490)",
		"local let rh: ref hr.rs? = a\n\nlocal let rm: ref mk.ks? = 9", "amend hr {\n  rs[rh].w: 6\n}\n\namend mk {\n  ks[rm].w: 6\n}\n",
		[]coded{diag.E3403, diag.E3403}},
	{"§3.3 one level: a member named like a let is the member", "local let b: Q = a", "amend ge {\n  es[b].v: 6\n}\n", []coded{diag.E1905}},
	{"§3.3 chain: a member named like a let is the member (456)", "local let b: Q = a", "amend ha {\n  rs[b].w: 6\n}\n", []coded{diag.E1905}},
	{"§3.3 chain: a let of the key field's own type is that value, never the member (490)",
		"local let b: ref ens = \"a\"", "amend ha {\n  rs[b].w: 6\n}\n", nil},
	{"§9.2 chain: a selected value of neither the key's nor its leaf's type is refused (456)", "", "amend mk {\n  ks[Q.a].w: 6\n}\n", []coded{diag.E3002}},
}

func TestRefLeafKeys(t *testing.T) {
	for _, tc := range refLeafCases {
		t.Run(tc.name, func(t *testing.T) {
			wantBuilt(t, tc)
		})
	}
}

func TestRefLeafAmendKeys(t *testing.T) {
	for _, tc := range refLeafAmendCases {
		t.Run(tc.name, func(t *testing.T) {
			wantBuilt(t, tc)
		})
	}
}

// wantBuilt builds tc's source over refLeafBase, its layer dev applied when it has one: the build
// reports exactly the errors tc wants.
func wantBuilt(t *testing.T, tc refLeafCase) {
	t.Helper()
	text := refLeafBase + "\n" + tc.src + "\n"
	var layer []string
	if tc.layer != "" {
		layer = append(layer, "package a\nlayer dev\n\n"+tc.layer)
	}
	res := buildUnder(t, shopProject, text, layer...)
	want := make([]diag.Code, 0, len(tc.want))
	for _, c := range tc.want {
		want = append(want, c.Def().Code)
	}
	if got := errorCodes(res.List); !slices.Equal(got, sortedCodes(want)) {
		t.Errorf("codes %v, want %v:\n%v", got, sortedCodes(want), res.List)
	}
}

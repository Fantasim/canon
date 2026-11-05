package check_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// staticItems is a list keyed by a ref into a table with static keys, shield and axe, and a let
// named like one of its entries: shieldLet declares it, of the key field's own type or another.
const staticItems = `local record I {
  v: Int = 0
}

local record S {
  item: ref items
  n: Int = 0
}

local record One {
  ts: [S] keyed by item
}

local record Many {
  one: One
}

local let items: table I = {
  shield {}
  axe {}
}

local let many: Many = { one: { ts: [{ item: shield, n: 1 }, { item: axe, n: 2 }] } }
`

const (
	shieldRef = staticItems + "\nlocal let shield: ref items = axe\n"
	shieldInt = staticItems + "\nlocal let shield: Int = 3\n"
	amendFive = "amend many {\n  one.ts[shield].n: 5\n}\n"
)

// TYPES.md §3.3, §4.1, §4.2, §6.2, §6.5, §7.5, STDLIB.md §5, DECISIONS 318, 490, 493.
var refKeyCodeCases = []refLeafCase{
	{"§4.1 static keys: a name in scope of another type naming no entry is no key, as with dynamic keys (318, 493)",
		"local let z: Int = 1\n\nlocal let x: U = { t: z }\n\nlocal let r: ref tab = z", "", []coded{diag.E3027, diag.E3027}},
	{"§4.1 static keys: a name in scope that is not a value is no key (318, 493)", "local let r: ref tab = pick", "", []coded{diag.E3027}},
	{"§6.2 static keys: a ref into another collection stays a wrong type", "local let rr: ref rs = a\n\nlocal let r: ref tab = rr", "", []coded{diag.E3002}},
	{"§4.1, §6.5 static keys: an optional of the ref must be proved present", "local let ort: ref tab? = y\n\nlocal let r: ref tab = ort", "", []coded{diag.E3403}},
	{"§4.1 ref position, Int leaf: a name in scope that is not a value is no key (493)", "local let r: ref js = pick", "", []coded{diag.E3027}},
	{"§6.5 an optional ref into another collection is a wrong type, as the plain ref is (493)",
		"local let orr: ref rs? = a\n\nlocal let x: Bool = ens[orr] == ens[a] or js[orr] == js[9] or tab[orr] == tab.y", "",
		[]coded{diag.E3002, diag.E3002, diag.E3002}},
	{"§6.5 amend: an optional ref into another collection is a wrong type, as the plain ref is (493)",
		"local let orr: ref rs? = a", "amend ge {\n  es[orr].v: 6\n}\n\namend mk {\n  ks[orr].w: 6\n}\n", []coded{diag.E3002, diag.E3002}},
	{"§4.1 an optional of another type than a ref is a name of another type (493)",
		"local let ob: Bool? = true\n\nlocal let x: E = ens[ob]", "", []coded{diag.E3027}},
	{"§3.3 step 1 static keys: an expression's key names the entry, never the let of the key's own type",
		shieldRef + "\nlocal let x: Int(1..=1) = many.one.ts[shield].n\n\nlocal let y: Int(1..=1) = many.one.ts.get(shield)!.n", "", nil},
	{"§3.3 step 1 static keys: an expression and an amend line name the same entry",
		shieldRef + "\nlocal let x: Int(5..=5) = many.one.ts[shield].n\n\nlocal let y: Int(2..=2) = many.one.ts[axe].n", amendFive, nil},
	{"§3.3 step 1 static keys: a let of another type is shadowed, in an expression and an amend line",
		shieldInt + "\nlocal let x: Int(5..=5) = many.one.ts[shield].n\n\nlocal let y: Bool = many.one.ts.hasKey(shield)", amendFive, nil},
	{"§4.2 static keys: a parameter of the key's type named like an entry is ambiguous",
		staticItems + "\nfn f(shield: ref items) -> Int {\n  return many.one.ts[shield].n\n}", "", []coded{diag.E2101}},
	{"§3.3 dynamic keys: a let of the key field's own type is that value (490)",
		"local let b: ref ens = \"b\"\n\nlocal let x: Int(9..=9) = hr.rs[b].w", "", nil},
	{"§4.1 ref position, Int leaf: a value of another type is no key, a literal is (493)",
		"local let bo: Bool = true\n\nlocal let a1: ref js = q\n\nlocal let a2: ref js = n\n\nlocal let a3: K = { j: n }\n\nlocal let a4: ref ks = bo", "",
		[]coded{diag.E3027, diag.E3027, diag.E3027, diag.E3027}},
	{"§4.1 ref position, Int leaf: a key literal is the key", "local let a1: ref js = 9\n\nlocal let a2: K = { j: 9 }", "", nil},
	{"§6.2 dynamic keys: a ref into another collection is a wrong type, as with tables (493)",
		"fn f(a: ref ens) -> ref js {\n  return a\n}\n\nlocal let x: ref js = rr\n\nlocal let rr: ref rs = a", "", []coded{diag.E3002, diag.E3002}},
	{"§6.2, §6.5 dynamic keys: an optional ref into another collection is a wrong type (493)",
		"local let orr: ref rs? = a\n\nlocal let x: ref js = orr", "", []coded{diag.E3002}},
	{"§7.5 dynamic keys: refs into two collections compared, in either order (493)",
		"fn f(a: ref ens, b: ref js) -> Bool {\n  return a == b or b == a\n}", "", []coded{diag.E3309, diag.E3309}},
	{"§7.5 a table's ref and a keyed list's ref compared, in either order (493)",
		"fn f(a: ref tab, b: ref js) -> Bool {\n  return a == b or b == a\n}", "", []coded{diag.E3309, diag.E3309}},
	{"§4.2 static keys: a parameter of the leaf's type named like an entry is ambiguous (490)",
		staticItems + "\nfn f(shield: String) -> Int {\n  return many.one.ts[shield].n\n}", "", []coded{diag.E2101}},
	{"§4.2 static keys: a local of the leaf's type named like an entry is ambiguous (490)",
		staticItems + "\nfn f() -> Int {\n  let shield: String = \"axe\"\n  return many.one.ts.get(shield)!.n\n}", "", []coded{diag.E2101}},
	{"§4.2 static keys: a field of the leaf's type named like an entry is ambiguous (490)",
		staticItems + "\nlocal record W {\n  shield: String = \"axe\"\n\n  fn n() -> Int {\n    return many.one.ts[shield].n\n  }\n}", "", []coded{diag.E2101}},
	{"§4.2 static keys: a parameter of neither the key's nor the leaf's type is shadowed silently",
		staticItems + "\nfn f(shield: Int) -> Int {\n  return many.one.ts[shield].n\n}\n\nlocal let x: Int(1..=1) = f(2)", "", nil},
	{"§4.1 dynamic keys, Int leaf: a parameter of the leaf's type is the key, with no step 1 (490)",
		"fn f(n: Int) -> Int {\n  return ks[n].w\n}\n\nlocal let x: Int(3..=3) = f(9)", "", nil},
	{"§7.5 static keys: a value that is no ref compared with a ref, in either order",
		"local let ri: ref tab = y\n\nlocal let x: Bool = ri == q or q == ri or ri == s or s == ri", "",
		[]coded{diag.E3002, diag.E3002, diag.E3002, diag.E3002}},
	{"§7.5 dynamic keys: a value that is no ref compared with a ref, in either order",
		"local let x: Bool = rj == q or q == rj or rj == s or s == rj", "",
		[]coded{diag.E3002, diag.E3002, diag.E3002, diag.E3002}},
}

func TestRefKeyCodes(t *testing.T) {
	for _, tc := range refKeyCodeCases {
		t.Run(tc.name, func(t *testing.T) {
			wantBuilt(t, tc)
		})
	}
}

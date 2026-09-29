package check_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
)

// shop is a package whose entries live in files of their own and name each other, a keyed
// list with entries too, and declarations reading entries: a let, a check, a view, a layer.
var shop = map[string]string{
	"shop/shop.canon": `package shop

/// A thing sold.
record Item {
  /// Its name.
  name: String(1..)
  /// Its price.
  price: Int(0..=LIMIT)
  /// The item it needs, if any.
  needs: ref items?
  /// How many are in stock.
  stock: Int(0..=99) = 5
  /// What kind of thing it is.
  kind: Kind = TOOL
}

/// A kind of thing.
variant Kind {
  FOOD {
    /// Its energy.
    kcal: Int = 100
  }
  TOOL
}

/// The highest price.
const LIMIT = 100

/// Every item.
let items: table Item = {}

/// A slot.
record Slot {
  /// Its code.
  code: Int
  /// Its size.
  size: Int(1..=9)
}

/// Every slot, by code.
let slots: [Slot] keyed by code = [{ code: 1, size: 2 }]

/// The price of the apple.
let applePrice: Int = items.apple.price

/// The price of the bread.
let breadPrice: Int = items.bread.price

check items.values().map(.price).isUnique() else "prices are unique"

view Item {
  title "{name}"
  columns { price 60 }
}
`,
	"shop/dev.layer.canon": `package shop
layer dev

amend items {
  apple.price: 1
}
`,
	"shop/items/apple.canon": `package shop

/// An apple.
entry items.apple {
  name: "Apple"
  price: 3
  kind: FOOD { kcal: 52 }
}

entry items.pear {
  name: "Pear"
  price: 4
  needs: apple
  stock: 7
  kind: FOOD {}
}
`,
	"shop/items/bread.canon": `package shop

@deprecated("stale")
entry items.bread {
  name: "Bread"
  price: 5
  needs: pear
}

entry slots.2 { size: 3 }
`,
}

// step is one edit of a chain: the file, the text replaced and its replacement, and whether
// the edit is body-only.
type step struct {
	path, old, new string
	ok             bool
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: Recheck after a body edit equals a cold Check; a signature edit is refused.
func TestRecheckEqualsCold(t *testing.T) {
	cases := []struct {
		name  string
		srcs  map[string]string
		steps []step
	}{
		{"shop apple, earlier path, higher FileID", shop, []step{
			{"shop/items/apple.canon", "price: 3", "price: 7", true},
			{"shop/items/apple.canon", "price: 7", `price: "seven"`, true},
			{"shop/items/apple.canon", `price: "seven"`, "price: 3", true},
			{"shop/items/apple.canon", "price: 3", "price: 3 +", true},
			{"shop/items/apple.canon", "price: 3 +", "price: 3", true},
			{"shop/items/apple.canon", `name: "Apple"`, `name: "App\q"`, true},
			{"shop/items/apple.canon", `name: "App\q"`, `name: "Apple"`, true},
			{"shop/items/apple.canon", `name: "Apple"`, `name: ""`, true},
			{"shop/items/apple.canon", "needs: apple", "needs: plum", true},
			{"shop/items/apple.canon", "needs: plum", "needs: bread", true},
			{"shop/items/apple.canon", "/// An apple.", "/// A red apple.", true},
			{"shop/items/apple.canon", "price: 4", "price: 4 + LIMIT", true},
			{"shop/items/apple.canon", "kcal: 52", `kcal: "x"`, true},
			{"shop/items/apple.canon", `FOOD { kcal: "x" }`, "TOOL", true},
			{"shop/items/apple.canon", "kind: FOOD {}", "kind: SEED {}", true},
			{"shop/items/apple.canon", "entry items.apple", "entry items.apples", false},
			{"shop/items/apple.canon", "entry items.pear", "retired entry items.pear", false},
			{"shop/items/apple.canon", "/// A red apple.", "@deprecated\n", false},
		}},
		{"shop bread and slots", shop, []step{
			{"shop/items/bread.canon", "price: 5", "price: 6", true},
			{"shop/items/bread.canon", "size: 3", "size: 12", true},
			{"shop/items/bread.canon", "size: 12", "size: 3", true},
			{"shop/items/bread.canon", `"stale"`, `"old"`, false},
			{"shop/items/bread.canon", "slots.2", "slots.3", false},
			{"shop/items/bread.canon", "entry slots.2 { size: 3 }", "", false},
			{"shop/items/bread.canon", "package shop\n", "package shop\n\nimport shop\n", false},
			{"shop/shop.canon", "size: 2 }", "size: 3 }", false},
			{"shop/dev.layer.canon", "price: 1", "price: 2", false},
		}},
		{"a fold of a const broken later", brokenLater, []step{
			{"p/x.canon", "n: 1", "n: 2", true},
		}},
		{"duplicate keys across files", dups, []step{
			{"dup/a.canon", "n: 1", "n: 11\n\n\n", true},
			{"dup/b.canon", "n: 2", "n: 22", true},
			{"dup/a.canon", "label: \"a\"", "label: 7", true},
			{"dup/c.canon", "label: \"c\"", "label: \"cc\"\n\n", true},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t, tc.srcs)
			_, s, _ := w.session()
			for _, st := range tc.steps {
				s = w.step(s, st)
			}
		})
	}
}

// step applies one edit to the latest session and compares the result with a cold Check.
func (w *world) step(s *check.Session, st step) *check.Session {
	w.t.Helper()
	nf := w.edit(st.path, st.old, st.new)
	prog, next, bags, ok := w.recheck(s, nf)
	if ok != st.ok {
		w.t.Fatalf("%s: %q to %q: Recheck ok=%v, want %v", st.path, st.old, st.new, ok, st.ok)
	}
	if !ok {
		return s
	}
	if got, want := canonical(w.fs, prog, bags), w.cold(); got != want {
		w.t.Fatalf("%s: %q to %q: Recheck differs from Check:\n%s", st.path, st.old, st.new, firstDiff(got, want))
	}
	if stale := staleRefs(prog); len(stale) > 0 {
		w.t.Fatalf("%s: Info names objects of a replaced file: %v", st.path, stale)
	}
	return next
}

// brokenLater folds a bound reading a const that E2111 breaks only at the end of its package:
// the fold saw it unbroken and reported E4102.
var brokenLater = map[string]string{
	"p/p.canon": "package p\n\nlocal enum E { a, b }\n\n/// L.\nconst L = [E.a]\n\nlocal record R {\n  v: Int(0..=L.len() / 0)\n}\n\nlocal record Row {\n  n: Int\n}\n\nlocal let rows: table Row = {}\n",
	"p/x.canon": "package p\n\nentry rows.x { n: 1 }\n",
}

// dups holds a key given twice across files, first in the earlier path (E3101), and a keyed
// list key written by its literal and by entries of two files (E3102).
var dups = map[string]string{
	"dup/dup.canon": `package dup

local record Row {
  n: Int
}

local let rows: table Row = {}

local record Tag {
  code: Int
  label: String
}

local let tags: [Tag] keyed by code = [{ code: 1, label: "one" }]

local const ZERO = 0

local record Box {
  v: Int(0..=10 / ZERO)
}
`,
	"dup/a.canon": `package dup

entry rows.x { n: 1 }

entry tags.1 { label: "a" }

entry tags.2 { label: "b" }
`,
	"dup/b.canon": `package dup

entry rows.x { n: 2 }

entry rows.y { n: 3 }
`,
	"dup/c.canon": `package dup

entry rows.y { n: 4 }

entry tags.2 { label: "c" }
`,
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: unchanged declarations keep their pointers, the old program its facts.
func TestRecheckKeepsTheOldProgram(t *testing.T) {
	w := newWorld(t, shop)
	old, s, oldBags := w.session()
	before := canonical(w.fs, old, oldBags)
	nf := w.edit("shop/items/apple.canon", "price: 3", "price: 8")
	prog, _, _, ok := w.recheck(s, nf)
	if !ok {
		t.Fatal("Recheck refused a body-only edit")
	}
	if after := canonical(w.fs, old, oldBags); after != before {
		t.Fatalf("the old program changed:\n%s", firstDiff(after, before))
	}
	for i, p := range prog.Packages {
		for k, o := range p.Decls {
			was := old.Packages[i].Decls[k]
			if changed := o.File() == nf; changed == (o == was) {
				t.Errorf("%s: changed=%v, same pointer=%v", objText(o), changed, o == was)
			}
			if o.File() != nf && o.Type() != was.Type() {
				t.Errorf("%s: its type is a new pointer", objText(o))
			}
		}
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: a stale session, a cancelled context and an unknown path are refused.
func TestRecheckRefuses(t *testing.T) {
	w := newWorld(t, shop)
	_, s, _ := w.session()
	nf := w.edit("shop/items/apple.canon", "price: 3", "price: 9")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bags := check.Bags{}
	w.parseInto(bags, nf)
	if _, _, ok := s.Recheck(ctx, []*syntax.File{nf}, bags, eval.NewFolder(bags, eval.Options{})); ok {
		t.Error("Recheck under a cancelled context")
	}
	stray := w.parse("shop/items/plum.canon", "package shop\n\nentry items.plum { name: \"Plum\", price: 1 }\n")
	if _, _, _, ok := w.recheck(s, stray); ok {
		t.Error("Recheck of a file the session does not hold")
	}
	if _, _, _, ok := w.recheck(s, nf); !ok {
		t.Fatal("Recheck refused a body-only edit")
	}
	again := w.edit("shop/items/apple.canon", "price: 9", "price: 10")
	if _, _, _, ok := w.recheck(s, again); ok {
		t.Error("Recheck of a session that is no longer its lineage's latest")
	}
}

// firstDiff is the first differing line of two texts, with its neighbours.
func firstDiff(got, want string) string {
	g, h := splitLines(got), splitLines(want)
	for i := 0; i < len(g) || i < len(h); i++ {
		if i >= len(g) || i >= len(h) || g[i] != h[i] {
			return "got:  " + lineAt(g, i) + "\nwant: " + lineAt(h, i)
		}
	}
	return ""
}

func splitLines(s string) []string { return strings.Split(s, "\n") }

func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "<end>"
}

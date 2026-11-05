package eval_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	rewardSrc = `/// A reward: gold, or an item.
type Reward(item: Bool) = match item {
  false => Int(1..)
  true => String(1..)
}

/// A quest.
record Q {
  /// Whether its rewards are items.
  items: Bool = false
  /// What it gives.
  rewards: [Reward(items)] = []
}

/// The quest.
let q: Q = { items: false }

/// The quests.
let qs: table Q = {
  a { items: false }
}
`
	rewardDev = "package a\nlayer dev\n\namend q {\n  items: true\n}\n\namend qs {\n  a.items: true\n}\n"
	shopSrc   = `/// Item.
record Item {
  /// Id.
  id: String
  /// W.
  w: Int
}

/// Shop.
record Shop {
  /// Items.
  items: [Item] keyed by id
  /// Fav.
  fav: ref Item
}

/// The shop.
let q: Shop = { items: [{ id: "a", w: 1 }, { id: "b", w: 2 }], fav: "a" }
`
	shopDev = "package a\nlayer dev\n\namend q {\n  items[a].w: 5\n  fav: \"b\"\n}\n"
)

// CLI.md §3.7, EVALUATION.md §9.3: Produced is the base value, through every copy of an amendment's copy (verification's, an owner's).
func TestProducedThroughEveryCopy(t *testing.T) {
	opt := eval.Options{Layers: []string{"dev"}}
	const shopBase = `Shop{items: [Item{id: "a", w: 1}, Item{id: "b", w: 2}], fav: a}`
	for _, c := range []struct {
		name, src, dev, root string
		at                   []int // the part: a field by index, else an element
		want                 string
	}{
		{"verified let", rewardSrc, rewardDev, "q", nil, "Q{items: false, rewards: []}"},
		{"verified table", rewardSrc, rewardDev, "qs", nil, "{a: Q{items: false, rewards: []}}"},
		{"verified entry", rewardSrc, rewardDev, "qs", []int{0}, "Q{items: false, rewards: []}"},
		{"owner", shopSrc, shopDev, "q", nil, shopBase},
		{"owned list", shopSrc, shopDev, "q", []int{0}, `[Item{id: "a", w: 1}, Item{id: "b", w: 2}]`},
		{"amended entry", shopSrc, shopDev, "q", []int{0, 0}, `Item{id: "a", w: 1}`},
		{"rebound entry", shopSrc, shopDev, "q", []int{0, 1}, `Item{id: "b", w: 2}`},
	} {
		b := buildFiles(t, opt, "a/a.canon", pkgA+c.src, "a/dev.layer.canon", c.dev)
		v, ok := b.values[eval.Root{Pkg: "a", Name: c.root}]
		if !ok {
			t.Fatalf("%s: %s not evaluated", c.name, c.root)
		}
		for _, i := range c.at {
			if rec, isRec := v.(*value.Record); isRec {
				v = rec.Fields[i]
			} else {
				v = std.Elems(v)[i]
			}
		}
		if got := b.ev.Produced(v).CanonText(); got != c.want {
			t.Errorf("%s: Produced(%s) = %s, want %s", c.name, v.CanonText(), got, c.want)
		}
	}
}

const (
	narrowSrc = `/// A number: narrow, or whole.
type N(f: Bool) = match f {
  true => Float32
  false => Int
}

/// X.
record X {
  /// Whether n is a float.
  f: Bool = true
  /// The number.
  n: N(f)
  /// Unrelated.
  k: Int = 0
}

/// The x.
let x: X = { n: 0.123456789123 }

/// P.
record P {
  /// A plain narrow number.
  v: Float32
}

/// The p.
let p: P = { v: 0.123456789123 }
`
	narrowDev = "package a\nlayer dev\n\namend x {\n  k: 1\n}\n"
)

// DECISIONS 468, TYPES.md §11.6, EVALUATION.md §4.3: Produced is the value evaluated, a dependent n as written, a plain Float32 rounded, with or without dev.
func TestProducedBeforeVerification(t *testing.T) {
	for _, layers := range [][]string{nil, {"dev"}} {
		opt := eval.Options{Layers: layers}
		b := buildFiles(t, opt, "a/a.canon", pkgA+narrowSrc, "a/dev.layer.canon", narrowDev)
		for root, want := range map[string]string{"x": "X{f: true, n: 0.123456789123, k: 0}", "p": "P{v: 0.12345679}"} { //canon:unordered each root is judged on its own
			v, ok := b.values[eval.Root{Pkg: "a", Name: root}]
			if !ok {
				t.Fatalf("layers %v: %s not evaluated", layers, root)
			}
			if got := b.ev.Produced(v).CanonText(); got != want {
				t.Errorf("layers %v: Produced(%s) = %s, want %s", layers, v.CanonText(), got, want)
			}
		}
	}
}

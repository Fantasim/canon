package std_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// unionSource declares the maps the union cases merge.
const unionSource = `/// A.
package a

/// A tone.
enum Tone { soft, loud, sharp }

/// A pet.
record Pet {
  /// Its age.
  age: Int
}

/// Pets.
let pets: table Pet = {
  rex { age: 3 }
  tom { age: 1 }
}

local let a: {String: Int} = { "a": 1, "b": 2 }
local let b: {String: Int} = { "z": 26, "b": 20, "c": 3 }
local let c: {String: Int} = { "c": 30, "d": 4, "a": 10 }
local let empty: {String: Int} = {}

local let ordered: {String: Int} = a.union({ "z": 26, "c": 3 })
local let firstWins: {String: Int} = a.union(b)
local let lastWins: {String: Int} = b.union(a)
local let emptyReceiver: {String: Int} = empty.union(b)
local let emptyArg: {String: Int} = a.union(empty)
local let emptyLiteral: {String: Int} = a.union({})
local let three: {String: Int} = a.union(b).union(c)
local let tones: {Tone: Int} = { loud: 1 }
local let toneUnion: {Tone: Int} = tones.union({ soft: 2, loud: 3, sharp: 4 })
local let byPet: {ref pets: Int} = { rex: 1 }
local let others: {ref pets: Int} = { tom: 2, rex: 9 }
local let petUnion: {ref pets: Int} = byPet.union(others)
local let rexKept: Int = byPet.union(others)[rex]

/// A kind.
enum K { num, col, text }

/// A node.
record Node {
  /// Its kind.
  k: K
}

/// Nodes.
let nodes: table Node = { a { k: num }, b { k: text }, c { k: col } }

/// A key per node kind.
type SK(e: Node) = match e.k {
  col => Tone
  _ => String
}

/// A holder.
record H {
  /// The node its keys follow.
  like: ref nodes
  /// By key.
  ks: {SK(like): Int} = {}
}

local let hs: [H] = [{ like: c, ks: { soft: 1 } }]
local let keyUnion = hs[0].ks.union({ loud: 8, soft: 3 })
`

// STDLIB.md §6, DECISIONS 338, TYPES.md §7.5: a's entries, then b's new keys in b order.
func TestMapUnion(t *testing.T) {
	out, values := evaluateAll(t, &txtar.Archive{Files: []txtar.File{{Name: "a/a.canon", Data: []byte(unionSource)}}})
	if !strings.Contains(out, "0 errors") {
		t.Fatalf("findings:\n%s", out)
	}
	for _, c := range []struct{ rule, name, want string }{
		{"order: a's entries, then b's in b order", "ordered", `{"a": 1, "b": 2, "z": 26, "c": 3}`},
		{"first wins on a shared key", "firstWins", `{"a": 1, "b": 2, "z": 26, "c": 3}`},
		{"b.union(a) is the last-wins merge", "lastWins", `{"z": 26, "b": 20, "c": 3, "a": 1}`},
		{"empty receiver: b", "emptyReceiver", `{"z": 26, "b": 20, "c": 3}`},
		{"empty b: the receiver", "emptyArg", `{"a": 1, "b": 2}`},
		{"empty literal b: the receiver", "emptyLiteral", `{"a": 1, "b": 2}`},
		{"three maps merged first-wins", "three", `{"a": 1, "b": 2, "z": 26, "c": 3, "d": 4}`},
		{"enum keys", "toneUnion", `{loud: 1, soft: 2, sharp: 4}`},
		{"ref keys equal by key", "petUnion", `{rex: 1, tom: 2}`},
		{"the receiver's value of a shared ref key", "rexKept", `1`},
		{"TYPES.md §11.5 a dependent-key map", "keyUnion", `{soft: 1, loud: 8}`},
	} {
		if got := values["a."+c.name]; got != c.want {
			t.Errorf("STDLIB.md §6, DECISIONS 338 %s: %s = %s, want %s", c.rule, c.name, got, c.want)
		}
	}
}

// STDLIB.md §6, DECISIONS 338, §1.3: n + len(b) steps, plus one per shared key compared.
func TestMapUnionCost(t *testing.T) {
	strs := func(keys ...string) *value.Map {
		m := &value.Map{T: unionType}
		for i, k := range keys {
			m.Keys = append(m.Keys, &value.Str{V: k, T: types.StringType})
			m.Vals = append(m.Vals, &value.Int{V: int64(i), T: types.IntType})
		}
		return m
	}
	for _, c := range []struct {
		a, b   *value.Map
		shared int
	}{
		{strs(), strs(), 0},
		{strs("a", "b", "c"), strs(), 0},
		{strs(), strs("a", "b"), 0},
		{strs("a", "b", "c"), strs("d", "e"), 0},
		{strs("a", "b", "c"), strs("c", "x", "a", "y"), 2},
	} {
		h := &countingHost{}
		got, ok := std.Method(h, &std.Call{Name: "union", Recv: c.a, Args: []value.Value{c.b}, Result: unionType})
		if !ok || got == nil {
			t.Fatalf("union of %s and %s failed", c.a.CanonText(), c.b.CanonText())
		}
		if want := len(c.a.Keys) + len(c.b.Keys) + c.shared; h.steps != want {
			t.Errorf("%s.union(%s): %d steps, want n + len(b) + %d pairs = %d", c.a.CanonText(), c.b.CanonText(), h.steps, c.shared, want)
		}
	}
}

// unionType is the {String: Int} of the cost cases.
var unionType = &types.MapType{Key: types.StringType, Value: types.IntType}

// countingHost counts the steps a built-in charges; equality costs a step per pair (DECISIONS 197).
type countingHost struct {
	steps int
}

func (h *countingHost) Invoke(value.Value, ...value.Value) (value.Value, bool) { return nil, false }

func (h *countingHost) Charge(n int) bool {
	h.steps += n
	return true
}

func (h *countingHost) Remaining() int                                     { return 1 << 30 }
func (h *countingHost) Fail(*diag.Builder)                                 {}
func (h *countingHost) Site() source.Span                                  { return source.Span{} }
func (h *countingHost) Entry(value.Value, value.Key) (*value.Record, bool) { return nil, false }
func (h *countingHost) Regexp(string) *regexp.Regexp                       { return nil }
func (h *countingHost) Key(_ *value.Map, k value.Value) value.Value        { return k }

func (h *countingHost) Belongs(*value.Record, *types.Collection) (bool, bool) {
	return false, true
}

func (h *countingHost) Equal(a, b value.Value) (bool, bool) {
	h.steps++
	return value.Equal(a, b), true
}

func (h *countingHost) Coerce(v value.Value, _ types.Type) (value.Value, bool) { return v, true }

// TYPES.md §11.4, DECISIONS 338: a dependent value's literal where evaluation binds its arguments evaluates, no finding.
func TestDependentLiterals(t *testing.T) {
	a, err := txtar.ParseFile("testdata/dependent_literals.txtar")
	if err != nil {
		t.Fatal(err)
	}
	out, values := evaluateAll(t, a)
	if !strings.Contains(out, "0 errors") {
		t.Fatalf("findings:\n%s", out)
	}
	for name, want := range map[string]string{
		"a.top":         `{a: Sub{lim: 5}, d: Sub{lim: 4}, n: 2}`,
		"a.tops":        `{a: L{top: 5}, d: L{top: 7}}`,
		"a.nested":      `{a: {"k": L{top: 5}}}`,
		"a.zs":          `{d: {"k": L{top: 7}}, a: {"j": L{top: 5}}}`,
		"a.zo":          `{d: {o1: L{top: 7}, o2: L{top: 7}}}`,
		"a.fromStrings": `7`,
		"a.fromRefs":    `7`,
	} {
		if got := values[name]; got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
}

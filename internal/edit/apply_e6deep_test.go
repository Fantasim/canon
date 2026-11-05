package edit_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// deepSrc holds records whose objects a `partial` load keeps unknown keys in: beside `@json(pairs:)`
// slots (a slot's fields are scalars, WIRE.md 4.1), inside the objects `@json(path:)` creates, in a
// variant case holding both; and an inline variant one level down (DECISIONS 471 round 5).
const deepSrc = `package d

/// A pair.
record PB {
  /// K.
  k: Int
  /// V.
  v: Int
}

/// Slots between two plain fields.
record HP {
  /// N.
  n: Int = 0
  /// The slots.
  st: [PB](..=2) = [] @json(pairs: ["k{i}", "v{i}"])
  /// M.
  m: Int = 0
}

/// Holds an HP, its default not HP's own.
record HHP {
  /// The holder.
  h: HP = { m: 6 }
}

/// A variant whose first case holds slots and a path.
variant VS {
  /// With slots.
  sl {
    /// The slots.
    st: [PB](..=2) = [] @json(pairs: ["k{i}", "v{i}"])
    /// A, under a path.
    a: Int = 0 @json(path: "lg.a")
  }
  /// Plain, under the same path.
  pl {
    /// W.
    w: Int = 3 @json(path: "lg.w")
  }
}

/// Holds a VS.
record HVS {
  /// The shape.
  vs: VS = pl { w: 4 }
}

/// Fields under a path.
record PP {
  /// A.
  a: Int = 1 @json(path: "lg.a")
  /// N.
  n: Int = 0
  /// B.
  b: Int = 2 @json(path: "lg.b")
}

/// Holds a PP, its default not PP's own.
record HPP {
  /// The holder.
  h: PP = { b: 7 }
}

/// Two levels of path.
record P2 {
  /// C.
  c: Int = 0 @json(path: "x.y.c")
  /// D.
  d: Int = 0 @json(path: "x.d")
}

/// Holds a P2.
record HP2 {
  /// The holder.
  h: P2 = {}
}

/// A plain part.
record Q {
  /// Q.
  q: Int = 1
  /// R.
  r: Int = 2
}

/// Holds a Q, its default not Q's own.
record HQ {
  /// The part.
  o: Q = { r: 7 }
}

/// A shape tagged type.
variant Sh @json(tag: "type") {
  /// A circle.
  circle {
    /// R.
    r: Int = 1 @json("R")
  }
  /// A square.
  square {
    /// S.
    s: Int = 2
  }
}

/// A shape held inline.
record HJ {
  /// The shape.
  sh: Sh = square { s: 4 } @json(inline)
  /// Z.
  z: Int
}

/// Holds an HJ.
record HH {
  /// The holder.
  h: HJ
}

/// Slots, a key before them.
let jpa1: HHP = load("pa1.json", partial: true)

/// Slots, a key between them.
let jpa2: HHP = load("pa2.json", partial: true)

/// Slots, a key after them.
let jpa3: HHP = load("pa3.json", partial: true)

/// Slots, keys no slot claims: past the last, a leading zero, a sign.
let jpa4: HHP = load("pa4.json", partial: true)

/// A case holding slots and a path.
let jvs: HVS = load("vs.json", partial: true)

/// A path, a key before.
let jpp1: HPP = load("pp1.json", partial: true)

/// A path, a key between.
let jpp2: HPP = load("pp2.json", partial: true)

/// A path, a key after.
let jpp3: HPP = load("pp3.json", partial: true)

/// A path, a key after a removed one.
let jpp4: HPP = load("pp4.json", partial: true)

/// Two levels, a key in the inner object.
let jpx1: HP2 = load("px1.json", partial: true)

/// Two levels, a key in the outer object.
let jpx2: HP2 = load("px2.json", partial: true)

/// Two levels, a key beside them.
let jpx3: HP2 = load("px3.json", partial: true)

/// Plain, a key after a removed one.
let jpc: HQ = load("pc.json", partial: true)

/// An inline variant one level down, no key kept.
let jji: HH = load("ji.json", partial: true)
`

// deepJSON are the partial sources, compact: each is put in canonical layout (FMT-02).
var deepJSON = map[string]string{
	"pa1.json": `{"h": {"zzz": 9, "n": 1, "k0": 1, "v0": 2, "m": 3}}`,
	"pa2.json": `{"h": {"n": 1, "k0": 1, "zzz": 9, "v0": 2, "m": 3}}`,
	"pa3.json": `{"h": {"n": 1, "k0": 1, "v0": 2, "m": 3, "zzz": 9}}`,
	"pa4.json": `{"h": {"k0": 1, "v0": 2, "k2": 9, "k01": 5, "v+1": 4}}`,
	"vs.json":  `{"vs": {"kind": "sl", "k0": 1, "zzz": 9, "v0": 2, "k3": 8, "lg": {"a": 1, "yyy": 7}}}`,
	"pp1.json": `{"h": {"lg": {"zzz": 9, "a": 3, "b": 4}, "n": 1}}`,
	"pp2.json": `{"h": {"lg": {"a": 3, "zzz": 9, "b": 4}, "n": 1}}`,
	"pp3.json": `{"h": {"lg": {"a": 3, "b": 4, "zzz": 9}, "n": 1}}`,
	"pp4.json": `{"h": {"lg": {"a": 3, "zzz": 9}, "n": 1}}`,
	"px1.json": `{"h": {"x": {"y": {"c": 1, "zzz": 9}, "d": 2}}}`,
	"px2.json": `{"h": {"x": {"y": {"c": 1}, "zzz": 9, "d": 2}}}`,
	"px3.json": `{"h": {"x": {"y": {"c": 1}, "d": 2}, "zzz": 9}}`,
	"pc.json":  `{"o": {"q": 3, "zzz": 9}}`,
	"ji.json":  `{"h": {"type": "circle", "R": 5, "z": 1}}`,
}

// jsonFS is package d holding src beside files, each put in canonical layout (FMT-02), over
// replacing some.
func jsonFS(t *testing.T, src string, files, over map[string]string) mapFS {
	t.Helper()
	fsys := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(src)}
	for name, text := range files { //canon:unordered fills a map
		if o, ok := over[name]; ok {
			text = o
		}
		fsys["law/d/"+name] = file(string(formatted(t, name, []byte(text))))
	}
	return fsys
}

// deepCase is an op on a partial source, the text its file then holds (compact), and the same
// file with no unknown key and the edited field as its default states it: what the file must
// decode to.
type deepCase struct {
	name string
	op   edit.Operation
	file string
	want string
	ref  string
}

// deepCases are TestE6PartialKeepsDeepUnknownKeys's rows.
var deepCases = []deepCase{
	// @json(pairs:): the object holding the slots.
	{"pairs, before: Reset", reset("jpa1.h"), "pa1.json", `{"h": {"zzz": 9, "m": 6}}`, `{}`},
	{"pairs, before: Set to the default", setTo("jpa1.h", "{ m: 6 }"), "pa1.json", `{"h": {"zzz": 9, "m": 6}}`, `{}`},
	{"pairs, between: Reset", reset("jpa2.h"), "pa2.json", `{"h": {"zzz": 9, "m": 6}}`, `{}`},
	{"pairs, after: Reset", reset("jpa3.h"), "pa3.json", `{"h": {"m": 6, "zzz": 9}}`, `{}`},
	{"pairs, after: Set equal to the default", setTo("jpa3.h", "{ n: 0, st: [], m: 6 }"), "pa3.json", `{"h": {"m": 6, "zzz": 9}}`, `{}`},
	{"pairs, keys no slot claims: Reset", reset("jpa4.h"), "pa4.json", `{"h": {"k2": 9, "k01": 5, "v+1": 4, "m": 6}}`, `{}`},
	{"pairs, the slots: Reset", reset("jpa2.h.st"), "pa2.json", `{"h": {"n": 1, "zzz": 9, "m": 3}}`, `{"h": {"n": 1, "m": 3}}`},
	// A case change: the old case's slot keys and path keys go, the unknown keys stay.
	{"case: Reset", reset("jvs.vs"), "vs.json", `{"vs": {"kind": "pl", "zzz": 9, "k3": 8, "lg": {"yyy": 7, "w": 4}}}`, `{}`},
	{"case: Set to the default", setTo("jvs.vs", "pl { w: 4 }"), "vs.json", `{"vs": {"kind": "pl", "zzz": 9, "k3": 8, "lg": {"yyy": 7, "w": 4}}}`, `{}`},
	{"case: Set, the old path object alone", setTo("jvs.vs", "pl {}"), "vs.json", `{"vs": {"kind": "pl", "zzz": 9, "k3": 8, "lg": {"yyy": 7}}}`, `{"vs": {"kind": "pl"}}`},
	{"case: Reset the path field", reset("jvs.vs.a"), "vs.json", `{"vs": {"kind": "sl", "k0": 1, "zzz": 9, "v0": 2, "k3": 8, "lg": {"yyy": 7}}}`, `{"vs": {"kind": "sl", "k0": 1, "v0": 2}}`},
	// @json(path:): the objects it creates.
	{"path, before: Reset", reset("jpp1.h"), "pp1.json", `{"h": {"lg": {"zzz": 9, "b": 7}}}`, `{}`},
	{"path, before: Set to the default", setTo("jpp1.h", "{ b: 7 }"), "pp1.json", `{"h": {"lg": {"zzz": 9, "b": 7}}}`, `{}`},
	{"path, between: Reset", reset("jpp2.h"), "pp2.json", `{"h": {"lg": {"zzz": 9, "b": 7}}}`, `{}`},
	{"path, after: Reset", reset("jpp3.h"), "pp3.json", `{"h": {"lg": {"b": 7, "zzz": 9}}}`, `{}`},
	{"path, after: Set equal to the default", setTo("jpp3.h", "{ a: 1, n: 0, b: 7 }"), "pp3.json", `{"h": {"lg": {"b": 7, "zzz": 9}}}`, `{}`},
	{"path, a new key beside a kept one: Reset", reset("jpp4.h"), "pp4.json", `{"h": {"lg": {"zzz": 9, "b": 7}}}`, `{}`},
	{"path, inner: Reset", reset("jpx1.h"), "px1.json", `{"h": {"x": {"y": {"zzz": 9}}}}`, `{}`},
	{"path, outer: Reset", reset("jpx2.h"), "px2.json", `{"h": {"x": {"zzz": 9}}}`, `{}`},
	{"path, outer: Set to the default", setTo("jpx2.h", "{}"), "px2.json", `{"h": {"x": {"zzz": 9}}}`, `{}`},
	{"path, beside: Reset", reset("jpx3.h"), "px3.json", `{"h": {"zzz": 9}}`, `{}`},
	{"path, emptied: Reset its fields", reset("jpx3.h.c"), "px3.json", `{"h": {"x": {"d": 2}, "zzz": 9}}`, `{"h": {"x": {"d": 2}}}`},
	// A plain object: the kept key before the key the default writes in its slot.
	{"plain, a new key beside a kept one: Reset", reset("jpc.o"), "pc.json", `{"o": {"zzz": 9, "r": 7}}`, `{}`},
	{"plain, a new key in a kept one's slot: Set", setTo("jpc.o.r", "8"), "pc.json", `{"o": {"q": 3, "zzz": 9, "r": 8}}`, `{"o": {"q": 3, "r": 8}}`},
	// DECISIONS 471 round 5: an inline variant one level down keeps its case tag.
	{"inline, one level down: Reset", reset("jji.h.sh"), "ji.json", `{"h": {"type": "square", "s": 4, "z": 1}}`, `{"h": {"z": 1, "type": "square", "s": 4}}`},
}

// DECISIONS 471; API.md M8, E6, E22, M9; FORMATTER.md 14.2: a Set to the default or a Reset keeps
// a `partial` object's unknown keys beside pairs slots and in path objects, in place; the file
// decodes to the field default, stays canonical, and its Undo restores.
func TestE6PartialKeepsDeepUnknownKeys(t *testing.T) {
	checkDeep(t, deepSrc, deepJSON, deepCases)
}

// checkDeep runs cases on the package src beside its JSON files: each file's text, what it
// decodes to, M9 on the next edit and both Undo rounds (API.md E22).
func checkDeep(t *testing.T, src string, files map[string]string, cases []deepCase) {
	t.Helper()
	for _, c := range cases {
		fsys := jsonFS(t, src, files, nil)
		edited, ok := applied(t, c.name, fsys, []edit.Operation{c.op})
		if !ok {
			continue
		}
		path := "law/d/" + c.file
		if got, want := string(edited.fsys[path].Data), string(formatted(t, c.file, []byte(c.want))); got != want {
			t.Errorf("%s: %s is\n%s\nwant\n%s", c.name, c.file, got, want)
		}
		ref := values(open(t, jsonFS(t, src, files, map[string]string{c.file: c.ref}), nil, "", "d"), false)
		got := values(open(t, edited.fsys, nil, "", "d"), false)
		if let := "d:" + letOf(c.op.Path); got[let] != ref[let] {
			t.Errorf("%s: %s decodes to\n%s\nwant\n%s", c.name, let, got[let], ref[let])
		}
		undoTwice(t, c.name, fsys, []edit.Operation{c.op})
	}
}

// letOf is the let a path starts at.
func letOf(path string) string {
	for i, r := range path {
		if r == '.' || r == '[' {
			return path[:i]
		}
	}
	return path
}

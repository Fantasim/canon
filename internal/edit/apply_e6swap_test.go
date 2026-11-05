package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// swapSrc holds an inline variant beside a record field, variants whose cases claim a key the
// other does not, and pairs slots beside a plain field declared before them.
const swapSrc = `package d

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

/// Holds an HP.
record HHP {
  /// The holder.
  h: HP = { m: 6 }
}

/// A plain part.
record Q {
  /// Q.
  q: Int = 1
  /// R.
  r: Int = 2
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

/// An inline shape beside a record field.
record HJQ {
  /// The shape.
  sh: Sh = square { s: 4 } @json(inline)
  /// O.
  o: Q = {}
  /// Z.
  z: Int = 0
}

/// Holds an HJQ.
record HHQ {
  /// The holder.
  h: HJQ = {}
}

/// A field declared before an inline shape.
record HA {
  /// A.
  a: Int = 0
  /// The shape.
  sh: Sh = square { s: 4 } @json(inline)
  /// Z.
  z: Int = 0
}

/// A path in one case, a leaf of its key in the other.
variant VL {
  /// Under a path.
  pa {
    /// A.
    a: Int = 0 @json(path: "lg.a")
  }
  /// A leaf.
  le {
    /// Lg.
    lg: Int = 0
  }
}

/// Holds a VL.
record HVL {
  /// The shape.
  vs: VL = le {}
}

/// Two cases, each claiming a key the other does not.
variant VW {
  /// One.
  sl {
    /// A.
    a: Int = 0
  }
  /// Two.
  pl {
    /// W.
    w: Int = 3
  }
}

/// Holds a VW.
record HVW {
  /// The shape.
  vs: VW = pl {}
}

/// Three cases: a key of the second is neither the first's nor the third's.
variant V3 {
  /// One.
  c1 {
    /// A.
    a: Int = 0
  }
  /// Two.
  c2 {
    /// B.
    b: Int = 0
  }
  /// Three.
  c3 {
    /// W.
    w: Int = 3
  }
}

/// Holds a V3.
record HV3 {
  /// The shape.
  vs: V3 = c3 {}
}

/// An inline shape beside a record holding a kept key.
let jq: HHQ = load("jq.json", partial: true)

/// A kept key before an inline tag.
let jt: HHQ = load("jt.json", partial: true)

/// No key kept.
let nq: HHQ = load("nq.json")

/// A kept key before a tag.
let jvm: HVW = load("vm.json", partial: true)

/// A path object against a leaf.
let jvl: HVL = load("vl.json", partial: true)

/// A key the other case claims.
let jvw: HVW = load("vw.json", partial: true)

/// Slots after a kept key.
let jp1: HHP = load("p1.json", partial: true)

/// Slots before a kept key.
let jp4: HHP = load("p4.json", partial: true)

/// Slots after a kept key, a key declared before them absent.
let jp2: HHP = load("p2.json", partial: true)

/// Slots, a key declared before them absent.
let np: HHP = load("np.json")

/// An inline shape, a key declared before it absent.
let na: HA = load("na.json")

/// A third case's key.
let jv3: HV3 = load("v3.json", partial: true)
`

// swapJSON are swapSrc's sources, compact.
var swapJSON = map[string]string{
	"jq.json": `{"h": {"type": "circle", "R": 5, "o": {"q": 3, "zzz": 9}, "z": 1}}`,
	"jt.json": `{"h": {"zzz": 9, "type": "circle", "R": 5, "o": {"q": 3}, "z": 1}}`,
	"nq.json": `{"h": {"type": "circle", "R": 5, "o": {"q": 3}, "z": 1}}`,
	"vm.json": `{"vs": {"zzz": 9, "kind": "sl", "a": 2}}`,
	"vl.json": `{"vs": {"kind": "pa", "lg": {"a": 1, "yyy": 7}}}`,
	"vw.json": `{"vs": {"kind": "sl", "a": 2, "w": 99}}`,
	"p1.json": `{"h": {"n": 1, "zzz": 9, "m": 3}}`,
	"p4.json": `{"h": {"k0": 1, "v0": 2, "zzz": 9, "m": 3}}`,
	"p2.json": `{"h": {"zzz": 9, "k0": 1, "v0": 2}}`,
	"np.json": `{"h": {"k0": 1, "v0": 2}}`,
	"na.json": `{"type": "circle", "R": 5, "z": 1}`,
	"v3.json": `{"vs": {"kind": "c1", "a": 1, "b": 7}}`,
}

// swapCases are TestE6PartialSwapKeepsSiblings's rows.
var swapCases = []deepCase{
	// An inline case changes member by member: the sibling record keeps its kept key.
	{"inline beside a record: Reset the case", reset("jq.h.sh"), "jq.json",
		`{"h": {"type": "square", "s": 4, "o": {"q": 3, "zzz": 9}, "z": 1}}`, `{"h": {"type": "square", "s": 4, "o": {"q": 3}, "z": 1}}`},
	{"inline beside a record: Set the case", setTo("jq.h.sh", "square { s: 9 }"), "jq.json",
		`{"h": {"type": "square", "s": 9, "o": {"q": 3, "zzz": 9}, "z": 1}}`, `{"h": {"type": "square", "s": 9, "o": {"q": 3}, "z": 1}}`},
	{"inline beside a record: Reset", reset("jq.h"), "jq.json", `{"h": {"type": "square", "s": 4, "o": {"zzz": 9}}}`, `{}`},
	{"inline, no key kept: Reset the case", reset("nq.h.sh"), "nq.json",
		`{"h": {"type": "square", "s": 4, "o": {"q": 3}, "z": 1}}`, `{"h": {"type": "square", "s": 4, "o": {"q": 3}, "z": 1}}`},
	// Ruling of 2026-11-05 (471's tie rule): a kept key before the old tag stays before the new one.
	{"inline, kept before the tag: Reset the case", reset("jt.h.sh"), "jt.json",
		`{"h": {"zzz": 9, "type": "square", "s": 4, "o": {"q": 3}, "z": 1}}`, `{"h": {"type": "square", "s": 4, "o": {"q": 3}, "z": 1}}`},
	{"case, kept before the tag: Reset", reset("jvm.vs"), "vm.json", `{"vs": {"zzz": 9, "kind": "pl"}}`, `{}`},
	// FORMATTER.md 14.2: a slot inserted beside a kept key comes after it.
	{"pairs, a slot after a kept key", setTo("jp1.h.st", "[{ k: 1, v: 2 }]"), "p1.json",
		`{"h": {"n": 1, "zzz": 9, "k0": 1, "v0": 2, "m": 3}}`, `{"h": {"n": 1, "k0": 1, "v0": 2, "m": 3}}`},
	{"pairs, a slot after the slots and a kept key", setTo("jp4.h.st", "[{ k: 1, v: 2 }, { k: 3, v: 4 }]"), "p4.json",
		`{"h": {"k0": 1, "v0": 2, "zzz": 9, "k1": 3, "v1": 4, "m": 3}}`, `{"h": {"k0": 1, "v0": 2, "k1": 3, "v1": 4, "m": 3}}`},
	// FORMATTER.md 14.2: declaration order counts slot keys and an inline case's keys.
	{"pairs, a key declared before the slots, kept first", setTo("jp2.h.n", "5"), "p2.json",
		`{"h": {"zzz": 9, "n": 5, "k0": 1, "v0": 2}}`, `{"h": {"n": 5, "k0": 1, "v0": 2}}`},
	{"pairs, a key declared before the slots", setTo("np.h.n", "5"), "np.json", `{"h": {"n": 5, "k0": 1, "v0": 2}}`, `{"h": {"n": 5, "k0": 1, "v0": 2}}`},
	{"inline, a key declared before the case", setTo("na.a", "2"), "na.json", `{"a": 2, "type": "circle", "R": 5, "z": 1}`, `{"a": 2, "type": "circle", "R": 5, "z": 1}`},
	// Ruling of 2026-11-05: a key of a case neither old nor new is unknown to the object: kept.
	{"case, a third case's key: Reset", reset("jv3.vs"), "v3.json", `{"vs": {"kind": "c3", "b": 7}}`, `{}`},
}

// thirdCases are the rows on partialSrc: an inline variant's third case's key kept by a case change.
var thirdCases = []deepCase{
	{"inline, a third case's key: Reset", reset("jo.o"), "hn.json", `{"o": {"kind": "ob", "q": 7, "zzz": 9}}`, `{}`},
}

// DECISIONS 471; API.md M8, E6, E22; FORMATTER.md 14.2: an inline case changes member by member, so
// a sibling field keeps its kept keys; a kept key before a tag stays first; a new slot or key is
// placed in declaration order, slot and inline keys counted, after the kept keys of its slot.
func TestE6PartialSwapKeepsSiblings(t *testing.T) {
	checkDeep(t, swapSrc, swapJSON, swapCases)
	checkDeep(t, partialSrc, partialJSON, thirdCases)
}

// swapRefusals are TestE6PartialSwapRefused's rows: an op, and the package and files it is
// applied to.
var swapRefusals = []struct {
	name  string
	op    edit.Operation
	src   string
	files map[string]string
}{
	{"a path object against a leaf: Reset", reset("jvl.vs"), swapSrc, swapJSON},
	{"a key the new case claims: Reset", reset("jvw.vs"), swapSrc, swapJSON},
	{"a key the new case claims: Set", setTo("jvw.vs", "pl { w: 5 }"), swapSrc, swapJSON},
	{"a third case's key, the new case's: Set", setTo("jv3.vs", "c2 {}"), swapSrc, swapJSON},
	{"a key the new inline case claims: Set", setTo("jo.o.inner", "iy {}"), partialSrc, partialJSON},
}

// DECISIONS 471 (rulings of 2026-11-05): a case change whose default cannot be written beside the
// kept keys is refused `computed`: an old path object keeping a key where the new case holds a
// value, or a key the old case did not claim and the new one does, which would decode as it.
func TestE6PartialSwapRefused(t *testing.T) {
	for _, c := range swapRefusals {
		fsys := jsonFS(t, c.src, c.files, nil)
		s := open(t, fsys, nil, "", "d")
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
		var ne *edit.NotEditableError
		if !errors.As(err, &ne) || ne.Reason != edit.ReasonComputed {
			t.Errorf("%s: %v, want refused computed", c.name, err)
		}
	}
}

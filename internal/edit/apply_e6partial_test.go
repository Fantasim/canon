package edit_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// partialSrc holds records that loads with `partial` read: a `{}` default, a default that is not
// the record's own (flat and one level down), a record with no field, and a dependent field.
const partialSrc = `package d

/// A part.
record Q {
  /// Q.
  q: Int = 1
  /// R.
  r: Int = 2
}

/// Its part left to {}.
record O {
  /// The part.
  o: Q = {}
}

/// Its part's default is not Q's own.
record O2 {
  /// The part.
  o: Q = { q: 5 }
  /// N.
  n: Int = 0
}

/// One level down.
record N2 {
  /// The holder.
  m: O2 = { o: { q: 7 } }
  /// K.
  k: Int = 0
}

/// No field.
record E {}

/// Holds an E.
record OE {
  /// The empty.
  e: E = {}
  /// N.
  n: Int = 0
}

/// A kind.
enum K { a, b }

/// A, by kind.
record RA {
  /// V.
  v: Int = 1
}

/// What a kind holds.
type Pick(k: K) = match k {
  a => RA
  b => Int
}

/// A dependent field.
record OT {
  /// Its kind.
  k: K = a
  /// What it holds.
  t: Pick(k) = {}
}

/// Holds an OT.
record HT {
  /// The holder.
  h: OT = {}
}

/// A shape.
variant Shape {
  /// A circle.
  circle {
    /// R.
    r: Int = 1
  }
  /// A square.
  square {
    /// S.
    s: Int = 2
  }
}

/// A shape held whole.
record HV {
  /// The shape.
  sh: Shape = square {}
  /// N.
  n: Int = 0
}

/// A shape held inline.
record HI {
  /// The shape.
  sh: Shape = square {} @json(inline)
  /// Z.
  z: Int
}

/// An inner variant, its own tag.
variant In @json(tag: "itag") {
  /// X.
  ix {
    /// P.
    p: Int = 1
  }
  /// Y.
  iy {
    /// Q.
    q: Int = 2
  }
}

/// An outer variant whose case holds In inline.
variant Out {
  /// A.
  oa {
    /// Inner.
    inner: In = iy {} @json(inline)
  }
  /// B.
  ob {
    /// K.
    k: Int = 0
  }
}

/// Holds an Out.
record HN {
  /// The outer.
  o: Out = ob {}
}

/// A shape tagged type, a field renamed.
variant Sh2 @json(tag: "type") {
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

/// A shape held whole, its default not the case's own.
record HM {
  /// The shape.
  sh: Sh2 = square { s: 4 }
}

/// A shape held inline.
record HJ {
  /// The shape.
  sh: Sh2 = square { s: 4 } @json(inline)
  /// Z.
  z: Int
}

/// Holds an HJ.
record HH {
  /// The holder.
  h: HJ
}

/// Each, partly known.
let jp: O = load("o.json", partial: true)

/// Flat.
let jq: O2 = load("q.json", partial: true)

/// Nested.
let jn: N2 = load("n.json", partial: true)

/// Empty.
let je: OE = load("e.json", partial: true)

/// Dependent.
let jt: HT = load("t.json", partial: true)

/// A shape.
let jv: HV = load("v.json", partial: true)

/// A shape inline.
let ji: HI = load("i.json", partial: true)

/// A case holding an inline variant.
let jo: HN = load("hn.json", partial: true)

/// A kept key first.
let jm: HM = load("m.json", partial: true)

/// A kept key between, inline.
let jj: HH = load("j.json", partial: true)
`

// The partial sources, in canonical layout (FMT-02), each holding the unknown key zzz.
var partialJSON = map[string]string{
	"o.json":  "{\n  \"o\": {\n    \"q\": 3,\n    \"zzz\": 9\n  }\n}\n",
	"q.json":  "{\n  \"o\": {\n    \"q\": 3,\n    \"zzz\": 9\n  },\n  \"n\": 1\n}\n",
	"n.json":  "{\n  \"m\": {\n    \"o\": {\n      \"q\": 3,\n      \"zzz\": 9\n    },\n    \"n\": 2\n  },\n  \"k\": 1\n}\n",
	"v.json":  "{\n  \"sh\": {\n    \"kind\": \"circle\",\n    \"r\": 5,\n    \"zzz\": 9\n  },\n  \"n\": 1\n}\n",
	"i.json":  "{\n  \"kind\": \"circle\",\n  \"r\": 5,\n  \"zzz\": 9,\n  \"z\": 1\n}\n",
	"hn.json": "{\n  \"o\": {\n    \"kind\": \"oa\",\n    \"itag\": \"ix\",\n    \"p\": 3,\n    \"q\": 7,\n    \"zzz\": 9\n  }\n}\n",
	"m.json":  "{\n  \"sh\": {\n    \"zzz\": 9,\n    \"type\": \"circle\",\n    \"R\": 5\n  }\n}\n",
	"j.json":  "{\n  \"h\": {\n    \"type\": \"circle\",\n    \"R\": 5,\n    \"zzz\": 9,\n    \"z\": 1\n  }\n}\n",
	"e.json":  "{\n  \"e\": {\n    \"zzz\": 9\n  },\n  \"n\": 1\n}\n",
	"t.json":  "{\n  \"h\": {\n    \"t\": {\n      \"v\": 3,\n      \"zzz\": 9\n    }\n  }\n}\n",
}

func partialFS() mapFS {
	fsys := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(partialSrc)}
	for name, text := range partialJSON { //canon:unordered fills a map
		fsys["law/d/"+name] = file(text)
	}
	return fsys
}

// partialCase is an op on a partial source, the text its file then holds, and the text its Undo
// brings back beside zzz.
type partialCase struct {
	name string
	op   edit.Operation
	file string
	want []string
	back string
}

// reset is a Reset of path.
func reset(path string) edit.Operation { return edit.Operation{Kind: edit.OpReset, Path: path} }

// partialCases are TestE6PartialKeepsUnknownKeys's rows.
var partialCases = []partialCase{
	{"{}: Set to {}", setTo("jp.o", "{}"), "o.json", []string{"\"o\": {\n    \"zzz\": 9\n  }"}, `"q": 3`},
	{"{}: Set equal to the default", setTo("jp.o", "{ q: 1 }"), "o.json", []string{"\"o\": {\n    \"zzz\": 9\n  }"}, `"q": 3`},
	{"{}: Reset", reset("jp.o"), "o.json", []string{"\"o\": {\n    \"zzz\": 9\n  }"}, `"q": 3`},
	{"flat: Set to the default", setTo("jq.o", "{ q: 5 }"), "q.json", []string{"\"q\": 5,\n    \"zzz\": 9"}, `"q": 3`},
	{"flat: Set equal to the default", setTo("jq.o", "{ q: 5, r: 2 }"), "q.json", []string{"\"q\": 5,\n    \"zzz\": 9"}, `"q": 3`},
	{"flat: Reset", reset("jq.o"), "q.json", []string{"\"q\": 5,\n    \"zzz\": 9"}, `"q": 3`},
	{"flat: whole-record Set", setTo("jq", "{ n: 1 }"), "q.json", []string{"\"q\": 5,\n    \"zzz\": 9", "\"n\": 1"}, `"q": 3`},
	{"nested: Set to the default", setTo("jn.m", "{ o: { q: 7 } }"), "n.json", []string{"\"q\": 7,\n      \"zzz\": 9"}, `"q": 3`},
	{"nested: Reset", reset("jn.m"), "n.json", []string{"\"q\": 7,\n      \"zzz\": 9"}, `"n": 2`},
	{"nested: whole-record Set", setTo("jn", "{ k: 1 }"), "n.json", []string{"\"q\": 7,\n      \"zzz\": 9", "\"k\": 1"}, `"q": 3`},
	{"no field: Reset", reset("je.e"), "e.json", []string{"\"e\": {\n    \"zzz\": 9\n  }"}, `"zzz": 9`},
	{"case: Reset", reset("jv.sh"), "v.json", []string{"\"sh\": {\n    \"kind\": \"square\",\n    \"zzz\": 9\n  }"}, `"r": 5`},
	{"case: Set to the default", setTo("jv.sh", "square {}"), "v.json", []string{"\"sh\": {\n    \"kind\": \"square\",\n    \"zzz\": 9\n  }"}, `"r": 5`},
	{"inline case: Reset", reset("ji.sh"), "i.json", []string{"{\n  \"kind\": \"square\",\n  \"zzz\": 9,\n  \"z\": 1\n}"}, `"r": 5`},
	{"inline case: Set to the default", setTo("ji.sh", "square {}"), "i.json", []string{"{\n  \"kind\": \"square\",\n  \"zzz\": 9,\n  \"z\": 1\n}"}, `"r": 5`},
	{"inline in a case: Set", setTo("jo.o", "oa { inner: ix { p: 4 } }"), "hn.json", []string{"\"kind\": \"oa\",\n    \"itag\": \"ix\",\n    \"p\": 4"}, `"p": 3`},
	{"inline in a case: Set the inline field", setTo("jo.o.inner", "iy {}"), "hn.json", []string{"\"kind\": \"oa\",\n    \"itag\": \"iy\""}, `"p": 3`},
	{"inline in a case: Reset", reset("jo.o"), "hn.json", []string{"\"kind\": \"ob\""}, `"p": 3`},
	{"in place, first: Reset", reset("jm.sh"), "m.json", []string{"{\n    \"zzz\": 9,\n    \"type\": \"square\",\n    \"s\": 4\n  }"}, `"R": 5`},
	{"in place, inline: Reset", reset("jj.h.sh"), "j.json", []string{"{\n    \"type\": \"square\",\n    \"zzz\": 9,\n    \"s\": 4,\n    \"z\": 1\n  }"}, `"R": 5`},
	{"dependent: Reset", reset("jt.h.t"), "t.json", []string{"\"t\": {\n      \"zzz\": 9\n    }"}, `"v": 3`},
}

// API.md M8, E6, W9, E22 (EDIT-E6RECORD rulings): a Set to the default, a Set equal to it, a Reset
// and a whole-record Set of a JSON object holding a key `partial` kept keep that key and write
// beside it what the field's default holds and the object would not decode to; the Undo restores.
func TestE6PartialKeepsUnknownKeys(t *testing.T) {
	for _, c := range partialCases {
		edited, ok := applied(t, c.name, partialFS(), []edit.Operation{c.op})
		if !ok {
			continue
		}
		got := string(edited.fsys["law/d/"+c.file].Data)
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: %s is\n%s\nwant it to hold\n%s", c.name, c.file, got, w)
			}
		}
		undoTwice(t, c.name, partialFS(), []edit.Operation{c.op})
		back, ok := applied(t, c.name+", Undo", edited.fsys, edited.plan.Undo)
		if !ok {
			continue
		}
		got = string(back.fsys["law/d/"+c.file].Data)
		if !strings.Contains(got, `"zzz": 9`) || !strings.Contains(got, c.back) || exactUndo[c.name] && got != partialJSON[c.file] {
			t.Errorf("%s, Undo: %s is\n%s", c.name, c.file, got)
		}
	}
}

// exactUndo are the rows whose Undo gives the file back byte for byte: a kept key before every
// member the edit rewrites comes back where it was (M8, FORMATTER.md 14.1).
var exactUndo = map[string]bool{"in place, first: Reset": true}

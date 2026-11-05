package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/value"
)

// tappSrc holds records whose field is typed by a type function applied to a value (TYPES.md
// 11.2, 11.4): by an earlier field, by one whose name sorts after it, by a parameter of the
// applied record (TYPES.md 11.1), and by a field selecting Never.
const tappSrc = `package d

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

/// What a kind holds, or nothing.
type PickN(k: K) = match k {
  a => RA
  b => Never
}

/// A dependent field, driven by an earlier field.
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

/// A dependent field, driven by an earlier field whose name sorts after it.
record OZ {
  /// Its kind.
  z: K = b
  /// What it holds.
  t: Pick(z) = 0
}

/// A dependent field that may compute Never.
record ON {
  /// Its kind.
  k: K = a
  /// What it holds.
  t: PickN(k)?
}

/// A dependent field, driven by a parameter.
record PT(k: K) {
  /// What it holds.
  t: Pick(k) = {}
}

/// Holds a PT.
record HP {
  /// Its kind.
  k: K = a
  /// The holder.
  p: PT(k) = {}
}

/// Written in Canon.
let ct: HT = { h: { t: { v: 3 } } }

/// Written in Canon, a typed literal.
let cr: HT = { h: { t: RA { v: 3 } } }

/// Written in Canon, by a parameter.
let cp: HP = { p: { t: { v: 3 } } }

/// Written in Canon, its driver sorting after.
let cz: OZ = {}

/// Written in Canon, Never by another kind.
let cn: ON = {}

/// Loaded.
let jt: HT = load("t.json")

/// Loaded, by a parameter.
let jp: HP = load("p.json")
`

func tappFS() mapFS {
	return mapFS{
		"law/project.canon": file(projectCanon), "law/d/d.canon": file(tappSrc),
		"law/d/t.json": file("{\n  \"h\": {\n    \"t\": {\n      \"v\": 3\n    }\n  }\n}\n"),
		"law/d/p.json": file("{\n  \"p\": {\n    \"t\": {\n      \"v\": 3\n    }\n  }\n}\n"),
	}
}

// API.md V1, E14, E22, E23; TYPES.md 11.4, 11.6: a record nested in a Set's value or a Reset's
// inverse takes the branch its record computes (earlier fields, defaults, bound parameters); the
// op checks clean and its Undo, then that Undo's Undo, give back the values (E22).
func TestTypeAppRecordFieldUndo(t *testing.T) {
	for _, c := range []struct {
		name string
		op   edit.Operation
	}{
		{"Canon: Reset the holder", reset("ct.h")},
		{"Canon: Reset the field", reset("ct.h.t")},
		{"Canon: Set the holder", setTo("ct.h", "{ t: { v: 5 } }")},
		{"Canon: Set the holder, driver given", setTo("ct.h", "{ k: b, t: 7 }")},
		{"Canon: Set the holder, driver given after", setTo("ct.h", "{ t: { v: 5 }, k: a }")},
		{"Canon: Set the holder, Obj", setAt("ct.h", edit.Obj{"t": edit.Obj{"v": edit.Int(5)}})},
		{"Canon: Set the holder, Obj, driver given", setAt("ct.h", edit.Obj{"k": edit.Member("a"), "t": edit.Obj{"v": edit.Int(5)}})},
		{"Canon: Set the field", setTo("ct.h.t", "{ v: 5 }")},
		{"Canon: Set the root", setTo("ct", "{ h: { t: { v: 5 } } }")},
		{"Canon: Set the field's member", setAt("ct.h.t.v", edit.Int(5))},
		{"Canon, typed literal: Reset the holder", reset("cr.h")},
		{"Canon, typed literal: Set the holder", setTo("cr.h", "{ t: { v: 5 } }")},
		{"Canon, typed literal: Set its member", setAt("cr.h.t.v", edit.Int(5))},
		{"Canon, by a parameter: Reset the holder", reset("cp.p")},
		{"Canon, by a parameter: Set the holder", setTo("cp.p", "{ t: { v: 5 } }")},
		{"Canon, by a parameter: Set the field", setTo("cp.p.t", "{ v: 5 }")},
		{"Canon, by a parameter: Set the root", setTo("cp", "{ p: { t: { v: 5 } } }")},
		{"Canon, by a parameter: Set the root, driver given", setTo("cp", "{ k: b, p: { t: 7 } }")},
		{"Canon, driver sorting after: Set the root, Obj", setAt("cz", edit.Obj{"z": edit.Member("a"), "t": edit.Obj{"v": edit.Int(5)}})},
		{"Canon, driver sorting after: Set the root", setTo("cz", "{ t: { v: 5 }, z: a }")},
		{"Canon, Never by another kind: Set the root", setTo("cn", "{ t: { v: 5 } }")},
		{"JSON: Reset the holder", reset("jt.h")},
		{"JSON: Reset the field", reset("jt.h.t")},
		{"JSON: Set the holder", setTo("jt.h", "{ t: { v: 5 } }")},
		{"JSON: Set the holder, Obj", setAt("jt.h", edit.Obj{"t": edit.Obj{"v": edit.Int(5)}})},
		{"JSON: Set the field", setTo("jt.h.t", "{ v: 5 }")},
		{"JSON, by a parameter: Reset the holder", reset("jp.p")},
		{"JSON, by a parameter: Set the holder", setTo("jp.p", "{ t: { v: 5 } }")},
		{"JSON, by a parameter: Set the root, driver given", setTo("jp", "{ k: b, p: { t: 7 } }")},
	} {
		checkClean(t, c.name, tappFS(), []edit.Operation{c.op})
		undoTwice(t, c.name, tappFS(), []edit.Operation{c.op})
	}
}

// API.md V1 (8.2 Source; TYPES.md 11.4, 11.6): a record literal is refused where the branch its
// record computes takes none (an Int, a Never), and a typed record literal names a type, which is
// not a contextual name (litsource_test's `Part { … }`).
func TestTypeAppRecordFieldRefused(t *testing.T) {
	for _, c := range []struct {
		name string
		op   edit.Operation
	}{
		{"Int branch, Source", setTo("ct.h", "{ k: b, t: { v: 5 } }")},
		{"Int branch, Obj", setAt("ct.h", edit.Obj{"k": edit.Member("b"), "t": edit.Obj{"v": edit.Int(5)}})},
		{"Int branch by the default, driver sorting after", setAt("cz", edit.Obj{"t": edit.Obj{"v": edit.Int(5)}})},
		{"Int branch by a parameter", setTo("cp", "{ k: b, p: { t: { v: 5 } } }")},
		{"Never branch, Source", setTo("cn", "{ k: b, t: { v: 5 } }")},
		{"Never branch, Obj", setAt("cn", edit.Obj{"k": edit.Member("b"), "t": edit.Obj{"v": edit.Int(5)}})},
		{"typed record literal", setTo("ct.h", "{ t: RA { v: 5 } }")},
		{"typed record literal, the field", setTo("ct.h.t", "RA { v: 5 }")},
	} {
		s := open(t, tappFS(), nil, "", "d")
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
		if !errors.Is(err, edit.ErrBadValue) {
			t.Errorf("%s: %v, want ErrBadValue", c.name, err)
		}
	}
}

// API.md V1 (DECISIONS 171): with no applier a left-out driver has no default: a given driver's
// branch types the literal, else a Source takes its form's first branch and an Obj is refused;
// a computed Never takes neither.
func TestTypeAppRecordFieldTyper(t *testing.T) {
	p, err := build.Open(tappFS(), "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	f := analyze(t, p, []string{"d"}, true)
	for _, c := range []struct {
		path string
		lit  edit.Lit
		want string
	}{
		{"d:ct.h", edit.Obj{"k": edit.Member("a"), "t": edit.Obj{"v": edit.Int(5)}}, "RA{v: 5}"},
		{"d:ct.h", edit.Obj{"t": edit.Obj{"v": edit.Int(5)}}, ""},
		{"d:ct.h", edit.Source("{ t: { v: 5 } }"), "RA{v: 5}"},
		{"d:ct.h", edit.Source("{ k: b, t: 7 }"), "7"},
		{"d:ct.h", edit.Source("{ k: b, t: { v: 5 } }"), ""},
		{"d:ct.h", edit.Obj{"k": edit.Member("b"), "t": edit.Obj{"v": edit.Int(5)}}, ""},
		{"d:cn", edit.Source("{ k: b, t: { v: 5 } }"), ""},
		{"d:cn", edit.Obj{"k": edit.Member("b"), "t": edit.Obj{"v": edit.Int(5)}}, ""},
	} {
		typ, err := f.Type(resolve(t, f, c.path))
		if err != nil {
			t.Fatal(err)
		}
		v, err := edit.Typer{Host: noHost{}}.Value(context.Background(), c.lit, typ)
		switch {
		case c.want == "" && !errors.Is(err, edit.ErrBadValue):
			t.Errorf("%s %#v: got %v, %v, want ErrBadValue", c.path, c.lit, v, err)
		case c.want != "" && err != nil:
			t.Errorf("%s %#v: %v", c.path, c.lit, err)
		case c.want != "" && fieldT(v) != c.want:
			t.Errorf("%s %#v: t = %s, want %s", c.path, c.lit, fieldT(v), c.want)
		}
	}
}

// fieldT is the text of field t of record v (OT's and ON's second field), "" when it holds none.
func fieldT(v value.Value) string {
	rec, ok := v.(*value.Record)
	if !ok || len(rec.Fields) < 2 || rec.Fields[1] == nil {
		return ""
	}
	return rec.Fields[1].CanonText()
}

package edit_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
)

// GRAMMAR.md LEX-08, API.md V1: Source `T.n`, `p.T.n` or `T.case { … }` is member or case n of
// the enum or variant T expected (an enum, its literal union, a map key, a variant); a qualifier
// naming anything else, any other selector, a member given to a ref is ErrBadValue.
func TestTypedQualifiedMember(t *testing.T) {
	checkTyped(t, []typeCase{
		{"c", edit.Source("Color.green"), "green"},
		{"c", edit.Source("v.Color.red"), "red"},
		{"tag", edit.Source("Color.green"), "green"},
		{"byC", edit.Source("{ Color.green: 2 }"), "{green: 2}"},
		{"sh", edit.Source("Shape.dot"), "dot"},
		{"sh", edit.Source("v.Shape.circle { r: 2 }"), "circle{r: 2, label: }"},
		{"bc", edit.Source("Color.red"), ""},              // a ref takes a key, not a member
		{"c", edit.Source("w.Color.red"), ""},             // another package
		{"c", edit.Source("Shape.red"), ""},               // another type
		{"c", edit.Source("Color.blue"), ""},              // no such member
		{"c", edit.Source("Color?.red"), ""},              // not a plain selector
		{"c", edit.Source("(Color).red"), ""},             // not a name
		{"c", edit.Source("Color.red.code"), ""},          // a member's member
		{"s", edit.Source("Color.red"), ""},               // no enum expected
		{"sh", edit.Source("Color.circle { r: 2 }"), ""},  // another type
		{"sh", edit.Source("Shape.square { r: 2 }"), ""},  // no such case
		{"part", edit.Source(`v.Part { name: "q" }`), ""}, // a record type is not qualified
	})
}

// recheckCase is an edit golden whose files after the edit are analyzed again: want are the codes
// of the errors expected, in report order.
type recheckCase struct {
	golden string
	want   []diag.Code
}

// GRAMMAR.md LEX-08, API.md V2: what the reserved-word goldens write checks clean once analyzed
// again; what has no legal form is a catalogue finding of the re-check (E2102, E8102), never ErrInternal.
func TestReservedMemberRecheck(t *testing.T) {
	cases := []recheckCase{
		{"reserved_member_json", nil},
		{"reserved_member_source", nil},
		{"reserved_member_import", nil},
		{"reserved_member_layer", nil},
		{"reserved_ref_key", nil},
		{"reserved_case", nil},
		{"reserved_rename_key", nil},
		{"reserved_member_unbound", []diag.Code{diag.E2102.Def().Code}},
		{"reserved_member_entry_file", []diag.Code{diag.E2102.Def().Code}},
		{"reserved_member_path", nil},
		{"reserved_dependent", nil},
		{"reserved_dependent_json", nil},
		{"reserved_dependent_branches", nil},
		{"reserved_dependent_branches_json", nil},
		{"reserved_ref_enum_key", nil},
		{"reserved_ref_enum_key_reset", nil},
		{"reserved_rename_map_key", nil},
		{"bits_repeated_canon", []diag.Code{diag.E3206.Def().Code, diag.E8102.Def().Code, diag.E8102.Def().Code}},
	}
	for _, c := range cases {
		t.Run(c.golden, func(t *testing.T) { recheck(t, c) })
	}
}

func recheck(t *testing.T, c recheckCase) {
	ar, err := txtar.ParseFile(filepath.Join("testdata", "edits", c.golden+".txtar"))
	if err != nil {
		t.Fatal(err)
	}
	gc := readCase(t, ar)
	if n := open(t, gc.fsys, gc.layers, gc.editLayer, gc.pkgs...).a.Result().Summary.Errors; n > 0 {
		t.Fatalf("the files before have %d errors", n)
	}
	plan := applyGolden(t, gc)
	var got []diag.Code
	for _, f := range open(t, written(gc.fsys, plan), gc.layers, gc.editLayer, gc.pkgs...).a.Result().List {
		if f.Severity == diag.Error {
			got = append(got, f.Code)
		}
	}
	if !slices.Equal(got, c.want) {
		t.Errorf("after the edit: errors %v, want %v", got, c.want)
	}
}

// applyGolden is the plan of a golden's edit.
func applyGolden(t *testing.T, gc goldenCase) *edit.Plan {
	t.Helper()
	var req struct {
		Ops []edit.Operation `json:"ops"`
	}
	if err := json.Unmarshal(gc.edit, &req); err != nil {
		t.Fatal(err)
	}
	s := open(t, gc.fsys, gc.layers, gc.editLayer, gc.pkgs...)
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: req.Ops})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

// reservedLaw declares an enum whose members include reserved words, and a value holding one.
var reservedLaw = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/g/g.canon": file(`package g

/// A word.
enum Word { plain, if }

/// Another word.
enum Other { if }

/// A row.
record Row {
  /// A word.
  word: Word = plain
  /// Words.
  words: [Word] = []
}

/// A row.
let row: Row = { word: Word.if }
`),
}

// GRAMMAR.md LEX-08, API.md V1: a reserved word given bare, or through another type, is ErrBadValue.
func TestReservedMemberRefused(t *testing.T) {
	s := open(t, reservedLaw, nil, "", "g")
	for _, c := range []struct{ path, src string }{
		{"row.word", "if"}, {"row.words", "[if]"}, {"row.word", "Other.if"}, {"row.word", "h.Word.if"}, {"row.word", "Word.while"},
	} {
		op := edit.Operation{Kind: edit.OpSet, Path: c.path, Value: edit.Source(c.src)}
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
		if !errors.Is(err, edit.ErrBadValue) || errors.Is(err, edit.ErrInternal) {
			t.Errorf("Set %s %s: %v, want ErrBadValue", c.path, c.src, err)
		}
	}
}

// GRAMMAR.md LEX-08, API.md E26: a Member or Case its type must qualify has no Source text.
func TestReservedMemberSourceText(t *testing.T) {
	for _, c := range []struct {
		lit  edit.Lit
		want error
	}{
		{edit.Member("if"), edit.ErrNoText}, {edit.Member("return"), edit.ErrNoText},
		{edit.Member("check"), nil}, {edit.Member("plain"), nil},
		{edit.Case{Name: "while"}, edit.ErrNoText}, {edit.Case{Name: "dot"}, nil},
	} {
		_, err := edit.Operation{Kind: edit.OpSet, Path: "row.word", Value: c.lit}.MarshalJSON()
		if !errors.Is(err, c.want) {
			t.Errorf("%#v: %v, want %v", c.lit, err, c.want)
		}
	}
}

// noBranchLaw has a dependent field whose discriminant is required, so a record lacking it
// computes no branch.
var noBranchLaw = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/p/p.canon": file(`package p

/// A kind.
enum Kind { a, b }

/// Words of a.
enum WA { x, if }

/// Words of b.
enum WB { y, if }

/// A param.
type Param(k: Kind) = match k {
  a => WA
  b => WB
}

/// An event.
record Ev {
  /// Its kind.
  kind: Kind
  /// Its param.
  param: Param(kind)?
}

/// Events.
let evs: table Ev = {
  ea { kind: a, param: x }
}
`),
}

// DECISIONS 366, API.md V1: where no branch can be computed, a symbol no bare name holds is
// ErrBadValue, never ErrInternal; a nameable one is kept for the re-check.
func TestReservedSymbolNoBranch(t *testing.T) {
	s := open(t, noBranchLaw, nil, "", "p")
	cases := []struct {
		value edit.Lit
		want  error
	}{
		{edit.Source("{ param: WB.if }"), edit.ErrBadValue},
		{edit.Obj{"param": edit.Member("if")}, edit.ErrBadValue},
		{edit.Source("{ param: y }"), nil},
	}
	for _, c := range cases {
		op := edit.Operation{Kind: edit.OpAddEntry, Path: "evs", Key: edit.Key("eb"), Value: c.value}
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
		if !errors.Is(err, c.want) || errors.Is(err, edit.ErrInternal) {
			t.Errorf("AddEntry %#v: %v, want %v", c.value, err, c.want)
		}
	}
}

// foreignLayerLaw has a layer file, which has no imports, over a value holding another package's enum.
var foreignLayerLaw = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/v/v.canon":     file("package v\n\n/// A word.\nenum Word { plain, if }\n"),
	"law/p/p.canon": file(`package p

import v

/// A local word.
enum Loc { plain, return }

/// An item.
record Item {
  /// Its word.
  word: v.Word = plain
  /// Its local word.
  loc: Loc = plain
}

/// Conf.
let conf: Item = {}
`),
	"law/p/dev.layer.canon": file("package p\nlayer dev\n\namend conf {\n  loc: plain\n}\n"),
}

// DECISIONS 366 (known limit), API.md W11: a layer file names its own package's member `Loc.return`
// and another package's by its last segment, `v.Word.if`, which the re-check reports (E2102).
func TestReservedMemberLayerForeign(t *testing.T) {
	layers := []string{"dev"}
	s := open(t, foreignLayerLaw, layers, "dev", "p")
	ops := []edit.Operation{
		{Kind: edit.OpSet, Path: "conf.loc", Value: edit.Member("return")},
		{Kind: edit.OpSet, Path: "conf.word", Value: edit.FromJSON(`"if"`)},
	}
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
	if err != nil {
		t.Fatal(err)
	}
	after := written(foreignLayerLaw, plan)
	text := string(after["law/p/dev.layer.canon"].Data)
	if !strings.Contains(text, "loc: Loc.return") || !strings.Contains(text, "word: v.Word.if") {
		t.Errorf("layer file:\n%s", text)
	}
	var got []diag.Code
	for _, f := range open(t, after, layers, "dev", "p").a.Result().List {
		if f.Severity == diag.Error {
			got = append(got, f.Code)
		}
	}
	if want := []diag.Code{diag.E2102.Def().Code}; !slices.Equal(got, want) {
		t.Errorf("after the edit: errors %v, want %v", got, want)
	}
}

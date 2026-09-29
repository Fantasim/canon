package edit_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// defaultsP has a default read from a required field (TYP-15).
var defaultsP = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/p/p.canon":     file("package p\n\n/// R.\nrecord R {\n  /// A.\n  a: Int\n  /// B.\n  b: Int = a + 1\n}\n\n/// X.\nlet x: R = { a: 1 }\n"),
}

// log-2026-09-29 M4 U4a round 3, API.md V3: a FromJSON value leaving out a field that a default
// reads (`b: Int = a + 1` with `{}`) fails cleanly as a ValueError, with the host of a real
// evaluator: the default is never run on a field wire.Decoder.Keep left nil.
func TestDefaultReadingKeptNil(t *testing.T) {
	s := open(t, defaultsP, nil, "", "p")
	rt := s.a.Program().Packages[0].Decls[0].Type()
	for _, d := range s.a.Program().Packages[0].Decls {
		if d.Name() == "R" {
			rt = d.Type()
		}
	}
	_, err := edit.Typer{Host: hostOf(s.a)}.Value(context.Background(), edit.FromJSON(`{}`), rt)
	if !errors.Is(err, edit.ErrBadValue) {
		t.Fatalf("FromJSON {} as %v: %v, want a ValueError", rt, err)
	}
	set := func(raw string) (*edit.Plan, error) {
		return edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{
			{Kind: edit.OpSet, Path: "x", Value: edit.FromJSON(raw)},
		}})
	}
	if _, err := set(`{}`); !errors.Is(err, edit.ErrBadValue) {
		t.Fatalf("Set(x, {}) = %v, want a ValueError", err)
	}
	// V3: a required field left out is the re-check's E3302; b, given, is written, since its
	// default can no longer be computed from the new record (E6 does not apply).
	plan, err := set(`{"b": 2}`)
	if err != nil || len(plan.Changes) != 1 || !strings.HasSuffix(string(plan.Changes[0].After), "let x: R = { b: 2 }\n") {
		t.Fatalf("Set(x, {b: 2}) = %v, %v; want { b: 2 }", plan, err)
	}
}

// API.md W1, API.md W4 (log-2026-09-29 U4c final): a JSON source whose file is not named
// `.json` has the editability of the format row; one named `.json` is edited in JSON mode.
func TestJSONSourceNamedJSON(t *testing.T) {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/p/p.canon": file("package p\n\n/// C.\nrecord C {\n  /// L.\n  l: Int\n}\n\n/// A.\nlet a: C = load(\"a.cfg\", format: json)\n\n" +
			"/// B.\nlet b: C = load(\"b.json\")\n"),
		"law/p/a.cfg":  file("{\"l\": 1}\n"),
		"law/p/b.json": file("{\"l\": 2}\n"),
	}
	s := open(t, fsys, nil, "", "p")
	for _, c := range []struct {
		path   string
		mode   edit.Mode
		reason edit.Reason
	}{{"p:a.l", edit.ModeNone, edit.ReasonFormat}, {"p:b.l", edit.ModeJSON, edit.ReasonNone}} {
		p, err := edit.Parse(c.path)
		if err != nil {
			t.Fatal(err)
		}
		r, err := edit.Resolve(s.snap, p)
		if err != nil {
			t.Fatal(err)
		}
		e, err := s.snap.Editable(r, edit.OpSet, "")
		if err != nil || e.Mode != c.mode || e.Reason != c.reason {
			t.Errorf("Editable(%s) = %+v, %v; want mode %d reason %d", c.path, e, err, c.mode, c.reason)
		}
	}
}

// API.md E26: every Lit is written as `source`, Canon literal text re-printed by format.Flat
// (canonical, single-line); a FromJSON as `value`. Text that is not one literal is kept.
func TestOpJSONWritesEveryLit(t *testing.T) {
	cases := []struct {
		lit  edit.Lit
		want string
	}{
		{edit.Bool(true), "true"},
		{edit.Int(-3), "-3"},
		{edit.Float(1.5), "1.5"},
		{edit.Str("a \"b\""), `"a \"b\""`},
		{edit.Dur(90_000_000_000), "1m30s"},
		{edit.Member("red"), "red"},
		{edit.Key("two words"), `"two words"`},
		{edit.IntKey(7), "7"},
		{edit.PathKey("open"), "open"},
		{edit.None{}, "none"},
		{edit.List{edit.Int(1), edit.Int(2)}, "[1, 2]"},
		{edit.Obj{"b": edit.Int(2), "a": edit.List{}}, "{ a: [], b: 2 }"},
		{edit.Map{{Key: edit.Str("k"), Value: edit.Int(1)}}, `{ "k": 1 }`},
		{edit.Case{Name: "circle", Fields: edit.Obj{"r": edit.Int(2)}}, "circle { r: 2 }"},
		{edit.Case{Name: "dot"}, "dot"},
		{edit.Source("{a:1,b:[1,2]}"), "{ a: 1, b: [1, 2] }"},
		{edit.Source("not ( a literal"), "not ( a literal"},
	}
	for _, c := range cases {
		b, err := json.Marshal(edit.Operation{Kind: edit.OpSet, Path: "p", Value: c.lit})
		want, _ := json.Marshal(map[string]string{"source": c.want})
		if err != nil || string(b) != `{"op":"set","path":"p",`+string(want[1:]) {
			t.Errorf("%#v = %s, %v, want source %s", c.lit, b, err, c.want)
		}
	}
	for _, lit := range []edit.Lit{edit.Float(math.Inf(1)), edit.Dur(1500), edit.List{edit.FromJSON(`1`)}} {
		if _, err := json.Marshal(edit.Operation{Kind: edit.OpSet, Path: "p", Value: lit}); !errors.Is(err, edit.ErrNoText) {
			t.Errorf("%#v: %v, want ErrNoText", lit, err)
		}
	}
}

// API.md E26, V1: the text written for a value types back as the same value.
func TestOpJSONSourceTypesBack(t *testing.T) {
	ts := fieldTypes(t)
	for _, c := range []struct {
		field string
		lit   edit.Lit
	}{{"n", edit.Int(4)}, {"ps", edit.List{edit.Obj{"name": edit.Str("x"), "w": edit.Int(2)}}}, {"sh", edit.Case{Name: "dot"}}, {"byC", edit.Map{{Key: edit.Member("red"), Value: edit.Int(1)}}}} {
		b, err := json.Marshal(edit.Operation{Kind: edit.OpSet, Path: "p", Value: c.lit})
		if err != nil {
			t.Fatal(err)
		}
		var back edit.Operation
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatal(err)
		}
		want, err1 := edit.Typer{Host: noHost{}}.Value(context.Background(), c.lit, ts[c.field])
		got, err2 := edit.Typer{Host: noHost{}}.Value(context.Background(), back.Value, ts[c.field])
		if err1 != nil || err2 != nil || want.CanonText() != got.CanonText() {
			t.Errorf("%s: %v, %v; %v then %v", c.field, want, got, err1, err2)
		}
	}
}

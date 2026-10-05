package ir_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	importedSrc = "package b\n\n/// Colors.\nenum Color { x, y }\n\n/// A key.\ntype K = Color | \"x\"\n\n/// The map.\nlet m: {K: Int} = {}\n"
	importerSrc = "package a\n\nimport b { m, K }\n\n/// The file.\n@text(\"clash.json\")\nexport fn clash() -> {K: Int} { return m }\n\nemit text { out: \"out/\" }\n"
	clashSrc    = "package a\n\n/// Colors.\nenum Color { x, y }\n\n/// A key.\ntype K = Color | \"x\"\n\n/// The file.\n@text(\"clash.json\")\nexport fn clash() -> {K: Int} { return { \"x\": 1 } }\n\nemit text { out: \"out/\" }\n"
	listSrc     = "package a\n\n/// The file.\n@text(\"list.json\")\nexport fn list() -> [Int] { return [1, 2] }\n\nemit text { out: \"out/\" }\n"
	textSrc     = "package a\n\n/// A name.\ntype Name = String(1..)\n\n/// A mode.\ntype Mode = String | \"auto\"\n\n/// The file.\n@text(\"name.txt\")\nexport fn name() -> Name { return \"x\" }\n\n/// The file.\n@text(\"mode.txt\")\nexport fn mode() -> Mode { return \"auto\" }\n\nemit text { out: \"out/\" }\n"
	misplace    = "package a\n\n/// The file.\n@text(\"a.txt\")\nexport fn a() -> String { return \"x\" }\n\n/// Misplaced.\n@text(\"b.json\")\nexport fn g(x: Int?) -> Range { return 0..1 }\n\nemit text { out: \"out/\" }\nemit go { out: \"@features/a\", package: \"a\" }\n"
)

func intVal(n int64) value.Value { return &value.Int{V: n, T: types.IntType} }

// declIn is the declaration name of package pkg.
func declIn(t *testing.T, w *world, pkg, name string) check.Object {
	t.Helper()
	for _, p := range w.prog.Packages {
		if p.Path != pkg {
			continue
		}
		for _, o := range p.Decls {
			if o.Name() == name {
				return o
			}
		}
	}
	t.Fatalf("no declaration %s.%s", pkg, name)
	return nil
}

// colorMember is member name of package pkg's enum Color.
func colorMember(t *testing.T, w *world, pkg, name string) value.Value {
	t.Helper()
	e, ok := declIn(t, w, pkg, "Color").Type().(*types.EnumType)
	if ok {
		if i := slices.IndexFunc(e.Members, func(m *types.Member) bool { return m.Name == name }); i >= 0 {
			return &value.Member{Enum: e, Index: i}
		}
	}
	t.Fatalf("no member %s", name)
	return nil
}

// WIRE.md §5.8, §8.5, DECISIONS 283, 308: a `@text` result map holding the member x and the literal "x", one wire text, is E3317 at stage E, the keys printed as the verifier prints them; the harness's JSON fixtures cannot hold two such keys, so the value is built here.
func TestTextResultKeyCollision(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(clashSrc))
	w.check(t)
	w.calls = func(fn check.Object, _ value.Value, _ []value.Value) (value.Value, bool) {
		m := &value.Map{
			T:    &types.MapType{Key: declIn(t, w, "a", "K").Type(), Value: types.IntType},
			Keys: []value.Value{colorMember(t, w, "a", "x"), &value.Str{V: "x", T: types.StringType}},
			Vals: []value.Value{intVal(1), intVal(2)},
		}
		return m, fn.Name() == "clash"
	}
	w.build(t)
	out := w.findings(t)
	if !strings.Contains(out, "["+string(diag.E3317.Def().Code)+"]") || !strings.Contains(out, `map keys x and "x" both encode to "x"`) {
		t.Errorf("want the finding on the clashing keys:\n%s", out)
	}
}

// WIRE.md §8.5, DECISIONS 308: a `@text` result that is not a String is written as JSON, pretty at depth 0 with one final LF; a package whose result is fine reports nothing.
func TestTextResultWrittenAsJSON(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(listSrc))
	w.check(t)
	w.calls = func(fn check.Object, _ value.Value, _ []value.Value) (value.Value, bool) {
		l := &value.List{T: &types.ListType{Elem: types.IntType}, Elems: []value.Value{intVal(1), intVal(2)}}
		return l, fn.Name() == "list"
	}
	pkgs := w.build(t)
	if out := w.findings(t); !strings.Contains(out, noErrors) {
		t.Fatalf("want no finding:\n%s", out)
	}
	files, err := ir.TextFiles(w.prog.Packages[0], pkgs[0])
	if err != nil || len(files) != 1 || string(files[0].Content) != "[\n  1,\n  2\n]\n" {
		t.Errorf("TextFiles = %q, %v", files, err)
	}
}

// DECISIONS 308 (precisions): a result whose type is an alias or refinement of String, or a literal union of strings, is written verbatim, not as JSON.
func TestTextResultTextTypesVerbatim(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(textSrc))
	w.check(t)
	w.calls = func(fn check.Object, _ value.Value, _ []value.Value) (value.Value, bool) {
		return &value.Str{V: "x", T: types.StringType}, true
	}
	pkgs := w.build(t)
	files, err := ir.TextFiles(w.prog.Packages[0], pkgs[0])
	if err != nil || len(files) != 2 {
		t.Fatalf("TextFiles = %q, %v", files, err)
	}
	for _, f := range files {
		if string(f.Content) != "x" {
			t.Errorf("%s = %q, want the string verbatim", f.Path, f.Content)
		}
	}
}

// CODEGEN.md §2.9, DECISIONS 308: a `@text` that check judged misplaced (E8021 position) is not a `@text` fn for stage E: the fn stays API (E9003 on its optional parameter) and gets no E8151.
func TestMisplacedTextIsNotAFile(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(misplace))
	w.check(t)
	w.calls = func(fn check.Object, _ value.Value, _ []value.Value) (value.Value, bool) {
		return &value.Str{V: "x", T: types.StringType}, fn.Name() == "a"
	}
	w.build(t)
	out := w.findings(t)
	for _, want := range []diag.Code{diag.E8021.Def().Code, diag.E9003.Def().Code} {
		if !strings.Contains(out, "["+string(want)+"]") {
			t.Errorf("want %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "["+string(diag.E8151.Def().Code)+"]") {
		t.Errorf("a misplaced @text got a wire-form finding:\n%s", out)
	}
}

// WIRE.md §5.8, DECISIONS 308 (ruling): the `@text` walk skips the values stage B verified in any package: a result returning an imported package's let repeats no finding of that package's verify.
func TestTextResultSkipsImportedLet(t *testing.T) {
	w := newWorld(t)
	w.add(t, "b/b.canon", []byte(importedSrc))
	w.add(t, "a/a.canon", []byte(importerSrc))
	w.add(t, "b.m.json", []byte(`{ "x": 1 }`))
	w.check(t)
	w.calls = func(fn check.Object, _ value.Value, _ []value.Value) (value.Value, bool) {
		v, ok := w.Value(context.Background(), "b", "m")
		m, isMap := v.(*value.Map)
		if !ok || !isMap {
			t.Fatalf("b.m = %v, %v", v, ok)
		}
		m.Keys = append(m.Keys, colorMember(t, w, "b", "x"))
		m.Vals = append(m.Vals, intVal(2))
		return m, fn.Name() == "clash"
	}
	w.build(t)
	if out := w.findings(t); strings.Contains(out, "["+string(diag.E3317.Def().Code)+"]") {
		t.Errorf("an imported let's composite was walked again:\n%s", out)
	}
}

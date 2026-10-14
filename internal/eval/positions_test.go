package eval_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// positionsBuild evaluates testdata/positions/positions.txtar with its layer.
func positionsBuild(t *testing.T) *build {
	t.Helper()
	a, err := txtar.ParseFile("testdata/positions/positions.txtar")
	if err != nil {
		t.Fatal(err)
	}
	b := runBuildWith(t, fromArchive(t, a), eval.Options{Layers: []string{"dev"}}, nil)
	if out := b.findings(t); strings.Contains(out, "error[") {
		t.Fatalf("positions.txtar has errors:\n%s", out)
	}
	return b
}

// leaves are the values of root name a mark is read on: a list's elements, a record's fields,
// else the value itself.
func (b *build) leaves(t *testing.T, name string) []value.Value {
	t.Helper()
	v, ok := b.values[eval.Root{Pkg: "a", Name: name}]
	if !ok || v == nil {
		t.Fatalf("root %s: no value", name)
	}
	switch x := v.(type) {
	case *value.List:
		if len(x.Elems) == 0 {
			t.Fatalf("root %s: an empty list proves nothing", name)
		}
		return x.Elems
	case *value.Record:
		return x.Fields
	}
	return []value.Value{v}
}

// expectMarks fails when a leaf of a root is not marked as want says.
func expectMarks(t *testing.T, b *build, roots []string, want bool) {
	t.Helper()
	for _, name := range roots {
		for i, v := range b.leaves(t, name) {
			if got := v.Prov() != nil && v.Prov().Position; got != want {
				t.Errorf("%s, leaf %d (%s): position mark %v, want %v", name, i, v.CanonText(), got, want)
			}
		}
	}
}

// EVALUATION.md §13.1: enumerate and indexOf mark entries' positions only.
func TestEntryPositions(t *testing.T) {
	b := positionsBuild(t)
	expectMarks(t, b, []string{"mTable", "mFiltered", "mKeys", "mKeyed", "mIndexOf", "mKeysIndexOf", "mFnEnumerate"}, true)
	expectMarks(t, b, []string{"uPlain", "uPlainIndexOf", "uMapped", "uRange"}, false)
}

// EVALUATION.md §13.1: each carrying rule keeps the mark; nothing else carries it.
func TestPositionCarried(t *testing.T) {
	b := positionsBuild(t)
	expectMarks(t, b, []string{
		"i1", "s1", "mAdd", "mAddRight", "mSub", "mMul", "mDiv", "mMod", "mNeg", "mTemplate", "mSpec",
		"mStrPlus", "mStrPlusRight", "mInt", "mFloat", "mString", "mJoin", "mPassed", "mRec",
		"mSpread", "mLayered", "mInFn", "mDefault",
	}, true)
	expectMarks(t, b, []string{
		"uBool", "uAt", "uIndex", "uLen", "uPositional", "uPositionalSum", "uPositionalNamed", "uFloor",
		"uAbs", "uAbsNeg", "uMin", "uMax", "uClamp",
	}, false)
	if layered := b.leaves(t, "mLayered")[0]; layered.Prov().Kind != value.ProvLayer || layered.Prov().Layer() != "dev" {
		t.Errorf("mLayered.n: want the layer's provenance, got kind %d layer %q", layered.Prov().Kind, layered.Prov().Layer())
	}
	if spread := b.leaves(t, "mSpread")[0]; spread.Prov().Kind != value.ProvSpread || spread.Prov().Via == nil {
		t.Errorf("mSpread.n: want a spread via mRec.n, got kind %d", spread.Prov().Kind)
	}
	if def := b.leaves(t, "mDefault")[1]; def.Prov().Kind != value.ProvDefault {
		t.Errorf("mDefault.n: want a default's provenance, got kind %d", def.Prov().Kind)
	}
	if inFn := b.leaves(t, "mInFn")[0]; len(inFn.Prov().Stack()) == 0 || inFn.Prov().Stack()[0].Fn != "plusOne" {
		t.Errorf("mInFn: want a value computed in plusOne's frame, got stack %v", inFn.Prov().Stack())
	}
}

// EVALUATION.md §13.1 blind spots, pinned unmarked: composites under String and templates, join's separator, string methods.
func TestPositionBlindSpots(t *testing.T) {
	b := positionsBuild(t)
	expectMarks(t, b, []string{"bStringList", "bTemplateList", "bJoinSeparator", "bTrim"}, false)
}

// DECISIONS 379: the mark belongs to the provenance, not the value: equality, hashing and the
// text form are those of the same value unmarked.
func TestPositionInvisible(t *testing.T) {
	b := positionsBuild(t)
	for _, name := range []string{"mAdd", "mFloat", "mTemplate"} {
		marked := b.leaves(t, name)[0]
		if !value.IsPosition(marked) {
			t.Fatalf("%s: want a marked value to compare", name)
		}
		p := *marked.Prov()
		p.Position = false
		plain := unmarked(marked, &p)
		if !value.Equal(marked, plain) {
			t.Errorf("%s: a marked value is not equal to itself unmarked", name)
		}
		if value.Hash(marked) != value.Hash(plain) {
			t.Errorf("%s: the mark changes the hash", name)
		}
		if marked.CanonText() != plain.CanonText() {
			t.Errorf("%s: the mark changes the text form: %q, %q", name, marked.CanonText(), plain.CanonText())
		}
	}
}

// unmarked is scalar v with provenance p.
func unmarked(v value.Value, p *value.Prov) value.Value {
	switch x := v.(type) {
	case *value.Int:
		return &value.Int{V: x.V, T: x.T, P: p}
	case *value.Float:
		return &value.Float{V: x.V, T: x.T, P: p}
	case *value.Str:
		return &value.Str{V: x.V, T: types.StringType, P: p}
	}
	return v
}

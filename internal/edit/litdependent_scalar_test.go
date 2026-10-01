package edit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/types"
)

// scalarSrc's dependent field `any` has a branch of each scalar kind, `anys` is a list of them;
// `num` has only Int branches.
const scalarSrc = `package d

/// Which kind.
enum K { i, s, f, b, d }

/// A value of each kind.
type Any(k: K) = match k {
  i => Int
  s => String
  f => Float
  b => Bool
  d => Duration
}

/// Only integers.
type Num(k: K) = match k {
  i => Int
  s => Int
  f => Int
  b => Int
  d => Int
}

/// A cell.
record Cell {
  /// Its kind.
  k: K = i
  /// Its value.
  any: Any(k)?
  /// Its number.
  num: Num(k)?
  /// Values of its kind.
  anys: [Any(k)] = []
}

/// One cell.
let cell: Cell = {}
`

func scalarFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(scalarSrc)}
}

// scalarTypes are the declared types of cell's fields any and num.
func scalarTypes(t *testing.T) (anyT, numT types.Type) {
	t.Helper()
	p, err := build.Open(scalarFS(), "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	f := analyze(t, p, []string{"d"}, true)
	var ts []types.Type
	for _, path := range []string{"d:cell.any", "d:cell.num"} {
		typ, err := f.Type(resolve(t, f, path))
		if err != nil {
			t.Fatal(err)
		}
		ts = append(ts, typ)
	}
	return ts[0], ts[1]
}

// TYPES.md 11.4 "Literals", API.md V1: a scalar literal, a Lit or in a Source, given to a dependent
// type is accepted when a branch takes it (a string or integer kept as written, a float, Bool or
// Duration of its own type), and a ValueError when none does.
func TestDependentScalarLiterals(t *testing.T) {
	anyT, numT := scalarTypes(t)
	for _, c := range []struct {
		lit  edit.Lit
		want string // the value typed for any; num takes only the integer
	}{
		{edit.Int(3), "3"},
		{edit.Str("x"), "x"},
		{edit.Float(1.5), "1.5"},
		{edit.Bool(true), "true"},
		{edit.Dur(2 * time.Second), "2s"},
		{edit.Source("3"), "3"},
		{edit.Source(`"x"`), "x"},
		{edit.Source("1.5"), "1.5"},
		{edit.Source("true"), "true"},
		{edit.Source("2s"), "2s"},
	} {
		ty := edit.Typer{Host: noHost{}}
		v, err := ty.Value(context.Background(), c.lit, anyT)
		if err != nil || v.CanonText() != c.want {
			t.Errorf("%#v given to Any(k): %v, %v, want %s", c.lit, v, err, c.want)
		}
		v, err = ty.Value(context.Background(), c.lit, numT)
		integer := c.want == "3"
		switch {
		case integer && err != nil:
			t.Errorf("%#v given to Num(k): %v, want it kept", c.lit, err)
		case !integer && !errors.Is(err, edit.ErrBadValue):
			t.Errorf("%#v given to Num(k): %v, %v, want ErrBadValue", c.lit, v, err)
		}
	}
	for _, lit := range []edit.Lit{edit.Dur(time.Microsecond), edit.Float(1.5)} {
		if _, err := (edit.Typer{Host: noHost{}}).Value(context.Background(), lit, numT); !errors.Is(err, edit.ErrBadValue) {
			t.Errorf("%#v given to Num(k): %v, want ErrBadValue", lit, err)
		}
	}
}

// TYPES.md 11.4, API.md V1, E22, M6: a whole record whose dependent field is a scalar of each
// kind, or a list of them, as an Obj or a Source, is written and checks clean, its Undo
// round-tripping.
func TestDependentScalarInRecord(t *testing.T) {
	for _, c := range []struct {
		k   string
		lit edit.Lit
	}{
		{"i", edit.Int(3)}, {"s", edit.Str("x")}, {"f", edit.Float(1.5)}, {"b", edit.Bool(true)}, {"d", edit.Dur(2 * time.Second)},
	} {
		ops := []edit.Operation{setAt("cell", edit.Obj{"k": edit.Member(c.k), "any": c.lit, "anys": edit.List{c.lit, c.lit}})}
		checkClean(t, "Obj, "+c.k, scalarFS(), ops)
		undoTwice(t, "Obj, "+c.k, scalarFS(), ops)
	}
	for _, src := range []string{`{ k: i, any: 3 }`, `{ k: s, any: "x" }`, `{ k: f, any: 1.5 }`, `{ k: b, any: true }`, `{ k: d, any: 2s }`, `{ k: f, anys: [1.5, 2] }`, `{ k: b, anys: [true] }`} {
		ops := []edit.Operation{setAt("cell", edit.Source(src))}
		checkClean(t, src, scalarFS(), ops)
		undoTwice(t, src, scalarFS(), ops)
	}
	s := open(t, scalarFS(), nil, "", "d")
	_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{setAt("cell", edit.Obj{"k": edit.Member("b"), "num": edit.Bool(true)})}})
	if !errors.Is(err, edit.ErrBadValue) {
		t.Errorf("a Bool no branch of Num takes: %v, want ErrBadValue", err)
	}
}

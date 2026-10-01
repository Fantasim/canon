package wire_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/wire"
)

// WIRE.md 4.1, 5.1, 5.3, 5.4 (log-2026-09-29 M4 B10-r): Decoder.Field gives the decoded whole its
// field's unit, int and bits forms and none marker, as a member of that field would take them.
func TestDecodeField(t *testing.T) {
	marker := func(t types.Type) *types.Field {
		f := field("x", t)
		f.NoneWire = []byte(`""`)
		return f
	}
	unit := func(t types.Type, u types.Unit) *types.Field {
		f := field("x", t)
		f.Unit = u
		return f
	}
	enc := func(t types.Type, e types.Enc) *types.Field {
		f := field("x", t)
		f.Enc = e
		return f
	}
	durs := &types.ListType{Elem: types.DurationType}
	flags := &types.ListType{Elem: flag}
	optStr := &types.OptionalType{Elem: types.StringType}
	for _, c := range []struct {
		name, src string
		typ       types.Type
		f         *types.Field
		want      string
	}{
		{"no field", `2`, types.DurationType, nil, "2ms"},
		{"unit", `2`, types.DurationType, unit(types.DurationType, types.UnitS), "2s"},
		{"unit, list", `[1, 2]`, durs, unit(durs, types.UnitM), "[1m, 2m]"},
		{"int", `1`, types.BoolType, enc(types.BoolType, types.EncInt), "true"},
		{"bits", `5`, flags, enc(flags, types.EncBits), "[tradable, soulbound]"},
		{"none marker", `""`, optStr, marker(optStr), "none"},
		{"not the marker", `"a"`, optStr, marker(optStr), "a"},
	} {
		d := decodeJSON(t, wire.Decoder{Field: c.f}, c.src, c.typ)
		if !d.ok || d.v.CanonText() != c.want {
			t.Errorf("%s: %v %v %q, want %s", c.name, d.ok, d.v, d.findings, c.want)
		}
	}
}

// WIRE.md 4.1, 6.5, DECISIONS 268: Dir gives each file's element Decoder.Field's unit and int
// form; a record element keeps its own fields' forms, and no field leaves the defaults.
func TestDirField(t *testing.T) {
	scoped := func(u types.Unit, e types.Enc) *types.Field {
		f := field("x", types.DurationType)
		f.Unit, f.Enc = u, e
		return f
	}
	wait := record("flow", "Wait", field("t", types.DurationType))
	for _, c := range []struct {
		name  string
		texts []string
		typ   types.Type
		f     *types.Field
		want  string
	}{
		{"no field", []string{`5`, `7`}, listOf(types.DurationType), nil, "[5ms, 7ms]"},
		{"unit", []string{`5`, `7`}, listOf(types.DurationType), scoped(types.UnitS, types.EncPlain), "[5s, 7s]"},
		{"int", []string{`1`, `0`}, listOf(types.BoolType), scoped(types.UnitMs, types.EncInt), "[true, false]"},
		{"records keep theirs", []string{`{"t": 5}`, `{"t": 7}`}, listOf(wait), scoped(types.UnitS, types.EncPlain), "[Wait{t: 5ms}, Wait{t: 7ms}]"},
	} {
		fs := &source.FileSet{}
		bag := diag.NewBag(fs, "p")
		dec := wire.Decoder{Bag: bag, Pkg: "p", Host: newHost(), Field: c.f}
		v, ok, err := dec.Dir(context.Background(), dirOf(t, fs, bag, []string{"a", "b"}, c.texts), c.typ)
		if err != nil || !ok || v.CanonText() != c.want {
			t.Errorf("%s: %v %v %v %v, want %s", c.name, v, ok, err, bag.Findings(), c.want)
		}
	}
}

package wire_test

import (
	"testing"

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

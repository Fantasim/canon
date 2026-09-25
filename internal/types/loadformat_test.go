package types_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// WIRE.md §6.2: the format of a path's extension, ASCII case ignored only, and of a `format:` symbol.
func TestLoadFormat(t *testing.T) {
	paths := []struct {
		name string
		want types.LoadFormat
	}{
		{"a/b.json", types.FormatJSON},
		{"B.CSV", types.FormatCSV},
		{"notes.TxT", types.FormatText},
		{"a.cſv", types.FormatUnknown},
		{"defines.h", types.FormatUnknown},
		{"noext", types.FormatUnknown},
	}
	for _, p := range paths {
		if got := types.FormatOfPath(p.name); got != p.want {
			t.Errorf("FormatOfPath(%q) = %d, want %d", p.name, got, p.want)
		}
	}
	for sym, want := range map[string]types.LoadFormat{"json": types.FormatJSON, "csv": types.FormatCSV, "text": types.FormatText, "txt": types.FormatUnknown, "": types.FormatUnknown} {
		if got := types.FormatNamed(sym); got != want {
			t.Errorf("FormatNamed(%q) = %d, want %d", sym, got, want)
		}
	}
}

// WIRE.md §6.6, §6.7: what a file of each format builds.
func TestFormatFits(t *testing.T) {
	rec := &types.RecordType{Pkg: "a", Name: "Row"}
	rows := &types.ListType{Elem: &types.ListType{Elem: types.StringType}}
	cases := []struct {
		f      types.LoadFormat
		header bool
		t      types.Type
		want   bool
	}{
		{types.FormatText, false, types.StringType, true},
		{types.FormatText, false, &types.OptionalType{Elem: types.StringType}, false},
		{types.FormatCSV, false, rows, true},
		{types.FormatCSV, false, &types.ListType{Elem: rec}, false},
		{types.FormatCSV, true, &types.ListType{Elem: rec}, true},
		{types.FormatCSV, true, &types.TableType{Elem: rec}, true},
		{types.FormatCSV, true, &types.ListType{Elem: types.IntType}, false},
		{types.FormatCSV, true, &types.OptionalType{Elem: &types.ListType{Elem: rec}}, false},
		{types.FormatJSON, false, types.IntType, true},
		{types.FormatUnknown, true, types.IntType, true},
	}
	for i, c := range cases {
		if got := types.FormatFits(c.f, c.header, c.t); got != c.want {
			t.Errorf("case %d: FormatFits(%d, %v, %s) = %v, want %v", i, c.f, c.header, c.t, got, c.want)
		}
	}
}

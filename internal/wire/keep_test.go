package wire_test

import (
	"context"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// keepCase is a source decoded with and without Keep: what each gives, and the findings,
// which Keep does not change.
type keepCase struct {
	name     string
	src      string
	typ      types.Type
	kept     string // the value's text with Keep; "" when Keep fails too
	findings []string
}

// API.md V2, WIRE.md §5.4: with Keep, a range, an overflow, a missing field, a key twice are kept.
func TestDecodeKeep(t *testing.T) {
	part := record("p", "Part", field("name", types.StringType), field("w", types.Int8Type))
	byN := &types.MapType{Key: types.Int8Type, Value: types.StringType}
	for _, c := range []keepCase{
		{"integer range", `1000`, types.Int8Type, "1000", []string{at(diag.E3201, "1:1", "")}},
		{"map key range", `{"300": "x"}`, byN, `{300: "x"}`, []string{at(diag.E3201, "1:2", "/300")}},
		{"missing field and range", `{"w": 1000}`, part, "Part{name: , w: 1000}",
			[]string{at(diag.E3302, "1:1", ""), at(diag.E3201, "1:7", "/w")}},
		{"one key text twice", `{"0": 1, "-0": 2}`, &types.MapType{Key: types.IntType, Value: types.IntType}, "{0: 1, 0: 2}",
			[]string{at(diag.E3317, "1:10", "/-0")}},
		{"Float64 overflow fails", `1e400`, types.FloatType, "", []string{at(diag.E3202, "1:1", "")}},
		{"shape fails", `"x"`, types.Int8Type, "", []string{at(diag.E7110, "1:1", "")}},
	} {
		plain := decodeJSON(t, wire.Decoder{}, c.src, c.typ)
		kept := decodeJSON(t, wire.Decoder{Keep: true}, c.src, c.typ)
		switch {
		case plain.ok || !slices.Equal(plain.findings, c.findings):
			t.Errorf("%s without Keep: %v %q, want a failure with %q", c.name, plain.ok, plain.findings, c.findings)
		case !slices.Equal(kept.findings, c.findings):
			t.Errorf("%s with Keep: findings %q, want %q", c.name, kept.findings, c.findings)
		case c.kept == "" && kept.ok, c.kept != "" && (!kept.ok || kept.v.CanonText() != c.kept):
			t.Errorf("%s with Keep = %v %v, want %q", c.name, kept.ok, kept.v, c.kept)
		}
	}
}

// API.md V2, WIRE.md §5.1: a Float32 overflow is kept whole, as the float64 the text holds.
func TestDecodeKeepFloat32Overflow(t *testing.T) {
	d := decodeJSON(t, wire.Decoder{Keep: true}, `1e39`, types.Float32Type)
	f, isFloat := d.v.(*value.Float)
	if !d.ok || !isFloat || f.V != 1e39 || !slices.Equal(d.findings, []string{at(diag.E3202, "1:1", "")}) {
		t.Errorf("1e39 as Float32 with Keep = %v %v %q, want 1e39 kept and its finding", d.ok, d.v, d.findings)
	}
}

// API.md V2, WIRE.md §6.5: with Keep, a stem given twice keeps both entries.
func TestDecodeKeepDuplicateStems(t *testing.T) {
	fs := &source.FileSet{}
	bag := diag.NewBag(fs, "p")
	var files []wire.File
	for _, label := range []string{"A", "B"} {
		f, _ := fs.Add(label+".json", "/p/"+label+".json", []byte(`{"label": "`+label+`"}`))
		root, err := jsonsrc.Parse(f, bag)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, wire.File{Sel: wire.Selection{Node: root}, Stem: "open", At: source.Span{File: f.ID}})
	}
	dec := wire.Decoder{Bag: bag, Pkg: "p", Keep: true}
	v, ok, err := dec.Dir(context.Background(), files, &types.TableType{Elem: status})
	want := `{open: Status{label: "A"}, open: Status{label: "B"}}`
	if !ok || err != nil || v.CanonText() != want || !slices.Equal(short(fs, bag.Findings()), []string{at(diag.E3102, "1:1", "")}) {
		t.Errorf("duplicate stems with Keep = %v %v %v %q, want %s and its finding", v, ok, err, short(fs, bag.Findings()), want)
	}
}

// API.md V2, WIRE.md §6.6: with Keep, a CSV keeps its missing column, empty cell, out-of-range cell.
func TestDecodeKeepCSV(t *testing.T) {
	part := record("p", "Part", field("name", types.StringType), field("w", types.Int8Type))
	for _, c := range []keepCase{
		{"missing column", "w\n1\n", listOf(part), "[Part{name: , w: 1}]", []string{at(diag.E3302, "1:1", "")}},
		{"empty cell, range", "name,w\n,1000\n", listOf(part), "[Part{name: , w: 1000}]",
			[]string{at(diag.E3302, "2:1", ""), at(diag.E3201, "2:2", "")}},
	} {
		plain := decodeCSV(t, wire.Decoder{}, c.src, c.typ)
		kept := decodeCSV(t, wire.Decoder{Keep: true}, c.src, c.typ)
		if plain.ok || !kept.ok || kept.v.CanonText() != c.kept || !slices.Equal(kept.findings, c.findings) || !slices.Equal(plain.findings, c.findings) {
			t.Errorf("%s: without Keep %v %q; with Keep %v %v %q; want %s and %q", c.name, plain.ok, plain.findings, kept.ok, kept.v, kept.findings, c.kept, c.findings)
		}
	}
}

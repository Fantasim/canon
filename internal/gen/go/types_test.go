package gogen_test

import (
	"errors"
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// typesMode switches an emit to types mode.
func typesMode(e *ir.Emit) { e.Mode = ir.ModeTypes }

// wiredRecord is a record of p whose Bool fields have their names as wire names.
func wiredRecord(name string, fields ...string) *ir.Record {
	r := &ir.Record{Pkg: "p", Name: name}
	for _, f := range fields {
		r.Fields = append(r.Fields, wired(f, f, "", boolT))
	}
	return r
}

// CODEGEN.md §2.2, §5.12, §5.13: types mode has no data for a stored export fn or a computed default (stage E's E8014) and writes no LoadInputs for an input field (E8019 InputField): each is ErrMalformed.
func TestTypesModeRefusals(t *testing.T) {
	stored := wiredRecord("R", "a")
	stored.Methods = []*ir.ExportFn{{Name: "f", Kind: ir.FnPrecomputed, Result: boolT}}
	computed := wiredRecord("R", "a")
	computed.Fields[0].Computed = true
	input := wiredRecord("R", "a")
	input.Fields[0].Input = &types.Input{Env: "A"}
	pkgFn := pkg(wiredRecord("R", "a"))
	pkgFn.Fns = []*ir.ExportFn{{Name: "f", Kind: ir.FnPrecomputed, Result: boolT, Value: &value.Bool{}}}
	cases := []struct {
		name, subject string
		p             *ir.Package
	}{
		{"a precomputed method", "p.R.f", pkg(stored)}, {"a computed default", "p.R.a", pkg(computed)},
		{"an input field", "p.R.a", pkg(input)}, {"a package-level precomputed fn", "p.f", pkgFn},
	}
	for _, c := range cases {
		var d *gogen.DetailError
		if err := generateErr(c.p, typesMode); !errors.Is(err, gogen.ErrMalformed) || !errors.As(err, &d) || d.Subject != c.subject {
			t.Errorf("%s: got %v, want ErrMalformed at %s", c.name, err, c.subject)
		}
	}
	if err := generateErr(pkg(wiredRecord("R", "a")), typesMode); err != nil {
		t.Errorf("a plain record: %v", err)
	}
}

// CODEGEN.md §2.2, §5.13: a types-mode file holds a public decoder per record and variant and the types, but no value accessor, loader, schema or snapshot, even of a public value; jsonDuration only beside a Duration it reads, and never math/big.
func TestTypesModeWritesDecodersOnly(t *testing.T) {
	r := wiredRecord("Item", "tradable")
	v := &ir.Variant{Pkg: "p", Name: "Shape", Tag: "kind", Cases: []*ir.Case{{Name: "dot", Wire: "dot"}}}
	p := pkg(r, v)
	p.Values = []*ir.Value{{Name: "item", Type: ir.TypeRef{Kind: types.Record, Named: r}, V: &value.Record{}}}
	typesMode(p.Emits[0])
	files, err := gogen.Generate(p, p.Emits[0])
	if err != nil {
		t.Fatal(err)
	}
	src := string(files[1].Content)
	for _, want := range []string{"func DecodeItem(raw []byte) (*Item, error) {", "func DecodeShape(raw []byte) (*Shape, error) {", "func jsonDocument(", "func Make_Item("} {
		if !strings.Contains(src, want) {
			t.Errorf("no %q", want)
		}
	}
	for _, unwanted := range []string{"GetItem", "LoadItem", "ItemSchema", "Snapshot", "sync.", "jsonDuration", "math/big"} {
		if strings.Contains(src, unwanted) {
			t.Errorf("a types-mode file holds %q", unwanted)
		}
	}
	r.Fields = append(r.Fields, wired("wait", "wait", "", ir.TypeRef{Kind: types.Duration}))
	files, err = gogen.Generate(p, p.Emits[0])
	if err != nil {
		t.Fatal(err)
	}
	if src := string(files[1].Content); !strings.Contains(src, "func jsonDuration(") || strings.Contains(src, "math/big") {
		t.Errorf("a Duration field: jsonDuration written %v, math/big imported %v", strings.Contains(src, "func jsonDuration("), strings.Contains(src, "math/big"))
	}
}

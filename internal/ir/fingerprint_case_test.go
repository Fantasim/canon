package ir_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// FINGERPRINT.md §4.4, decision 219: a case used as a type is case(@N,"<case wire tag>"), N its variant's number; the variant's block is written once, as for any variant.
func TestFingerprintCaseType(t *testing.T) {
	shape := &ir.Variant{Pkg: "a", Name: "Shape", Tag: "kind", Cases: []*ir.Case{
		{Name: "dot", Wire: "dot"},
		{Name: "box", Wire: "box", Fields: []*ir.Field{fld(integer(), "side")}},
	}}
	caseOf := func(i int) ir.TypeRef { return ir.TypeRef{Kind: types.Case, Named: shape, Case: shape.Cases[i]} }
	row := &ir.Record{Pkg: "a", Name: "Row", Fields: []*ir.Field{fld(caseOf(1), "b"), fld(caseOf(0), "d")}}
	root := named(row)
	want := "canon-fp v1\nroot @0\ntype @0 record params=0\n" +
		"  field [\"b\"] case(@1,\"box\") opt=0 none=- unit=- enc=-\n" +
		"  field [\"d\"] case(@1,\"dot\") opt=0 none=- unit=- enc=-\n" +
		"type @1 variant tag=\"kind\"\n  case \"dot\"\n  case \"box\"\n" +
		"    field [\"side\"] Int opt=0 none=- unit=- enc=-\n"
	text, err := ir.Fingerprint(&root, nil)
	if err != nil || string(text) != want {
		t.Errorf("Fingerprint = %v\n%s\nwant:\n%s", err, text, want)
	}
	if _, err := ir.Schema("a", "rows", &root, nil); err != nil {
		t.Errorf("Schema: %v", err)
	}
}

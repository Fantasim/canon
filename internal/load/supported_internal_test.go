package load

import (
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

func literalDefault() syntax.Expr {
	return &syntax.StringLit{Parts: []syntax.StringPart{{Text: "x"}}}
}

func nonLiteralDefault() syntax.Expr {
	return &syntax.IdentExpr{Name: "other"}
}

// DECISIONS 173, meta/decisions/log-2026-09-24.md "load.dir review (M2)": `supported` gates a
// decode this milestone can run with no host and no default the evaluator alone can build.
func TestSupported(t *testing.T) {
	literalRec := &types.RecordType{Fields: []*types.Field{{Name: "a", Type: types.StringType, Default: literalDefault()}}}
	badDefaultRec := &types.RecordType{Fields: []*types.Field{{Name: "a", Type: types.StringType, Default: nonLiteralDefault()}}}
	mismatchRec := &types.RecordType{Fields: []*types.Field{{Name: "a", Type: types.FloatType, Default: &syntax.IntLit{}}}}
	optionalRec := &types.RecordType{Fields: []*types.Field{{Name: "a", Type: &types.OptionalType{Elem: types.IntType}, Default: &syntax.IntLit{}}}}
	refField := &types.RecordType{Fields: []*types.Field{{Name: "r", Type: &types.RefType{}}}}
	cases := []struct {
		name string
		typ  types.Type
		want bool
	}{
		{"scalar string", types.StringType, true},
		{"scalar int", types.IntType, true},
		{"scalar enum", &types.EnumType{}, true},
		{"optional of supported", &types.OptionalType{Elem: types.IntType}, true},
		{"list of supported", &types.ListType{Elem: types.StringType}, true},
		{"table of literal-default record", &types.TableType{Elem: literalRec}, true},
		{"table of non-literal-default record", &types.TableType{Elem: badDefaultRec}, false},
		{"a literal default of the wrong kind (x: Float = 1)", &types.TableType{Elem: mismatchRec}, false},
		{"an int literal default of an optional int field", &types.TableType{Elem: optionalRec}, true},
		{"record with a ref field", refField, false},
		{"ref itself", &types.RefType{}, false},
		{"map", &types.MapType{Key: types.StringType, Value: types.IntType}, false},
		{"dependent map", &types.DepMapType{Value: types.IntType}, false},
		{"variant", &types.VariantType{}, false},
	}
	for _, c := range cases {
		if got := supported(c.typ); got != c.want {
			t.Errorf("%s: supported = %v, want %v", c.name, got, c.want)
		}
	}
}

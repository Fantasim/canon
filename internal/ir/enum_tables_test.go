package ir_test

import (
	"math"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// codedEnum is an enum with @codes whose members carry codes, in declaration order.
func codedEnum(codes ...int64) *ir.Enum {
	e := &ir.Enum{Codes: &ir.TypeRef{}}
	for _, c := range codes {
		e.Members = append(e.Members, &ir.EnumMember{Code: c})
	}
	return e
}

// TestDenseCodes proves CODEGEN.md §5.2 and DECISIONS 430: codes are dense when the enum has @codes, at least one member, and each code is the previous plus one without wrapping.
func TestDenseCodes(t *testing.T) {
	uncoded := codedEnum(0, 1)
	uncoded.Codes = nil
	tests := []struct {
		name  string
		enum  *ir.Enum
		first int64
		ok    bool
	}{
		{"from 6000", codedEnum(6000, 6001, 6002), 6000, true},
		{"negative", codedEnum(-128, -127), -128, true},
		{"from the minimum", codedEnum(math.MinInt64, math.MinInt64+1), math.MinInt64, true},
		{"to the maximum", codedEnum(math.MaxInt64-1, math.MaxInt64), math.MaxInt64 - 1, true},
		{"one member", codedEnum(7), 7, true},
		{"none", codedEnum(), 0, false},
		{"gap", codedEnum(1, 3), 0, false},
		{"descending", codedEnum(2, 1), 0, false},
		{"wrapping", codedEnum(math.MaxInt64, math.MinInt64), 0, false},
		{"no @codes", uncoded, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first, ok := ir.DenseCodes(tc.enum)
			if ok != tc.ok || first != tc.first {
				t.Errorf("DenseCodes = (%d, %v), want (%d, %v)", first, ok, tc.first, tc.ok)
			}
		})
	}
}

// TestEnumWiresAreNames proves CODEGEN.md §5.2 and DECISIONS 430: an enum's wires are its names when no member's wire differs, vacuously with no member; a variant's cases likewise.
func TestEnumWiresAreNames(t *testing.T) {
	enumOf := func(pairs ...string) *ir.Enum {
		e := &ir.Enum{}
		for i := 0; i < len(pairs); i += 2 {
			e.Members = append(e.Members, &ir.EnumMember{Name: pairs[i], Wire: pairs[i+1]})
		}
		return e
	}
	variantOf := func(pairs ...string) *ir.Variant {
		v := &ir.Variant{}
		for i := 0; i < len(pairs); i += 2 {
			v.Cases = append(v.Cases, &ir.Case{Name: pairs[i], Wire: pairs[i+1]})
		}
		return v
	}
	tests := []struct {
		name  string
		pairs []string
		want  bool
	}{
		{"no member", nil, true},
		{"equal", []string{"a", "a", "b", "b"}, true},
		{"one differs", []string{"a", "a", "series_1", "series-1"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ir.EnumWiresAreNames(enumOf(tc.pairs...)); got != tc.want {
				t.Errorf("EnumWiresAreNames = %v, want %v", got, tc.want)
			}
			if got := ir.CaseWiresAreNames(variantOf(tc.pairs...)); got != tc.want {
				t.Errorf("CaseWiresAreNames = %v, want %v", got, tc.want)
			}
		})
	}
}

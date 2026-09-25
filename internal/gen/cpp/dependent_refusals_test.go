package cppgen_test

import (
	"errors"
	"strings"
	"testing"

	cppgen "github.com/fantasim/canonlang/internal/gen/cpp"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// payloadOf is a Bool-discriminated `type P(on: Bool) = match on { false => String, true => String }`.
func payloadOf() *ir.Dependent {
	return &ir.Dependent{
		Pkg: "demo", Name: "P", Params: 1, Disc: &tBool, ByMember: []int{0, 1},
		Branches: []*ir.Branch{{Name: "false", Members: []int{0}, Type: tString}, {Name: "true", Members: []int{1}, Type: tString}},
	}
}

// withDependent adds payloadOf to the package and a field of Thing typed app of it.
func withDependent(name string, app func(*ir.Dependent) ir.TypeRef) func(*ir.Package, *ir.Emit) {
	return func(p *ir.Package, _ *ir.Emit) {
		d := payloadOf()
		p.Types = append(p.Types, d)
		thing(p).Fields = append(thing(p).Fields, field(name, name, "", app(d)))
	}
}

// fromField applies d to the earlier field at wire path from.
func fromField(from string) func(*ir.Dependent) ir.TypeRef {
	return func(d *ir.Dependent) ir.TypeRef { return appOf(d, from) }
}

// TestDependentAndInputRefusals is CODEGEN.md §4.1, §5.6, §5.7, §5.12 (decision 124): each refusal through its sentinel and its own words.
func TestDependentAndInputRefusals(t *testing.T) {
	flag := field("flag", "flag", "", tBool)
	refFlag := field("rf", "rf", "", ir.TypeRef{Kind: types.Ref, Key: &tString})
	allNever := payloadOf()
	allNever.Branches, allNever.ByMember = nil, []int{ir.NoBranch, ir.NoBranch}
	tests := []struct {
		name, words string
		edit        func(*ir.Package, *ir.Emit)
		want        error
	}{
		{"a discriminant that is a record parameter (§5.7)", "a record parameter", withDependent("p", func(d *ir.Dependent) ir.TypeRef {
			return ir.TypeRef{Kind: types.TypeApp, Named: d, Args: []*ir.Source{{From: types.ArgParam}}}
		}), cppgen.ErrUnsupported},
		{"a discriminant that is a dependent map key (§4.2)", "a dependent map key", withDependent("p", func(d *ir.Dependent) ir.TypeRef {
			return ir.TypeRef{Kind: types.TypeApp, Named: d, Args: []*ir.Source{{From: types.ArgKey}}}
		}), cppgen.ErrUnsupported},
		{"a dependent stored result", "not a field's type", func(p *ir.Package, _ *ir.Emit) {
			d := payloadOf()
			p.Types = append(p.Types, d)
			thing(p).Fields = append(thing(p).Fields, flag)
			thing(p).Methods = []*ir.ExportFn{{Name: "f", Kind: ir.FnPrecomputed, Result: appOf(d, "flag")}}
		}, cppgen.ErrUnsupported},
		{"a dependent type every arm of which is Never", "every arm of which is Never", func(p *ir.Package, _ *ir.Emit) {
			p.Types = append(p.Types, allNever)
		}, cppgen.ErrUnsupported},
		{"a discriminant read through a ref", "through a ref", func(p *ir.Package, e *ir.Emit) {
			thing(p).Fields = append(thing(p).Fields, refFlag)
			withDependent("p", fromField("rf"))(p, e)
		}, cppgen.ErrUnsupported},
		{"a discriminant path naming no field", "names no earlier field", withDependent("p", fromField("nope")), cppgen.ErrMalformed},
		{"a discriminant of another kind", "names no earlier field", withDependent("p", fromField("a")), cppgen.ErrMalformed},
		{"a discriminant of another enum", "names no earlier field", func(p *ir.Package, _ *ir.Emit) {
			disc, other := enumOf("D", "x", "y"), enumOf("O", "x", "y")
			tDisc := ir.TypeRef{Kind: types.Enum, Named: disc}
			d := &ir.Dependent{Pkg: "demo", Name: "P", Params: 1, Disc: &tDisc, ByMember: []int{0, 0},
				Branches: []*ir.Branch{{Name: "x", Members: []int{0, 1}, Type: tString}}}
			p.Types = append(p.Types, disc, other, d)
			thing(p).Fields = append(thing(p).Fields, field("o", "o", "", ir.TypeRef{Kind: types.Enum, Named: other}), field("p", "p", "", appOf(d, "o")))
		}, cppgen.ErrMalformed},
		{"a member map shorter than the discriminant", "members and branches disagree", func(p *ir.Package, _ *ir.Emit) {
			d := payloadOf()
			d.ByMember = d.ByMember[:1]
			p.Types = append(p.Types, d)
		}, cppgen.ErrMalformed},
		{"an input field of a case (EVALUATION.md §11.1)", "an input field outside a record", func(p *ir.Package, _ *ir.Emit) {
			p.Types = append(p.Types, &ir.Variant{Pkg: "demo", Name: "V", Tag: "k", Cases: []*ir.Case{
				{Name: "c", Wire: "c", Fields: []*ir.Field{input("x", "X", tString, true, nil)}},
			}})
		}, cppgen.ErrMalformed},
		{"a union over a string-keyed ref (owed)", "a literal union over a ref or dependent type, not generated yet",
			withField(field("u", "u", "", ir.TypeRef{Kind: types.LitUnion, Elem: &ir.TypeRef{Kind: types.Ref, Key: &tString}})), cppgen.ErrUnsupported},
		{"a union over a dependent type (owed)", "a literal union over a ref or dependent type, not generated yet", func(p *ir.Package, e *ir.Emit) {
			thing(p).Fields = append(thing(p).Fields, flag)
			withDependent("u", func(d *ir.Dependent) ir.TypeRef {
				app := appOf(d, "flag")
				return ir.TypeRef{Kind: types.LitUnion, Elem: &app}
			})(p, e)
		}, cppgen.ErrUnsupported},
		{"a union over an Int-keyed ref (check refuses it)", "a literal union whose wire is not a string",
			withField(field("u", "u", "", ir.TypeRef{Kind: types.LitUnion, Elem: &ir.TypeRef{Kind: types.Ref, Key: &tInt}})), cppgen.ErrMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, e := small(field("a", "a", "", tInt))
			tt.edit(p, e)
			_, err := cppgen.Generate(p, e)
			if !errors.Is(err, tt.want) || err == nil || !strings.Contains(err.Error(), tt.words) {
				t.Errorf("Generate: %v, want %v with %q", err, tt.want, tt.words)
			}
		})
	}
}

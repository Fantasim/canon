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

// TestDependentAndInputRefusals is CODEGEN.md §4.1, §5.6, §5.7, §5.12 (decisions 37, 124): each refusal through its sentinel and its own words; a dependent shape stage E refuses (E8019 DependentType) is malformed.
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
		{"a discriminant that is a record parameter (§5.7)", "whose discriminant is not read from earlier required fields", withDependent("p", func(d *ir.Dependent) ir.TypeRef {
			return ir.TypeRef{Kind: types.TypeApp, Named: d, Args: []*ir.Source{{From: types.ArgParam}}}
		}), cppgen.ErrMalformed},
		{"a discriminant that is a dependent map key (§4.2)", "whose discriminant is not read from earlier required fields", withDependent("p", func(d *ir.Dependent) ir.TypeRef {
			return ir.TypeRef{Kind: types.TypeApp, Named: d, Args: []*ir.Source{{From: types.ArgKey}}}
		}), cppgen.ErrMalformed},
		{"a dependent stored result", "not a field's type", func(p *ir.Package, _ *ir.Emit) {
			d := payloadOf()
			p.Types = append(p.Types, d)
			thing(p).Fields = append(thing(p).Fields, flag)
			thing(p).Methods = []*ir.ExportFn{{Name: "f", Kind: ir.FnPrecomputed, Result: appOf(d, "flag")}}
		}, cppgen.ErrMalformed},
		{"a dependent type every arm of which is Never (log-2026-09-28)", "every arm of which is Never", func(p *ir.Package, _ *ir.Emit) {
			p.Types = append(p.Types, allNever)
		}, cppgen.ErrMalformed},
		// unreachable: stage E holds every define table a ref targets (CODEGEN.md §5.8, DECISIONS 298).
		{"a define branch whose table the IR does not hold", "a ref into a load.defines table the package's IR does not hold", func(p *ir.Package, _ *ir.Emit) {
			d := payloadOf()
			d.Branches[1].Type = ir.TypeRef{Kind: types.Ref, Key: &tString, Ref: &ir.RefTarget{Coll: types.CollDefines, Pkg: "demo", Value: "defs"}}
			p.Types = append(p.Types, d)
		}, cppgen.ErrMalformed},
		{"a discriminant read through a ref (WIRE.md §5.9)", "whose discriminant is not read from earlier required fields", func(p *ir.Package, e *ir.Emit) {
			thing(p).Fields = append(thing(p).Fields, refFlag)
			withDependent("p", fromField("rf"))(p, e)
		}, cppgen.ErrMalformed},
		{"a discriminant path naming no field", "whose discriminant is not read from earlier required fields", withDependent("p", fromField("nope")), cppgen.ErrMalformed},
		{"a discriminant of another kind", "whose discriminant is not read from earlier required fields", withDependent("p", fromField("a")), cppgen.ErrMalformed},
		{"a discriminant of another enum", "whose discriminant is not read from earlier required fields", func(p *ir.Package, _ *ir.Emit) {
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
		{"a union over a string-keyed ref, which stage E refuses (RefUnion)", "a literal union over a ref",
			withField(field("u", "u", "", ir.TypeRef{Kind: types.LitUnion, Elem: &ir.TypeRef{Kind: types.Ref, Key: &tString}})), cppgen.ErrMalformed},
		{"a union over a dependent type, which stage E refuses", "a literal union over a dependent type", func(p *ir.Package, e *ir.Emit) {
			thing(p).Fields = append(thing(p).Fields, flag)
			withDependent("u", func(d *ir.Dependent) ir.TypeRef {
				app := appOf(d, "flag")
				return ir.TypeRef{Kind: types.LitUnion, Elem: &app}
			})(p, e)
		}, cppgen.ErrMalformed},
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

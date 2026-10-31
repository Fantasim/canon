package ir_test

import "github.com/fantasim/canonlang/internal/ir"

// classBounds are p's records and cases with fields, each with the name its bound constants are owned by (owner) and the names name gives them. The plans reserve every bound constant, written or not (CODEGEN.md §5.15), so a plan-to-generator comparison sets them aside on both sides.
func classBounds(p *ir.Package, owner func(ir.Type, *ir.Case) string, name func(string, *ir.Field, ir.BoundConst) string) map[string][]string {
	out := map[string][]string{}
	for _, t := range p.Types {
		switch x := t.(type) {
		case *ir.Record:
			out[owner(x, nil)] = fieldBoundNames(owner(x, nil), x.Fields, name)
		case *ir.Variant:
			for _, c := range x.Cases {
				out[owner(x, c)] = fieldBoundNames(owner(x, c), c.Fields, name)
			}
		}
	}
	return out
}

// fieldBoundNames are the bound constants of fields owned by class.
func fieldBoundNames(class string, fields []*ir.Field, name func(string, *ir.Field, ir.BoundConst) string) []string {
	var out []string
	for _, f := range fields {
		for _, b := range ir.FieldBounds(f) {
			out = append(out, name(class, f, b))
		}
	}
	return out
}

// ownerName names a record, or a case of a variant, with the plan's type and case names.
func ownerName(typeName func(ir.Type) string, caseName func(*ir.Variant, *ir.Case) string) func(ir.Type, *ir.Case) string {
	return func(t ir.Type, c *ir.Case) string {
		if v, ok := t.(*ir.Variant); ok {
			return caseName(v, c)
		}
		return typeName(t)
	}
}

// goBoundNames are the Go bound constants of p, all in the package scope.
func goBoundNames(p *ir.Package, pl *ir.GoNamePlan) []string {
	var out []string
	for _, names := range classBounds(p, ownerName(pl.TypeName, pl.CaseName), pl.BoundName) { //canon:unordered a set
		out = append(out, names...)
	}
	return out
}

// cppBoundNames are the C++ bound constants of p, by class scope.
func cppBoundNames(p *ir.Package, pl *ir.CppNamePlan) map[string][]string {
	return classBounds(p, ownerName(pl.TypeName, pl.CaseName), func(_ string, f *ir.Field, b ir.BoundConst) string { return pl.BoundName(f, b) })
}

// withoutNames is names less drop, a copy.
func withoutNames(names map[string]bool, drop []string) map[string]bool {
	out := map[string]bool{}
	for n := range names { //canon:unordered a set
		out[n] = true
	}
	for _, n := range drop {
		delete(out, n)
	}
	return out
}

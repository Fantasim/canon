package ir_test

import (
	"maps"

	"github.com/fantasim/canonlang/internal/ir"
)

// goPlanned are the plan's names by scope less the hooks it reserves but gen/go does not write: a data loader resolves one of the class's refs (GoHook.Written, log-2026-10-06 "U1 review" 3).
func goPlanned(p *ir.Package, pl *ir.GoNamePlan) map[string]map[string]bool {
	want := ir.GoScopeNames(pl)
	pkg := maps.Clone(want["package"])
	for _, n := range goUnwrittenHooks(p, pl) {
		delete(pkg, n)
	}
	want["package"] = pkg
	return want
}

// goUnwrittenHooks are the reserved Go make hooks of p's records and cases that are not written.
func goUnwrittenHooks(p *ir.Package, pl *ir.GoNamePlan) []string {
	var out []string
	for _, h := range goTypeHooks(p, pl) {
		if !h.Written {
			out = append(out, h.Name, h.CaseType)
		}
	}
	return out
}

// goTypeHooks are the Go make hooks of p's records and of each case of its variants.
func goTypeHooks(p *ir.Package, pl *ir.GoNamePlan) []ir.GoHook {
	var hooks []ir.GoHook
	for _, t := range p.Types {
		if rec, ok := t.(*ir.Record); ok {
			hooks = append(hooks, pl.RecordHook(rec))
		}
		if v, ok := t.(*ir.Variant); ok {
			for _, c := range v.Cases {
				hooks = append(hooks, pl.CaseHook(v, c))
			}
		}
	}
	return hooks
}

// cppPlanned are the C++ plan's names by scope less the members of detail::<P>Make it reserves but gen/cpp does not write: a data or types-mode loader resolves one of the class's refs (CppHook.Written, log-2026-10-06 "U1 review" 3).
func cppPlanned(p *ir.Package, pl *ir.CppNamePlan) map[string]map[string]bool {
	want := ir.CppScopeNames(pl)
	hooks := pl.MakeStruct(p.Name)
	members := maps.Clone(want[hooks])
	for _, h := range cppTypeHooks(p, pl) {
		if !h.Written {
			delete(members, h.Name)
			delete(members, h.CaseType)
		}
	}
	want[hooks] = members
	return want
}

// cppTypeHooks are the C++ make hooks of p's records and of each case of its variants.
func cppTypeHooks(p *ir.Package, pl *ir.CppNamePlan) []ir.CppHook {
	var hooks []ir.CppHook
	for _, t := range p.Types {
		if rec, ok := t.(*ir.Record); ok {
			hooks = append(hooks, pl.RecordHook(rec))
		}
		if v, ok := t.(*ir.Variant); ok {
			for _, c := range v.Cases {
				hooks = append(hooks, pl.CaseHook(v, c))
			}
		}
	}
	return hooks
}

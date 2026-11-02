package ir

import (
	"cmp"
	"slices"
	"strings"
)

// CppDeclared is a C++ name that a package's cpp emit declares for an enum member (Enum and Member set) or a constant (Const set): the declarations `@cpp(defines:)` guards (CODEGEN.md 7.9, API.md L9).
type CppDeclared struct {
	Name   string
	Enum   *Enum
	Member *EnumMember
	Const  *Const
}

// CppDeclaredNames are the names of p's cpp emits that §7.9's foreignDefine judges and `@cpp(defines:)` guards: the name of an enum member or a constant that `@cpp(name:)` does not rename (a prefix guard cannot cover a renamed one), from the name plan of each emit in a mode gen/cpp writes names for, each name and item once, sorted by name then declaration. Names the plan declares for anything else are not listed.
func CppDeclaredNames(p *Package) []CppDeclared {
	enumOf := map[*EnumMember]*Enum{}
	for _, t := range p.Types {
		if e, ok := t.(*Enum); ok {
			for _, m := range e.Members {
				enumOf[m] = e
			}
		}
	}
	var out []CppDeclared
	seen := map[declaredItem]bool{}
	for _, e := range p.Emits {
		if e.Target == TargetCpp && plannedMode(e.Mode) {
			out = append(out, planDeclared(PlanCppNames(p, e), enumOf, seen)...)
		}
	}
	slices.SortFunc(out, compareDeclared)
	return out
}

// planDeclared are the names of pl that CppDeclaredNames lists and seen does not hold yet, which it gains.
func planDeclared(pl *CppNamePlan, enumOf map[*EnumMember]*Enum, seen map[declaredItem]bool) []CppDeclared {
	var out []CppDeclared
	for _, sc := range pl.scopes {
		if sc.ids {
			continue
		}
		for name, item := range sc.items { //canon:unordered the caller sorts the result
			d, ok := guardable(name, item, sc.notes[name], enumOf)
			if k := (declaredItem{name, item}); ok && !seen[k] {
				seen[k] = true
				out = append(out, d)
			}
		}
	}
	return out
}

// declaredItem is a name with the item it was declared for.
type declaredItem struct {
	name string
	item any
}

// guardable is the CppDeclared of a name declared for an enum member or a constant in the way foreignDefine judges it: not built on a Canon name `@cpp(name:)` does not change, and not fixed by the generator.
func guardable(name string, item any, note foreignNote, enumOf map[*EnumMember]*Enum) (CppDeclared, bool) {
	if note.derived || note.fixed {
		return CppDeclared{}, false
	}
	switch x := item.(type) {
	case *EnumMember:
		if e := enumOf[x]; e != nil && x.Cpp.Name == "" {
			return CppDeclared{Name: name, Enum: e, Member: x}, true
		}
	case *Const:
		if x.Cpp.Name == "" {
			return CppDeclared{Name: name, Const: x}, true
		}
	}
	return CppDeclared{}, false
}

// compareDeclared orders names, then the declarations that hold them, by their order in the package.
func compareDeclared(a, b CppDeclared) int {
	return cmp.Or(strings.Compare(a.Name, b.Name), cmp.Compare(declOrder(a), declOrder(b)))
}

// declOrder is the position of d's declaration among the package's.
func declOrder(d CppDeclared) int {
	if d.Const != nil {
		return d.Const.Order
	}
	return d.Enum.Order
}

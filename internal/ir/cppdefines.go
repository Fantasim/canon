package ir

import (
	"slices"
	"strings"
)

// CppDefine is one line of a package's defines header, a member's or a constant's (CODEGEN.md §7.9): Const is nil for a member's, whose Enum and Member name it.
type CppDefine struct {
	Name   string
	Enum   *Enum
	Member *EnumMember
	Const  *Const
	order  int
}

// CppDefineLines are p's defines in declaration order, the members of an enum in theirs, retired ones included (CODEGEN.md §2.7, §7.9, DECISIONS 389, 390).
func CppDefineLines(p *Package) []CppDefine {
	var out []CppDefine
	for _, t := range p.Types {
		e, ok := t.(*Enum)
		if !ok || !e.HasCppDefines {
			continue
		}
		for _, m := range e.Members {
			out = append(out, CppDefine{Name: defineName(e.CppDefines, m.Name), Enum: e, Member: m, order: e.Order})
		}
	}
	for _, c := range p.Consts {
		if c.HasCppDefines {
			out = append(out, CppDefine{Name: defineName(c.CppDefines, c.Name), Const: c, order: c.Order})
		}
	}
	slices.SortStableFunc(out, func(a, b CppDefine) int { return a.order - b.order })
	return out
}

// CppDefineNames are the macros p's defines header writes, sorted and distinct (DECISIONS 391).
func CppDefineNames(p *Package) []string {
	var out []string
	for _, d := range CppDefineLines(p) {
		out = append(out, d.Name)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// defineName is a name's macro: the name when it starts with the prefix, else prefix + name (CODEGEN.md §7.9).
func defineName(prefix, name string) string {
	if strings.HasPrefix(name, prefix) {
		return name
	}
	return prefix + name
}

// origin is the Canon item the line defines, as findings name it.
func (d CppDefine) origin() string {
	if d.Const != nil {
		return d.Const.Name
	}
	return d.Enum.QName() + qnameSep + d.Member.Name
}

// decl is the declaration the line belongs to: its enum or its constant.
func (d CppDefine) decl() any {
	if d.Const != nil {
		return d.Const
	}
	return d.Enum
}

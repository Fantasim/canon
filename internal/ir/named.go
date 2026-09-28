package ir

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// site locates a declaration by its name, whose object knows its file.
func (s *stage) site(name *syntax.Ident, node syntax.Node) declSite {
	if name == nil {
		return declSite{}
	}
	if o := s.info.Defs[name]; o != nil {
		return declSite{file: o.File(), node: node}
	}
	return declSite{}
}

// enum is the IR of an enum, members retired ones included (CODEGEN.md §5.2).
func (s *stage) enum(e *types.EnumType) *Enum {
	if t, ok := s.named[e]; ok {
		return t.(*Enum)
	}
	out := &Enum{Pkg: e.Pkg, Name: e.Name, Doc: e.Doc, JSONCodes: e.WireCodes, Ordered: e.Ordered}
	s.named[e] = out
	if e.Codes != nil {
		codes := s.basicRef(*e.Codes)
		out.Codes = &codes
	}
	var decls []*syntax.EnumMember
	if e.Decl != nil {
		s.decls[out] = s.site(e.Decl.Name, e.Decl.Name)
		n := nameOverrides(e.Decl.Annotations)
		out.Go, out.Cpp, out.TS = n.goName, n.cpp, n.ts
		out.CppDefines = argText(annotation(e.Decl.Annotations, syntax.AnnCpp), syntax.ArgDefines)
		decls = e.Decl.Members
	}
	for i, m := range e.Members {
		im := &EnumMember{Name: m.Name, Wire: m.Wire, Doc: m.Doc, Index: m.Index, Code: m.Code, Retired: m.Retired, Deprecated: m.Deprecated != nil}
		if i < len(decls) {
			n := nameOverrides(decls[i].Annotations)
			im.Go, im.Cpp, im.TS = n.goName, n.cpp, n.ts
			s.nodeSites[im] = declSite{file: s.decls[out].file, node: decls[i].Name}
		}
		out.Members = append(out.Members, im)
	}
	return out
}

// record is the IR of a record; a parameterized one is one erased class (CODEGEN.md §5.7).
func (s *stage) record(r *types.RecordType) *Record {
	if t, ok := s.named[r]; ok {
		return t.(*Record)
	}
	out := &Record{Pkg: r.Pkg, Name: r.Name, Doc: r.Doc, Params: len(r.Params)}
	s.named[r] = out
	var items []syntax.RecordItem
	var owner check.Object
	if r.Decl != nil {
		s.decls[out] = s.site(r.Decl.Name, r.Decl.Name)
		owner = s.info.Defs[r.Decl.Name]
		n := nameOverrides(r.Decl.Annotations)
		out.Go, out.TS = n.goName, n.ts
		out.Cpp = recordCpp(r.Decl.Annotations)
		if r.Decl.Body != nil {
			items = r.Decl.Body.Items
		}
	}
	out.Fields = s.fields(r.Fields, items, owner, s.recordParams(r))
	out.Methods = s.methods(r.Methods, items, out.Name, r)
	return out
}

// variant is the IR of a variant, cases retired ones included (CODEGEN.md §5.5).
func (s *stage) variant(v *types.VariantType) *Variant {
	if t, ok := s.named[v]; ok {
		return t.(*Variant)
	}
	out := &Variant{Pkg: v.Pkg, Name: v.Name, Doc: v.Doc, Tag: v.Tag}
	s.named[v] = out
	var items []syntax.VariantItem
	if v.Decl != nil {
		s.decls[out] = s.site(v.Decl.Name, v.Decl.Name)
		s.members[out] = s.variantMembers(s.decls[out].file, v.Decl.Items)
		n := nameOverrides(v.Decl.Annotations)
		out.Go, out.Cpp, out.TS = n.goName, n.cpp, n.ts
		items = v.Decl.Items
	}
	for _, c := range v.Cases {
		out.Cases = append(out.Cases, &Case{Name: c.Name, Wire: c.Wire, Doc: c.Doc, Retired: c.Retired})
	}
	for i, c := range v.Cases {
		d := caseDecl(items, c.Name)
		if d != nil {
			s.nodeSites[out.Cases[i]] = declSite{file: s.decls[out].file, node: d.Name}
		}
		s.fillCase(out.Cases[i], c, d, out.Name)
	}
	return out
}

// fillCase fills a case's fields and methods once every case exists (a field may name its variant).
func (s *stage) fillCase(ic *Case, c *types.CaseType, d *syntax.VariantCase, variant string) {
	var items []syntax.RecordItem
	var owner check.Object
	if d != nil {
		owner = s.info.Defs[d.Name]
		n := nameOverrides(d.Annotations)
		ic.Go, ic.TS = n.goName, n.ts
		ic.Cpp = caseCpp(d.Annotations)
		if d.Body != nil {
			items = d.Body.Items
		}
	}
	ic.Fields = s.fields(c.Fields, items, owner, nil)
	ic.Methods = s.methods(c.Methods, items, variant+qnameSep+c.Name, c)
}

func caseDecl(items []syntax.VariantItem, name string) *syntax.VariantCase {
	for _, it := range items {
		if c, ok := it.(*syntax.VariantCase); ok && c.Name != nil && c.Name.Name == name {
			return c
		}
	}
	return nil
}

// dependent is the IR of a type function with a match: its discriminant, its non-Never arms in arm order, and each member's branch (CODEGEN.md §5.6, FINGERPRINT.md §4.4).
func (s *stage) dependent(fn *types.TypeFunc) *Dependent {
	if t, ok := s.named[fn]; ok {
		return t.(*Dependent)
	}
	sc := fn.Scrutinee
	out := &Dependent{Pkg: fn.Pkg, Name: fn.Name, Doc: fn.Doc, Params: len(fn.Params), DiscPath: wirePath(sc.Path), Disc: s.refPtr(sc.Type)}
	s.named[fn] = out
	s.depFns[out] = fn
	if sc.Param != nil {
		out.DiscParam = sc.Param.Index
	}
	s.nameDependent(out, fn)
	members := discMembers(sc.Type)
	out.ByMember, out.Branches = s.branches(fn, members)
	return out
}

// nameDependent fills a dependent type's name overrides from its declaration, if any.
func (s *stage) nameDependent(out *Dependent, fn *types.TypeFunc) {
	if fn.Decl == nil {
		return
	}
	s.decls[out] = s.site(fn.Decl.Name, fn.Decl.Name)
	n := nameOverrides(fn.Decl.Annotations)
	out.Go, out.Cpp, out.TS = n.goName, n.cpp, n.ts
}

// branches groups fn's arms by member (CODEGEN.md §5.6): each member's branch index (NoBranch for a Never arm or one it does not cover), and the branches themselves in arm order.
func (s *stage) branches(fn *types.TypeFunc, members []string) ([]int, []*Branch) {
	byArm := map[*types.TypeArm][]int{}
	for i := range members {
		if arm := fn.Arm(i); arm != nil {
			byArm[arm] = append(byArm[arm], i)
		}
	}
	byMember := make([]int, len(members))
	for i := range byMember {
		byMember[i] = NoBranch
	}
	var out []*Branch
	for _, arm := range fn.Arms {
		covered := byArm[arm]
		if len(covered) == 0 || arm.Result.Base().Kind() == types.Never {
			continue
		}
		first := covered[0]
		if !arm.Wildcard && len(arm.Members) > 0 {
			first = arm.Members[0]
		}
		for _, m := range covered {
			byMember[m] = len(out)
		}
		br := &Branch{Name: members[first], Members: covered, Type: s.ref(arm.Result)}
		s.branchTypes[br] = arm.Result
		out = append(out, br)
	}
	return byMember, out
}

// discMembers are the names of a discriminant's members: an enum's, or false and true.
func discMembers(t types.Type) []string {
	e, ok := t.Base().(*types.EnumType)
	if !ok {
		return []string{boolFalse, boolTrue}
	}
	out := make([]string, len(e.Members))
	for i, m := range e.Members {
		out[i] = m.Name
	}
	return out
}

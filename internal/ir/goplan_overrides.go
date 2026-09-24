package ir

// nameSite is one position that takes @go/@cpp/@ts(name:) (CODEGEN.md §3.5): the IR node it names, that node in messages, and its three overrides.
type nameSite struct {
	item                any
	origin              string
	goName, cpp, tsName string
}

// nameSites are override positions in declaration order.
type nameSites struct {
	sites []nameSite
}

// overrideSites are the override positions of p's own declarations, in declaration order:
// each type, its members or cases, their fields and export fns, then constants, values and
// package fns.
func overrideSites(p *Package) *nameSites {
	s := &nameSites{}
	for _, t := range p.Types {
		s.typ(t)
	}
	for _, c := range p.Consts {
		s.add(c, c.Name, c.Go, c.Cpp, c.TS)
	}
	for _, v := range p.Values {
		s.add(v, v.Name, v.Go, v.Cpp, v.TS)
	}
	for _, fn := range p.Fns {
		s.add(fn, fn.Name, fn.Go, fn.Cpp, fn.TS)
	}
	return s
}

func (s *nameSites) add(item any, origin string, g, c, t NameOptions) {
	s.sites = append(s.sites, nameSite{item: item, origin: origin, goName: g.Name, cpp: c.Name, tsName: t.Name})
}

func (s *nameSites) typ(t Type) {
	switch x := t.(type) {
	case *Record:
		s.add(x, x.QName(), x.Go, NameOptions{Name: x.Cpp.Name}, x.TS)
		s.body(x.QName(), x.Fields, x.Methods)
	case *Enum:
		s.add(x, x.QName(), x.Go, x.Cpp, x.TS)
		for _, m := range x.Members {
			s.add(m, x.QName()+qnameSep+m.Name, m.Go, m.Cpp, m.TS)
		}
	case *Variant:
		s.add(x, x.QName(), x.Go, x.Cpp, x.TS)
		for _, c := range x.Cases {
			s.add(c, x.QName()+qnameSep+c.Name, c.Go, NameOptions{Name: c.Cpp.Name}, c.TS)
			s.body(x.QName()+qnameSep+c.Name, c.Fields, c.Methods)
		}
	case *Dependent:
		s.add(x, x.QName(), x.Go, x.Cpp, x.TS)
	}
}

func (s *nameSites) body(owner string, fields []*Field, fns []*ExportFn) {
	for _, f := range fields {
		s.add(f, owner+qnameSep+f.Name, f.Go, NameOptions{Name: f.Cpp.Name}, f.TS)
	}
	for _, fn := range fns {
		s.add(fn, owner+qnameSep+fn.Name, fn.Go, fn.Cpp, fn.TS)
	}
}

// goOverrideProblems are p's @go(name:) overrides that are not exported identifiers (CODEGEN.md §1.3, §3.5, decision 182), in declaration order: every go emit checks them, whatever its mode (decision 203).
func goOverrideProblems(p *Package) []GoNameProblem {
	var out []GoNameProblem
	for _, site := range overrideSites(p).sites {
		if site.goName != "" && !goValidOverride(site.goName) {
			out = append(out, GoNameProblem{Kind: GoUnexported, Name: site.goName, Origin: site.origin, Item: site.item})
		}
	}
	return out
}

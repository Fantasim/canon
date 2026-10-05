package ir

// hookSite is one make hook a package writes (CODEGEN.md §5.14): a record's, a case's or its case type's, a dependent branch's; item and origin are what findings name, from the type its name is built on (decision 213).
type hookSite struct {
	origin   string
	item     any
	from     Type
	rec      *Record
	variant  *Variant
	c        *Case
	caseType bool
	dep      *Dependent
	branch   *Branch
}

// hookNamer names hooks in one target: GoNamePlan, CppNamePlan.
type hookNamer interface {
	MakeName(rec *Record) string
	MakeCaseName(v *Variant, c *Case) string
	MakeCaseTypeName(v *Variant, c *Case) string
	MakeBranchName(d *Dependent, b *Branch) string
}

// hookSites are p's hooks in declaration order: each record's, each case's then its case type's (a case with fields), each branch's that is not Never.
func hookSites(p *Package) []hookSite {
	var out []hookSite
	for _, t := range p.Types {
		switch x := t.(type) {
		case *Record:
			out = append(out, hookSite{origin: x.QName(), item: x, from: x, rec: x})
		case *Variant:
			out = append(out, caseHookSites(x)...)
		case *Dependent:
			for _, b := range x.Branches {
				out = append(out, hookSite{origin: x.QName() + qnameSep + b.Name, item: x, from: x, dep: x, branch: b})
			}
		}
	}
	return out
}

// caseHookSites are a variant's case hooks, each followed by its case-type hook when it has fields.
func caseHookSites(v *Variant) []hookSite {
	var out []hookSite
	for _, c := range v.Cases {
		site := hookSite{origin: v.QName() + qnameSep + c.Name, item: c, from: v, variant: v, c: c}
		out = append(out, site)
		if len(c.Fields) > 0 {
			site.caseType = true
			out = append(out, site)
		}
	}
	return out
}

// name is the site's hook as n names it.
func (s hookSite) name(n hookNamer) string {
	switch {
	case s.rec != nil:
		return n.MakeName(s.rec)
	case s.branch != nil:
		return n.MakeBranchName(s.dep, s.branch)
	case s.caseType:
		return n.MakeCaseTypeName(s.variant, s.c)
	}
	return n.MakeCaseName(s.variant, s.c)
}

// declareHooks declares, in sc, every hook of p as hn names it, each built on its type's name (CODEGEN.md §3.5, §5.14).
func (n *namer) declareHooks(sc *nameScope, p *Package, hn hookNamer) {
	for _, s := range hookSites(p) {
		n.declareHook(sc, s.origin, s.item, derivation{s.from, func() string { return s.name(hn) }})
	}
}

// declareHook is declareFrom for a make hook: a name a user's declaration of sc took first is E8005 at that declaration, the overriding one (CODEGEN.md §3.5); a hook meeting a hook or a fixed name is reported at the later hook.
func (n *namer) declareHook(sc *nameScope, origin string, item any, d derivation) {
	name := d.build()
	if n.builtOnRefused(name, item, d) {
		return
	}
	if first, taken := sc.names[name]; taken && !sc.hooks[name] && sc.items[name] != nil {
		n.collide(sc, name, origin, first, sc.items[name])
		return
	}
	n.declare(sc, name, origin, item)
	if sc.hooks == nil {
		sc.hooks = map[string]bool{}
	}
	sc.hooks[name] = true
}

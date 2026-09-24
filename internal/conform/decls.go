package conform

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// site is a translated fn with its declaration. owner names its record or case as messages
// do (`Potion`, `Reward.item`), "" for a package fn; label names the fn (`Potion.healFor`);
// selfFns is the owner's precomputed methods without parameters, by name.
type site struct {
	fn      *ir.ExportFn
	obj     check.Object
	sig     *types.FuncType
	decl    *syntax.FnDecl
	owner   string
	label   string
	pkg     string
	method  bool
	selfFns map[string]check.Object
}

// span is the fn's name in its declaration, where E9008 and E9009 point.
func (s *site) span() source.Span {
	return s.obj.File().Span(s.decl.Name)
}

// fnKey is an export fn by owner and name.
type fnKey struct {
	owner, name string
}

// translated is every translated fn of p: methods of its own types in order, then package fns.
// A fn the checker left broken is skipped: its findings are already reported.
func translated(prog *check.Program, p *ir.Package) ([]*site, error) {
	c := &collector{defs: declared(prog, p.Name), info: prog.Info, pkg: p.Name}
	for _, t := range p.Types {
		for _, m := range ownMethods(p.Name, t) {
			if err := c.add(m.owner, m.fns, c.selfFns(m)); err != nil {
				return nil, err
			}
		}
	}
	if err := c.add("", p.Fns, nil); err != nil {
		return nil, err
	}
	return c.sites, nil
}

// collector gathers the translated fns of one package.
type collector struct {
	defs  map[fnKey]*syntax.FnDecl
	info  *check.Info
	pkg   string
	sites []*site
}

// add adds the translated fns among fns, declared under owner, whose precomputed methods are selfFns.
func (c *collector) add(owner string, fns []*ir.ExportFn, selfFns map[string]check.Object) error {
	for _, fn := range fns {
		if fn.Kind != ir.FnTranslated {
			continue
		}
		s, err := newSite(c.defs, c.info, c.pkg, owner, fn)
		if err != nil {
			return err
		}
		if s != nil {
			s.selfFns = selfFns
			c.sites = append(c.sites, s)
		}
	}
	return nil
}

// selfFns is the declared object of each precomputed method of m without parameters, which a translated method reads like a field (CONFORMANCE.md §2.2).
func (c *collector) selfFns(m methodSet) map[string]check.Object {
	out := map[string]check.Object{}
	for _, fn := range m.fns {
		d := c.defs[fnKey{m.owner, fn.Name}]
		if fn.Kind != ir.FnPrecomputed || len(fn.Params) != 0 || d == nil {
			continue
		}
		if obj := c.info.Defs[d.Name]; obj != nil && !c.info.Broken[obj] {
			out[fn.Name] = obj
		}
	}
	return out
}

// methodSet is the export methods of one record or case.
type methodSet struct {
	owner string
	fns   []*ir.ExportFn
}

// ownMethods is the methods of a record, or of each case of a variant, declared in package pkg.
func ownMethods(pkg string, t ir.Type) []methodSet {
	switch x := t.(type) {
	case *ir.Record:
		if x.Pkg == pkg {
			return []methodSet{{owner: x.Name, fns: x.Methods}}
		}
	case *ir.Variant:
		if x.Pkg != pkg {
			return nil
		}
		out := make([]methodSet, len(x.Cases))
		for i, c := range x.Cases {
			out[i] = methodSet{owner: x.Name + ownerSep + c.Name, fns: c.Methods}
		}
		return out
	}
	return nil
}

// newSite is fn with its declaration, nil when the checker left it broken (TYPES.md §1).
func newSite(defs map[fnKey]*syntax.FnDecl, info *check.Info, pkg, owner string, fn *ir.ExportFn) (*site, error) {
	s := &site{fn: fn, owner: owner, label: fn.Name, pkg: pkg, method: owner != ""}
	if s.method {
		s.label = owner + ownerSep + fn.Name
	}
	s.decl = defs[fnKey{owner, fn.Name}]
	if s.decl == nil {
		return nil, fmt.Errorf(fmtNamed, ErrNoDecl, pkg+ownerSep+s.label)
	}
	s.obj = info.Defs[s.decl.Name]
	if s.obj == nil || info.Broken[s.obj] {
		return nil, nil
	}
	sig, ok := s.obj.Type().(*types.FuncType)
	if !ok || len(sig.Params) != len(fn.Params) {
		return nil, nil
	}
	s.sig = sig
	return s, nil
}

// declared is every fn declaration of package pkg: top level, and in record and case bodies.
func declared(prog *check.Program, pkg string) map[fnKey]*syntax.FnDecl {
	out := map[fnKey]*syntax.FnDecl{}
	for _, cp := range prog.Packages {
		if cp.Path != pkg {
			continue
		}
		for _, f := range cp.Files {
			for _, d := range f.Decls {
				declaredIn(out, d)
			}
		}
	}
	return out
}

func declaredIn(out map[fnKey]*syntax.FnDecl, d syntax.Decl) {
	switch x := d.(type) {
	case *syntax.FnDecl:
		if x.Name != nil {
			out[fnKey{"", x.Name.Name}] = x
		}
	case *syntax.RecordDecl:
		if x.Name != nil && x.Body != nil {
			bodyFns(out, x.Name.Name, x.Body.Items)
		}
	case *syntax.VariantDecl:
		for _, it := range x.Items {
			c, ok := it.(*syntax.VariantCase)
			if ok && x.Name != nil && c.Name != nil && c.Body != nil {
				bodyFns(out, x.Name.Name+ownerSep+c.Name.Name, c.Body.Items)
			}
		}
	}
}

func bodyFns(out map[fnKey]*syntax.FnDecl, owner string, items []syntax.RecordItem) {
	for _, it := range items {
		if d, ok := it.(*syntax.FnDecl); ok && d.Name != nil {
			out[fnKey{owner, d.Name.Name}] = d
		}
	}
}

// objects is the declared object of each site, in order.
func objects(sites []*site) []check.Object {
	out := make([]check.Object, len(sites))
	for i, s := range sites {
		out[i] = s.obj
	}
	return out
}

package check

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// studioPkg is the package project.studio names, nil when none is loaded (VIEWMODEL.md G16).
func (c *checker) studioPkg() *pkgState {
	if c.proj == nil {
		return nil
	}
	return c.pkgs[c.proj.Studio.Path]
}

// studioMember is the member name of the studio package's enum enumName (`Menu`, `Icon`,
// `Tone`), nil when there is none: E1610 is views'.
func (c *checker) studioMember(enumName, name string) *object {
	_, e := c.studioEnum(enumName)
	if e == nil {
		return nil
	}
	return c.memberObject(e, name)
}

// studioEnum is the studio package's enum enumName and the name declaring it, nil when there is
// none.
func (c *checker) studioEnum(enumName string) (*object, *types.EnumType) {
	sp := c.studioPkg()
	if sp == nil {
		return nil, nil
	}
	o := sp.names[enumName]
	if o == nil || o.kind != ObjTypeName {
		return nil, nil
	}
	e, _ := c.resolveTypeName(o).(*types.EnumType)
	if e == nil {
		return nil, nil
	}
	return o, e
}

// studioProp is `icon` or `tone`: a studio enum's member, bare or qualified (VIEWMODEL.md §3.5, G16).
func studioProp(enumName string) func(*checker, *viewCtx, syntax.Expr) {
	return func(c *checker, vc *viewCtx, e syntax.Expr) {
		switch x := e.(type) {
		case *syntax.IdentExpr:
			if o := c.studioMember(enumName, x.Name); o != nil {
				c.info.Uses[x] = o
				c.info.Types[x] = o.typ
				c.deprecatedUse(vc.env, x, o)
			}
			return
		case *syntax.SelectorExpr:
			if c.studioQualified(vc.env, enumName, x) {
				return
			}
		}
		if _, en := c.studioEnum(enumName); en != nil {
			c.expr(vc.env, e, en)
		}
	}
}

// studioQualified resolves `Icon.gem` in the studio, else false; an unknown member is views' E1610 (VIEWMODEL.md §3.5).
func (c *checker) studioQualified(env *env, enumName string, s *syntax.SelectorExpr) bool {
	q, ok := s.X.(*syntax.IdentExpr)
	if !ok || s.Optional || q.Name != enumName {
		return false
	}
	o, en := c.studioEnum(enumName)
	if en == nil {
		return false
	}
	c.info.Uses[q], c.info.Types[q] = o, en
	if m := c.memberObject(en, s.Name.Name); m != nil {
		c.info.NameUses[s.Name] = m
		c.info.Types[s] = en
		c.deprecatedUse(env, s.Name, m)
	}
	return true
}

// unitProp is `unit`, a ref into the studio's `units`: a name or string key, else typed (VIEWMODEL.md §16).
func (c *checker) unitProp(vc *viewCtx, e syntax.Expr) {
	sp := c.studioPkg()
	if sp == nil {
		return
	}
	let := sp.names[syntax.StudioUnits]
	if let == nil || let.kind != ObjLet || let.keys == nil {
		return
	}
	elem, keyed, isColl := collectionElem(c.letType(let))
	if !isColl {
		return
	}
	ref := &types.RefType{Target: c.internLet(let, nil, elem, keyed)}
	switch x := e.(type) {
	case *syntax.IdentExpr:
		if u := let.keys.byName[x.Name]; u != nil {
			c.info.Uses[x] = u
			c.info.Types[x] = ref
			c.deprecatedUse(vc.env, x, u)
		}
		return
	case syntax.StrLit:
		if u := let.keys.byName[constText(x)]; u != nil {
			c.deprecatedUse(vc.env, x, u)
		}
		return
	}
	c.expr(vc.env, e, ref)
}

// widgetProp is `widget`: a widget of the studio package, typed by its `value` parameter.
func (c *checker) widgetProp(_ *viewCtx, e syntax.Expr) {
	id, ok := e.(*syntax.IdentExpr)
	sp := c.studioPkg()
	if !ok || sp == nil {
		return
	}
	if o := sp.names[id.Name]; o != nil && o.kind == ObjWidget {
		c.info.Uses[id] = o
		c.info.Types[id] = o.typ
	}
}

// menuAnnotation resolves `@menu(m, icon: i)` on a let against the studio (VIEWMODEL.md G23, G16).
func (c *checker) menuAnnotation(p *pkgState, f *syntax.File, l *syntax.LetDecl) {
	a := annotation(l.Annotations, syntax.AnnMenu)
	if a == nil {
		return
	}
	env := c.fileEnv(p, f, nil)
	for _, n := range []struct {
		v    syntax.AnnValue
		enum string
	}{{firstArg(a), syntax.StudioMenu}, {named(a, syntax.PropIcon), syntax.StudioIcon}} {
		if q, ok := n.v.(*syntax.QualifiedName); ok {
			c.studioName(env, n.enum, q)
		}
	}
}

// studioName resolves a studio member, a WORD (GRAMMAR.md studio{Enum}); an unknown one is views' E1610.
func (c *checker) studioName(env *env, enumName string, q *syntax.QualifiedName) {
	if len(q.Parts) != 1 {
		return
	}
	if m := c.studioMember(enumName, q.Parts[0].Name); m != nil {
		c.info.NameUses[q.Parts[0]] = m
		c.deprecatedUse(env, q.Parts[0], m)
	}
}

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
	sp := c.studioPkg()
	if sp == nil {
		return nil
	}
	o := sp.names[enumName]
	if o == nil || o.kind != ObjTypeName {
		return nil
	}
	e, ok := c.resolveTypeName(o).(*types.EnumType)
	if !ok {
		return nil
	}
	return c.memberObject(e, name)
}

// studioProp is the `icon` or `tone` property: a member of the studio enum enumName.
func studioProp(enumName string) func(*checker, *viewCtx, syntax.Expr) {
	return func(c *checker, _ *viewCtx, e syntax.Expr) {
		id, ok := e.(*syntax.IdentExpr)
		if !ok {
			return
		}
		if o := c.studioMember(enumName, id.Name); o != nil {
			c.info.Uses[id] = o
			c.info.Types[id] = o.typ
		}
	}
}

// unitProp is `unit`: an entry of the studio package's `units` table, a ref into it.
func (c *checker) unitProp(_ *viewCtx, e syntax.Expr) {
	id, ok := e.(*syntax.IdentExpr)
	sp := c.studioPkg()
	if !ok || sp == nil {
		return
	}
	let := sp.names[studioUnits]
	if let == nil || let.kind != ObjLet || let.keys == nil || let.keys.byName[id.Name] == nil {
		return
	}
	elem, keyed, isColl := collectionElem(c.letType(let))
	if !isColl {
		return
	}
	c.info.Uses[id] = let.keys.byName[id.Name]
	c.info.Types[id] = &types.RefType{Target: c.internLet(let, nil, elem, keyed)}
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

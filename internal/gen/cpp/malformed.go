package cppgen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// validate refuses an IR stage E cannot produce before anything reads it, so the generator
// returns ErrMalformed instead of panicking: a nil item, a composite type without its parts, a
// named kind without its declaration, a keyed list without its key field.
func (g *gen) validate() {
	for _, t := range g.p.Types {
		g.validateType(t)
	}
	for _, c := range g.p.Consts {
		if c == nil {
			g.malformed(nilItem, g.p.Name)
			continue
		}
		g.checkType(c.Type, c.Name)
	}
	for _, v := range g.p.Values {
		if v == nil {
			g.malformed(nilItem, g.p.Name)
			continue
		}
		g.checkType(v.Type, v.Name)
	}
	g.checkFns(g.p.Fns, g.p.Name)
}

func (g *gen) malformed(what, at string) {
	g.fail(fmt.Errorf("%w: %s at %s", ErrMalformed, what, at))
}

func (g *gen) validateType(t ir.Type) {
	switch x := t.(type) {
	case *ir.Record:
		g.checkFields(x.Fields, x.Name)
		g.checkFns(x.Methods, x.Name)
	case *ir.Variant:
		for _, c := range x.Cases {
			if c == nil {
				g.malformed(nilItem, x.Name)
				continue
			}
			g.checkFields(c.Fields, x.Name+qnameSep+c.Name)
			g.checkFns(c.Methods, x.Name+qnameSep+c.Name)
		}
	case *ir.Dependent:
		g.checkDependent(x)
	case *ir.Enum:
	default:
		g.malformed(nilItem, g.p.Name)
	}
}

func (g *gen) checkFields(fields []*ir.Field, at string) {
	for _, f := range fields {
		if f == nil {
			g.malformed(nilItem, at)
			continue
		}
		g.checkType(f.Type, at+qnameSep+f.Name)
	}
}

func (g *gen) checkFns(fns []*ir.ExportFn, at string) {
	for _, fn := range fns {
		if fn == nil {
			g.malformed(nilItem, at)
			continue
		}
		where := at + qnameSep + fn.Name
		g.checkType(fn.Result, where)
		for _, p := range fn.Params {
			if p == nil {
				g.malformed(nilItem, where)
				continue
			}
			g.checkType(p.Type, where)
		}
		for _, r := range fn.Reads {
			if r == nil {
				g.malformed(nilItem, where)
				continue
			}
			g.checkType(r.Type, where)
		}
	}
}

// checkType checks t and the types it is made of.
func (g *gen) checkType(t ir.TypeRef, at string) {
	if what := malformedType(t); what != "" {
		g.malformed(what, at)
		return
	}
	for _, part := range []*ir.TypeRef{t.Elem, t.Key} {
		if part != nil {
			g.checkType(*part, at)
		}
	}
}

// malformedType names what is missing from t, or "".
func malformedType(t ir.TypeRef) string {
	switch {
	case (t.Kind == types.List || t.Kind == types.Optional || t.Kind == types.Table) && t.Elem == nil:
		return noElem
	case t.Kind == types.Map && (t.Key == nil || t.Elem == nil), t.Kind == types.Ref && t.Key == nil:
		return noKey
	case !namedAs(t):
		return noDecl
	case t.KeyedBy != nil:
		return keyFieldMissing(t)
	default:
		return ""
	}
}

// namedAs reports that a record, variant, enum or type-application kind names a declaration of that kind.
func namedAs(t ir.TypeRef) bool {
	switch t.Kind {
	case types.Record:
		_, ok := t.Named.(*ir.Record)
		return ok
	case types.Variant:
		_, ok := t.Named.(*ir.Variant)
		return ok
	case types.Enum:
		_, ok := t.Named.(*ir.Enum)
		return ok
	case types.TypeApp:
		_, ok := t.Named.(*ir.Dependent)
		return ok
	default:
		return true
	}
}

func keyFieldMissing(t ir.TypeRef) string {
	if t.Elem == nil {
		return noElem
	}
	rec, ok := t.Elem.Named.(*ir.Record)
	if !ok || fieldNamed(rec.Fields, t.KeyedBy.Name) == nil {
		return noKeyField
	}
	return ""
}

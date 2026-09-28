package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// codeTargets are the emits that write types (TYPES.md §3.7).
var codeTargets = []string{TargetGo, TargetCpp, TargetTS}

// letSelection is the public lets a code emit of a package writes: all of them, or those named.
type letSelection struct {
	all   bool
	names map[string]bool
}

func (s letSelection) has(name string) bool { return s.all || s.names[name] }

// checkExposure is E2111 for each emitted public declaration of p exposing a local type (TYPES.md §3.7).
func (c *checker) checkExposure(p *pkgState) {
	sel := emittedLets(p)
	for _, o := range p.all {
		if o.local || o.kind == ObjMethod { // a method is exposed with its record or variant
			continue
		}
		env := c.declEnv(o)
		for _, t := range exposedTypes(o, sel) {
			c.exposes(env, o, t)
		}
	}
}

// emittedLets is what the code emits of p that hold values select (CODEGEN.md §2.1 `values`).
func emittedLets(p *pkgState) letSelection {
	sel := letSelection{names: map[string]bool{}}
	for _, f := range p.files {
		for _, d := range f.Decls {
			if e, ok := d.(*syntax.EmitDecl); ok && holdsValues(e) {
				sel.add(e)
			}
		}
	}
	return sel
}

// holdsValues reports a go, cpp or ts emit that is not in `types` mode (CODEGEN.md §2.2).
func holdsValues(e *syntax.EmitDecl) bool {
	if e.Target == nil || e.Options == nil || !slices.Contains(codeTargets, e.Target.Name) {
		return false
	}
	opt := emitOption(e, OptMode)
	if opt == nil {
		return true
	}
	mode, ok := opt.(*syntax.IdentExpr) // a malformed mode selects none: its E8009 stands alone
	return ok && mode.Name != ModeTypes && slices.Contains(codeModes, mode.Name)
}

// add records the lets e selects: every public one without `values` or with `values: []`; none
// for a `values` that is not a list (E8009 says it).
func (s *letSelection) add(e *syntax.EmitDecl) {
	opt := emitOption(e, OptValues)
	list, ok := opt.(*syntax.ListLit)
	if opt == nil || ok && len(list.Elems) == 0 {
		s.all = true
		return
	}
	if !ok {
		return
	}
	for _, x := range list.Elems {
		if id, isName := x.(*syntax.IdentExpr); isName {
			s.names[id.Name] = true
		}
	}
}

// emitOption is the value of the option name of e, nil when absent.
func emitOption(e *syntax.EmitDecl, name string) syntax.Expr {
	for _, it := range e.Options.Items {
		if fi, ok := it.(*syntax.FieldItem); ok && fi.Name != nil && fi.Name.Name == name {
			return fi.Value
		}
	}
	return nil
}

// exposedTypes are the type expressions a code emit writes for a public declaration (TYPES.md §3.7).
func exposedTypes(o *object, sel letSelection) []syntax.Type {
	switch d := o.decl.(type) {
	case *syntax.RecordDecl:
		return bodyTypes(d.Body)
	case *syntax.VariantDecl:
		return variantTypes(d)
	case *syntax.TypeDecl:
		return []syntax.Type{d.Type}
	case *syntax.FnDecl:
		if d.Mods != nil && d.Mods.Export.Valid() {
			return signatureTypes(d)
		}
	case *syntax.LetDecl:
		if sel.has(o.name) {
			return []syntax.Type{d.Type}
		}
	}
	return nil
}

// variantTypes are the field types of a variant's cases and its methods' signatures.
func variantTypes(d *syntax.VariantDecl) []syntax.Type {
	var out []syntax.Type
	for _, it := range d.Items {
		switch it := it.(type) {
		case *syntax.VariantCase:
			out = append(out, bodyTypes(it.Body)...)
		case *syntax.FnDecl:
			out = append(out, signatureTypes(it)...)
		}
	}
	return out
}

// bodyTypes are the field types of a record body and its methods' signatures.
func bodyTypes(b *syntax.RecordBody) []syntax.Type {
	if b == nil {
		return nil
	}
	var out []syntax.Type
	for _, it := range b.Items {
		switch it := it.(type) {
		case *syntax.FieldDecl:
			out = append(out, it.Type)
		case *syntax.FnDecl:
			out = append(out, signatureTypes(it)...)
		}
	}
	return out
}

// signatureTypes are a function's parameter types and its result type.
func signatureTypes(fn *syntax.FnDecl) []syntax.Type {
	out := make([]syntax.Type, 0, len(fn.Params)+1)
	for _, p := range fn.Params {
		out = append(out, p.Type)
	}
	return append(out, fn.Result)
}

// exposes reports each type expression in t naming a local type, a ref's target aside (TYPES.md §10.2).
func (c *checker) exposes(env *env, o *object, t syntax.Type) {
	if t == nil {
		return
	}
	syntax.Inspect(t, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.RefType, *syntax.TypeArgs, *syntax.Pattern, syntax.Expr:
			return false
		case *syntax.NamedType:
			c.localNamed(env, o, x, x.Name)
		case *syntax.TableType:
			c.localNamed(env, o, x, x.Name)
		}
		return true
	})
}

// checkConstExposure is E2111 at the name of each public const whose inferred type holds a local type (TYPES.md §3.7).
func (c *checker) checkConstExposure(p *pkgState) {
	for _, o := range p.all {
		d, ok := o.decl.(*syntax.ConstDecl)
		if !ok || o.local {
			continue
		}
		if named := c.localIn(c.constType(o)); named != nil {
			env := c.declEnv(o)
			c.report(env, diag.E2111.At(env.span(d.Name), o.name, named.name))
		}
	}
}

// localIn is the first local record, variant or enum in t, through optionals, lists and maps.
func (c *checker) localIn(t types.Type) *object {
	switch x := t.(type) {
	case *types.OptionalType:
		return c.localIn(x.Elem)
	case *types.ListType:
		return c.localIn(x.Elem)
	case *types.MapType:
		if o := c.localIn(x.Key); o != nil {
			return o
		}
		return c.localIn(x.Value)
	case *types.CaseType:
		return c.localIn(x.Variant)
	}
	if o := c.typeObjects[t]; o != nil && o.local {
		return o
	}
	return nil
}

// localNamed is E2111 at at when q names a local record, variant, enum or alias.
func (c *checker) localNamed(env *env, o *object, at syntax.Node, q *syntax.QualifiedName) {
	if q == nil || len(q.Parts) == 0 {
		return
	}
	named, ok := c.info.NameUses[q.Parts[0]].(*object)
	if !ok || !named.local || named.kind != ObjTypeName {
		return
	}
	c.report(env, diag.E2111.At(env.span(at), o.name, named.name))
}

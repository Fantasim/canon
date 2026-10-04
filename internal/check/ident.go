package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// ident is a bare name in value position (TYPES.md §3.3).
func (c *checker) ident(env *env, e *syntax.IdentExpr, want types.Type) types.Type {
	if want != nil {
		if t, ok := c.contextual(env, e, want); ok {
			return t
		}
	}
	if fn := depFunc(unwrapUnion(want)); fn != nil {
		return c.dependentIdent(env, e, want, fn)
	}
	o := c.lookup(env, e.Name)
	if o == nil && e.Name == itName && env.it != nil {
		o = env.it
	}
	if o == nil {
		return c.unresolved(env, e, want)
	}
	if coll := c.dynamicKeys(want); coll != nil && !c.valueOf(o, want) && !c.optionalOf(o, want) {
		c.info.Keys[e] = coll
		return refTo(want)
	}
	return c.use(env, e, o)
}

// unresolved is a name no scope has: a dynamic key (§4.1, §11.4), silent against an error type (TYPES.md §1).
func (c *checker) unresolved(env *env, e *syntax.IdentExpr, want types.Type) types.Type {
	if coll := c.dynamicKeys(want); coll != nil {
		c.info.Keys[e] = coll
		return refTo(want)
	}
	if e.Name == itName {
		c.report(env, diag.E2109.At(env.span(e)))
		return types.ErrorType
	}
	if want == nil || want.Base().Kind() != types.Error {
		c.unknownName(env, e, e.Name)
	}
	return types.ErrorType
}

// dynamicKeys is the collection of an expected ref whose keys are not known statically.
func (c *checker) dynamicKeys(want types.Type) *types.Collection {
	coll := c.keyTarget(want)
	if coll == nil || c.staticKeys(coll) != nil {
		return nil
	}
	return coll
}

// valueOf reports that o is a value assignable to want (a ref with dynamic keys, §4.1).
func (c *checker) valueOf(o *object, want types.Type) bool {
	switch o.kind {
	case ObjLocal, ObjParam, ObjField, ObjConst, ObjLet:
		t := o.typ
		if o.kind == ObjLet {
			t = c.letType(o)
		}
		return t != nil && c.assignable(t, want)
	default:
		return false
	}
}

// optionalOf reports that o is a value of the expected ref made optional: the name, E3403 where it is accepted (DECISIONS 303).
func (c *checker) optionalOf(o *object, want types.Type) bool {
	if _, opt := want.Base().(*types.OptionalType); opt {
		return false
	}
	return c.valueOf(o, &types.OptionalType{Elem: refTo(want)})
}

// staticKeys are the keys of a collection known statically (TYPES.md §4.1).
func (c *checker) staticKeys(coll *types.Collection) *entryKeys {
	if coll.Kind != types.CollLet || len(coll.FieldPath) > 0 {
		return nil
	}
	p := c.pkgs[coll.Pkg]
	if p == nil {
		return nil
	}
	if o := p.names[coll.Name]; o != nil && o.kind == ObjLet {
		return o.keys
	}
	return nil
}

// contextual is step 1 (TYPES.md §4.1).
func (c *checker) contextual(env *env, e *syntax.IdentExpr, want types.Type) (types.Type, bool) {
	o, t := c.inExpected(unwrapUnion(want), e.Name)
	if o == nil {
		return nil, false
	}
	c.ambiguity(env, e, e.Name, o, t)
	c.info.Uses[e] = o
	c.deprecatedUse(env, e, o)
	if o.kind == ObjCase && t.Kind() == types.Case {
		c.bareCase(env, e, o)
	}
	return t, true
}

// unwrapUnion is unwrap, then a literal union's alternative A.
func unwrapUnion(t types.Type) types.Type {
	u := unwrap(t)
	if l, ok := u.(*types.LitUnionType); ok {
		return unwrap(l.Of)
	}
	return u
}

// inExpected is the member, case or static entry named name in the expected type t.
func (c *checker) inExpected(t types.Type, name string) (*object, types.Type) {
	switch x := t.(type) {
	case *types.EnumType:
		if m := c.memberObject(x, name); m != nil {
			return m, x
		}
	case *types.VariantKindType:
		if cs := c.caseObject(x.Variant, name); cs != nil {
			return cs, x
		}
	case *types.VariantType:
		if cs := c.caseObject(x, name); cs != nil {
			return cs, cs.typ
		}
	case *types.CaseType:
		if cs := c.caseObject(x.Variant, name); cs != nil {
			return cs, cs.typ
		}
	case *types.RefType:
		if keys := c.staticKeys(c.coll(x)); keys != nil && keys.byName[name] != nil {
			return keys.byName[name], x
		}
	}
	return nil, nil
}

// ambiguity is E2101 (TYPES.md §4.2); it reports whether it fired.
func (c *checker) ambiguity(env *env, n syntax.Node, name string, o *object, t types.Type) bool {
	other := env.lookupLocal(name)
	if other == nil {
		other = env.lookupRecord(name)
	}
	if other == nil || other.typ == nil || other.kind == ObjMethod || !c.assignable(other.typ, t) {
		return false
	}
	kind := diag.KindLocal
	switch other.kind {
	case ObjParam:
		kind = diag.KindParameter
	case ObjField:
		kind = diag.KindField
	default:
	}
	c.report(env, diag.E2101.At(env.span(n), name, qualifiedMember(o), kind))
	return true
}

// qualifiedMember is the qualified form of a contextual name: `Icon.columns`, `statuses.open`.
func qualifiedMember(o *object) string {
	switch t := o.owner.(type) {
	case *types.EnumType:
		return t.Name + dot + o.name
	case *types.VariantType:
		return t.Name + dot + o.name
	}
	if o.kind == ObjEntry && o.parent != nil {
		return o.parent.name + dot + o.name
	}
	return o.name
}

// bareCase is a case written without fields: E3302 when a field is required (TYPES.md §8.2, §5.2).
func (c *checker) bareCase(env *env, e syntax.Node, o *object) {
	ct := o.typ.(*types.CaseType)
	for _, f := range ct.Fields {
		if required(f) {
			c.report(env, diag.E3302.At(env.span(e), ct, f.Name))
			return
		}
	}
}

// use is the type of a name found by steps 2 to 6, per the kind of what it names.
func (c *checker) use(env *env, e *syntax.IdentExpr, o *object) types.Type {
	c.info.Uses[e] = o
	c.dependsOn(env, o)
	if !c.allowedIn(env, e, o) {
		return types.ErrorType
	}
	switch o.kind {
	case ObjLocal, ObjParam:
		return c.narrowed(env, e, staticView(o.typ))
	case ObjField:
		return c.fieldRead(env, e, o)
	case ObjConst:
		return c.constType(o)
	case ObjLet:
		return c.narrowed(env, e, staticView(c.letType(o)))
	case ObjFn:
		c.textUse(env, e, o, false)
		return o.typ
	case ObjMethod:
		c.report(env, diag.E3016.At(env.span(e), diag.KindMethod, o.name))
	case ObjBuiltin:
		if o.typ != nil {
			c.report(env, diag.E2110.AtType(env.span(e), o.name))
		} else {
			c.report(env, diag.E3016.At(env.span(e), diag.KindBuiltin, o.name))
		}
	default:
		c.report(env, diag.E2110.AtType(env.span(e), o.name))
	}
	return types.ErrorType
}

// fieldRead is a field named in its record body, `self.f`: E3313 for an input (TYPES.md §14).
func (c *checker) fieldRead(env *env, e syntax.Expr, o *object) types.Type {
	if o.field.Input != nil {
		c.report(env, diag.E3313.At(env.span(e), o.name))
		return types.ErrorType
	}
	c.deprecatedUse(env, e, o)
	return c.narrowed(env, e, staticView(o.field.Type))
}

// deprecatedUse is W3301 for a deprecated field, member or entry named in Canon source.
func (c *checker) deprecatedUse(env *env, n syntax.Node, o *object) {
	var d *types.Deprecation
	switch o.kind {
	case ObjField:
		d = o.field.Deprecated
	case ObjMember:
		d = o.member.Deprecated
	case ObjCase:
		d = o.typ.(*types.CaseType).Deprecated
	case ObjEntry:
		d = c.entryDeprecation(o)
	default:
	}
	if d == nil {
		return
	}
	if d.Why == "" {
		c.warn(env, diag.W3301.AtPlain(env.span(n), o.name))
		return
	}
	c.warn(env, diag.W3301.AtReason(env.span(n), o.name, d.Why))
}

// entryDeprecation is the `@deprecated` of a table entry or an `entry` declaration.
func (c *checker) entryDeprecation(o *object) *types.Deprecation {
	var anns []*syntax.Annotation
	switch d := o.decl.(type) {
	case *syntax.EntryItem:
		anns = d.Annotations
	case *syntax.EntryDecl:
		anns = d.Annotations
	}
	if why, ok := c.deprecation(anns); ok {
		return &types.Deprecation{Why: why}
	}
	return nil
}

// allowedIn is E3015 in a constant expression and E3010 in a field default (TYPES.md §15): what each may name.
func (c *checker) allowedIn(env *env, e syntax.Node, o *object) bool {
	switch {
	case env.what != noConstant && !constName(env, o):
		c.report(env, diag.E3015.AtNotConstant(env.span(e), env.what))
		return false
	case env.fields >= 0 && !defaultName(env, o):
		c.report(env, diag.E3010.At(env.span(e)))
		return false
	}
	return true
}

// constName reports a name a constant expression may use: a constant, a built-in, or a
// binder of the expression itself (a lambda parameter, a comprehension variable).
func constName(env *env, o *object) bool {
	switch o.kind {
	case ObjConst, ObjBuiltin:
		return true
	case ObjLocal, ObjParam:
		return env.inner(o)
	default:
	}
	return false
}

// defaultName reports a name a field default may use: a constant, an earlier field, a record
// parameter, a built-in, or a binder of the default itself.
func defaultName(env *env, o *object) bool {
	switch o.kind {
	case ObjConst, ObjBuiltin:
		return true
	case ObjField:
		return o.field.Index < env.fields && env.rec.fields[o.name] == o
	case ObjParam:
		return env.params[o.name] == o || env.inner(o)
	case ObjLocal:
		return env.inner(o)
	default:
	}
	return false
}

// inner reports a local declared inside the expression env.base began: it is found before
// the scope that was innermost there.
func (env *env) inner(o *object) bool {
	for s := env.scope; s != nil && s != env.base; s = s.parent {
		if s.names[o.name] == o {
			return true
		}
	}
	return false
}

// constant is env checking a constant expression of what (E3015).
func (env *env) constant(what diag.Kind) *env {
	e := env.with()
	e.what = what
	e.base = env.scope
	return e
}

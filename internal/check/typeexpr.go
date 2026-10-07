package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// typeCtx is where a written type appears: what it may hold and what encloses it.
type typeCtx struct {
	env    *env
	pos    typePos
	encl   types.Type             // the record or case whose field type this is (TYPES.md §10.2 level 1)
	scope  map[string]typeArgRoot // the roots a type argument may name (TYPES.md §11.1)
	later  map[string]bool        // the fields declared after the one typed (E3805)
	fnBody bool                   // the body of a type function: no refinement may use a parameter
	scrut  bool                   // a type-level match's scrutinee: E3803 names it (TYPES.md §11.2)
}

// typePos is a set of flags: the kinds of types a position allows.
type typePos uint8

// resolveType resolves a written type and records it in TypeExprs (TYPES.md §1 step 2).
func (c *checker) resolveType(tc *typeCtx, t syntax.Type) types.Type {
	r := c.resolveTypeNode(tc, t)
	if r == nil {
		r = types.ErrorType
	}
	r = c.pastType(tc, t, r)
	c.info.TypeExprs[t] = r
	return r
}

func (c *checker) resolveTypeNode(tc *typeCtx, t syntax.Type) types.Type {
	switch t := t.(type) {
	case *syntax.NamedType:
		return c.resolveNamed(tc, t)
	case *syntax.ListType:
		return c.resolveList(tc, t)
	case *syntax.KeyedType:
		return c.resolveKeyed(tc, t)
	case *syntax.MapType:
		return c.resolveMap(tc, t)
	case *syntax.DepMapType:
		return c.resolveDepMap(tc, t)
	case *syntax.TableType:
		return c.resolveTable(tc, t)
	case *syntax.RefType:
		return c.newRef(tc, t)
	case *syntax.OptionalType:
		return c.resolveOptional(tc, t)
	case *syntax.WhereType:
		return c.resolveWhere(tc, t)
	case *syntax.UnionType:
		return c.resolveUnion(tc, t)
	case *syntax.AssetType:
		return c.resolveAsset(tc, t)
	case *syntax.FnType:
		return c.resolveFnType(tc, t)
	case *syntax.ParenType:
		return c.resolveType(tc, t.Type)
	case *syntax.AnyType:
		return c.resolveAny(tc, t)
	case *syntax.BadType:
		c.breakObj(tc.env.owner)
		return types.ErrorType
	}
	return c.misplacedType(tc, t)
}

// misplacedType is E3002 for a misplaced literal or type-level match; a later union alternative's literal is a String (TYPES.md §13.2).
func (c *checker) misplacedType(tc *typeCtx, t syntax.Type) types.Type {
	if l, ok := t.(*syntax.LiteralType); ok {
		c.info.Types[l.Value] = types.StringType
		if tc.pos&posLiteral != 0 {
			return types.StringType
		}
	}
	c.report(tc.env, diag.E3002.At(tc.env.span(t), types.AnyType, types.StringType))
	return types.ErrorType
}

// resolveNamed resolves `Name[(args)]`: a type, a refinement or an application (GRAMMAR.md §6.6).
func (c *checker) resolveNamed(tc *typeCtx, t *syntax.NamedType) types.Type {
	o := c.typeName(tc.env, t.Name)
	if o == nil {
		return types.ErrorType
	}
	c.dependsOn(tc.env, o)
	base := c.typeOfName(tc.env, o, t.Name)
	if base == nil || base.Kind() == types.Error {
		return base
	}
	if fn := c.typeFuncs[o]; fn != nil {
		return c.applyTypeFunc(tc, t, fn)
	}
	// A record may still be a shell (TYPES.md §3.2): its declaration says whether it takes parameters.
	if rec, ok := base.(*types.RecordType); ok && rec.Decl != nil && len(rec.Decl.Params) > 0 {
		return c.applyRecord(tc, t, rec)
	}
	if t.Args == nil {
		return base
	}
	return c.refine(tc, base, t.Args)
}

// typeName resolves the qualified name of a type (TYPES.md §3.3, type position).
func (c *checker) typeName(env *env, q *syntax.QualifiedName) *object {
	first := q.Parts[0]
	o := c.lookupType(env, first.Name)
	if o == nil {
		c.unknownName(env, first, first.Name)
		return nil
	}
	c.info.NameUses[first] = o
	for _, part := range q.Parts[1:] {
		next := c.member(env, o, part)
		if next == nil {
			return nil
		}
		c.info.NameUses[part] = next
		o = next
	}
	return o
}

// member resolves `.part` after a package or a variant in a qualified type name.
func (c *checker) member(env *env, o *object, part *syntax.Ident) *object {
	switch o.kind {
	case ObjPackage:
		if m, ok := o.target.names[part.Name]; ok && !m.local {
			return m
		}
		c.report(env, diag.E2004.At(env.span(part), o.target.path, part.Name))
		return nil
	case ObjTypeName:
		if v, ok := c.typeOfName(env, o, nil).(*types.VariantType); ok {
			if cs := c.caseObject(v, part.Name); cs != nil {
				return cs
			}
			c.report(env, diag.E3003.At(env.span(part), v, diag.KindCase, part.Name))
			return nil
		}
	default:
	}
	c.report(env, diag.E2110.AtValue(env.span(part), o.name))
	return nil
}

// typeOfName is the type an object names in type position: E2110 for a value.
func (c *checker) typeOfName(env *env, o *object, n *syntax.QualifiedName) types.Type {
	switch o.kind {
	case ObjTypeName:
		return c.resolveTypeName(o)
	case ObjCase:
		return o.typ
	case ObjBuiltin:
		if o.typ != nil {
			return o.typ
		}
	default:
	}
	if n != nil {
		c.report(env, diag.E2110.AtValue(env.span(n), o.name))
	}
	return nil
}

// typeOfNameOrNil is the type o names, nil when o is nil or names no type.
func (c *checker) typeOfNameOrNil(env *env, o *object) types.Type {
	if o == nil {
		return nil
	}
	return c.typeOfName(env, o, nil)
}

// resolveList is `[T][(length)]`.
func (c *checker) resolveList(tc *typeCtx, t *syntax.ListType) types.Type {
	l := &types.ListType{Elem: c.resolveType(tc.element(), t.Elem)}
	if t.Args == nil {
		return l
	}
	return c.refine(tc, l, t.Args)
}

// resolveKeyed is `[T] keyed by f`, f a key-typed field of the record T, `R(args)` included (TYPES.md §2, §9.1).
func (c *checker) resolveKeyed(tc *typeCtx, t *syntax.KeyedType) types.Type {
	inner := c.resolveType(tc, t.List)
	l, ok := listOf(inner)
	if !ok || l.Elem.Base().Kind() == types.Error {
		return inner
	}
	rec := requiredRecord(l.Elem)
	if rec == nil {
		c.report(tc.env, diag.E3012.AtField(tc.env.span(t.Key), t.Key.Name))
		return inner
	}
	c.completeRecord(rec)
	f := fieldNamed(rec.Fields, t.Key.Name)
	if f == nil {
		c.report(tc.env, diag.E3012.AtField(tc.env.span(t.Key), t.Key.Name))
		return inner
	}
	if fo := c.fieldObjects[f]; fo != nil {
		c.info.NameUses[t.Key] = fo
	}
	if ft := c.fieldType(f); !keyable(ft) {
		c.report(tc.env, diag.E3012.AtType(tc.env.span(t.Key), t.Key.Name, ft))
		return inner
	}
	l.KeyedBy = f
	c.keyedOf[rec] = true
	c.refKey(tc.env, t.Key, c.fieldType(f))
	return inner
}

// listOf is the list under a written list type's refinement.
func listOf(t types.Type) (*types.ListType, bool) {
	for {
		switch x := t.(type) {
		case *types.ListType:
			return x, true
		case *types.Refined:
			t = x.Of
		default:
			return nil, false
		}
	}
}

// keyable reports a key field type: String, an integer type, an enum or a ref, refined or not.
func keyable(t types.Type) bool {
	switch t.Base().Kind() {
	case types.String, types.Int, types.Enum, types.Ref:
		return true
	default:
		return false
	}
}

func fieldNamed(fields []*types.Field, name string) *types.Field {
	for _, f := range fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// resolveMap is `{K: V}[(length)]`; K must be a key type (E3011, TYPES.md §9.2).
func (c *checker) resolveMap(tc *typeCtx, t *syntax.MapType) types.Type {
	k := c.resolveType(tc.element(), t.Key)
	m := &types.MapType{Key: k, Value: c.resolveType(tc.element(), t.Value)}
	if !mapKey(k) {
		c.report(tc.env, diag.E3011.At(tc.env.span(t.Key), k))
	}
	if t.Args == nil {
		return m
	}
	return c.refine(tc, m, t.Args)
}

// mapKey reports a map key type: String, an integer type, an enum, a ref, a Kind, a literal
// union over one of these, or a dependent value.
func mapKey(t types.Type) bool {
	b := t.Base()
	switch b.Kind() {
	case types.String, types.Int, types.Enum, types.Ref, types.VariantKind, types.DepUnion, types.TypeApp, types.Error:
		return true
	case types.LitUnion:
		return mapKey(b.(*types.LitUnionType).Of)
	default:
		return false
	}
}

// resolveTable is `[stable] table T`, T a record; an element in error loses @stable's table (TYPES.md §9.3, LOCK.md §1).
func (c *checker) resolveTable(tc *typeCtx, t *syntax.TableType) types.Type {
	rec, elem := c.tableElement(tc, t)
	if rec == nil {
		c.stableLost[tc.env.pkg] = c.stableLost[tc.env.pkg] || t.Stable.Valid()
		return types.ErrorType
	}
	c.tableOf[rec] = true
	stable := t.Stable.Valid()
	if stable {
		c.stableOf[rec] = true
	}
	if stable && tc.pos&posStable == 0 {
		c.report(tc.env, diag.E6003.AtTable(tc.env.span(t)))
	}
	return &types.TableType{Elem: elem, Stable: stable}
}

// tableElement is a table's record, else E3013, or E3806 for a parameterized one; nil after a finding (TYPES.md §9.3, §11.1).
func (c *checker) tableElement(tc *typeCtx, t *syntax.TableType) (*types.RecordType, types.Type) {
	o := c.typeName(tc.env, t.Name)
	if o == nil {
		return nil, nil
	}
	c.dependsOn(tc.env, o)
	elem := c.typeOfName(tc.env, o, t.Name)
	if elem == nil {
		return nil, nil
	}
	rec, ok := elem.Base().(*types.RecordType)
	if !ok {
		c.report(tc.env, diag.E3013.At(tc.env.span(t.Name), elem))
		return nil, nil
	}
	if rec.Decl != nil && len(rec.Decl.Params) > 0 {
		c.completeRecord(rec)
		c.wrongArity(tc.env, t.Name, rec.Name, rec.Params)
		return nil, nil
	}
	return rec, elem
}

// optionalJob is a `T??` whose E3401 is judged once the refs T holds have their targets.
type optionalJob struct {
	env  *env
	at   *syntax.OptionalType
	elem types.Type
}

// resolveOptional is `T?`; `T??` is E3401 and recovers to `T?`, the evident intent (TYPES.md §2).
func (c *checker) resolveOptional(tc *typeCtx, t *syntax.OptionalType) types.Type {
	elem := c.resolveType(tc.inner(), t.Elem)
	if elem.Base().Kind() != types.Optional {
		return &types.OptionalType{Elem: elem}
	}
	j := optionalJob{env: tc.env, at: t, elem: elem}
	if c.pendingIn(elem) { // a pending ref reads as the error type until its target is found
		c.optionals = append(c.optionals, j)
	} else {
		c.doubleOptional(j)
	}
	return elem
}

// doubleOptional is E3401 for a `T??`, unless T holds an error: `Nope??` has only Nope's finding (TYPES.md §1).
func (c *checker) doubleOptional(j optionalJob) {
	if !holdsError(j.elem) {
		c.report(j.env, diag.E3401.At(j.env.span(j.at), j.elem))
	}
}

// checkOptionals judges the `T??` written in p's types, its refs now resolved.
func (c *checker) checkOptionals(p *pkgState) {
	drainPkgJobs(&c.optionals, p, func(j optionalJob) *pkgState { return j.env.pkg }, c.doubleOptional)
}

// pendingIn reports a ref in t whose target is not found yet (TYPES.md §10.2).
func (c *checker) pendingIn(t types.Type) bool {
	if t == nil {
		return false
	}
	if r, ok := t.Base().(*types.RefType); ok && c.pending[r] != nil {
		return true
	}
	return slices.ContainsFunc(typeParts(t.Base()), c.pendingIn)
}

// resolveFnType is `fn(T, …) -> R`, allowed only for a parameter or a local let or var; elsewhere E3306 and the error type (TYPES.md §1).
func (c *checker) resolveFnType(tc *typeCtx, t *syntax.FnType) types.Type {
	in := tc.inner()
	in.pos |= posFn
	f := &types.FuncType{Result: c.resolveType(in, t.Result)}
	for _, p := range t.Params {
		f.Params = append(f.Params, c.resolveType(in, p))
	}
	if tc.pos&posFn == 0 {
		c.report(tc.env, diag.E3306.At(tc.env.span(t)))
		return types.ErrorType
	}
	return f
}

// resolveAny is `_`, valid only in a widget parameter type (TYPES.md §13.6).
func (c *checker) resolveAny(tc *typeCtx, t *syntax.AnyType) types.Type {
	if tc.pos&posAny == 0 {
		c.report(tc.env, diag.E3002.At(tc.env.span(t), types.AnyType, types.AnyType))
		return types.ErrorType
	}
	return types.AnyType
}

// inner is the context of a component type: `stable` is no longer the whole type.
func (tc *typeCtx) inner() *typeCtx {
	in := *tc
	in.pos &^= posStable
	return &in
}

// element is the context of an element of a list, map or union: no function type (E3306).
func (tc *typeCtx) element() *typeCtx {
	in := tc.inner()
	in.pos &^= posFn
	return in
}

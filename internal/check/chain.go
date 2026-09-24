package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// chainTop types a postfix chain (TYPES.md §6.5).
func (c *checker) chainTop(env *env, e syntax.Expr, want types.Type) types.Type {
	t, opt := c.link(env, e, want)
	if !opt {
		return t
	}
	switch t.Base().Kind() {
	case types.Optional, types.None, types.Error:
		return t
	default:
	}
	return &types.OptionalType{Elem: t}
}

// link types one suffix of a chain: its type with every `?.` before it unwrapped, and whether
// the chain has one so far.
func (c *checker) link(env *env, e syntax.Expr, want types.Type) (types.Type, bool) {
	switch x := e.(type) {
	case *syntax.SelectorExpr:
		return c.selector(env, x)
	case *syntax.IndexExpr:
		return c.index(env, x)
	case *syntax.CallExpr:
		return c.call(env, x, want)
	case *syntax.ForceExpr:
		return c.force(env, x)
	}
	return c.synth(env, e), false
}

// recv types a receiver: a link of the same chain keeps its `?.`; any other expression (a
// parenthesized chain included) ends the chain.
func (c *checker) recv(env *env, x syntax.Expr) (types.Type, bool) {
	switch x.(type) {
	case *syntax.SelectorExpr, *syntax.IndexExpr, *syntax.CallExpr, *syntax.ForceExpr:
		t, opt := c.link(env, x, nil)
		c.info.Types[x] = t
		return t, opt
	}
	return c.synth(env, x), false
}

// optionalRecv applies a suffix's rule to its receiver: `?.` unwraps (W3401 on a value never
// none), anything else on a T? is E3402. The result is false after a finding.
func (c *checker) optionalRecv(env *env, x syntax.Expr, t types.Type, optDot bool, op string) (types.Type, bool) {
	o, isOpt := t.Base().(*types.OptionalType)
	switch {
	case optDot && isOpt:
		return o.Elem, true
	case optDot:
		c.warn(env, diag.W3401.At(env.span(x), env.span(x), op))
	case isOpt || t.Base().Kind() == types.None:
		c.report(env, diag.E3402.At(env.span(x), env.span(x)))
		return types.ErrorType, false
	}
	return t, true
}

// selector is `x.name` or `x?.name` (TYPES.md §3.5).
func (c *checker) selector(env *env, s *syntax.SelectorExpr) (types.Type, bool) {
	if q := c.qualifier(env, s.X); q != nil {
		return c.qualifiedValue(env, s, q), false
	}
	var t types.Type
	opt := false
	if s.X == nil {
		t = env.shorthand
	} else {
		t, opt = c.recv(env, s.X)
	}
	if t.Kind() == types.Error {
		return t, opt
	}
	t, ok := c.optionalRecv(env, s.X, t, s.Optional, optDotText)
	if !ok {
		return types.ErrorType, opt
	}
	sel := c.memberOf(env, s, t)
	if sel == nil {
		return types.ErrorType, opt || s.Optional
	}
	return c.narrowed(env, s, sel), opt || s.Optional
}

// memberOf resolves `.name` on a receiver of type t, recording the selection.
func (c *checker) memberOf(env *env, s *syntax.SelectorExpr, t types.Type) types.Type {
	recv, deref := t, false
	if r, ok := t.Base().(*types.RefType); ok {
		recv, deref = c.coll(r).Elem, true
	}
	name := s.Name.Name
	sel, typ := c.selectOn(recv, name)
	if sel == nil && recv.Kind() != types.Error {
		c.unknownMember(env, s, recv)
	}
	if sel == nil {
		return nil
	}
	sel.Recv, sel.Deref = t, deref
	if c.reservedRead(sel, t, s.X, name) {
		typ = types.ErrorType
	}
	if sel.Kind == SelEntry && s.X != nil {
		sel.Obj = c.staticEntry(s.X, name)
		if coll := c.receiverColl(s.X); sel.Obj == nil && coll != nil {
			c.info.Keys[s] = coll
		}
	}
	c.info.Selections[s] = sel
	if o, isObj := sel.Obj.(*object); isObj && o != nil {
		c.info.NameUses[s.Name] = o
		c.memberUse(env, s, o)
	}
	return typ
}

// reservedRead reports a table element's field named id or retired (E2105) read where it may be
// the entry's pseudo-field: on a table entry, or on any value when no keyed list holds the record.
func (c *checker) reservedRead(sel *Selection, t types.Type, x syntax.Expr, name string) bool {
	rec := requiredRecord(c.deref(t))
	if sel.Kind != SelField || rec == nil || !c.tableOf[rec] || !reservedOnEntries(name) {
		return false
	}
	return !c.keyedOf[rec] || c.tableEntry(t, x)
}

// tableEntry reports a receiver known to be a table's entry: a ref into a table, or an entry
// selected from one.
func (c *checker) tableEntry(t types.Type, x syntax.Expr) bool {
	if r, ok := t.Base().(*types.RefType); ok {
		return c.coll(r).KeyedBy == nil
	}
	s, ok := inner(x).(*syntax.SelectorExpr)
	if !ok {
		return false
	}
	sel := c.info.Selections[s]
	return sel != nil && sel.Kind == SelEntry && c.deref(sel.Recv).Base().Kind() == types.Table
}

// memberUse reports what reading a member may: an input field (E3313), a deprecated field or
// entry (W3301), a dependent value's field (E3804).
func (c *checker) memberUse(env *env, s *syntax.SelectorExpr, o *object) {
	if o.kind == ObjField && o.field.Input != nil {
		c.report(env, diag.E3313.At(env.span(s.Name), o.name))
		return
	}
	c.deprecatedUse(env, s.Name, o)
}

// selectOn is the member name of a value of type t: its selection and type, nil when it has
// none.
func (c *checker) selectOn(t types.Type, name string) (*Selection, types.Type) {
	switch x := t.Base().(type) {
	case *types.RecordType:
		return c.recordMember(x, name)
	case *types.AppliedRecord:
		return c.recordMember(x.Rec, name)
	case *types.CaseType:
		if sel, st := c.fieldMember(x.Fields, name); sel != nil {
			return sel, st
		}
		return c.builtinMember(name, kindMember, &types.VariantKindType{Variant: x.Variant})
	case *types.VariantType:
		return c.builtinMember(name, kindMember, &types.VariantKindType{Variant: x})
	case *types.EnumType:
		return c.enumBuiltin(x, name)
	case *types.TableType:
		return entryMember(x.Elem)
	case *types.ListType:
		if x.KeyedBy != nil && x.KeyedBy.Type.Base().Kind() == types.String {
			return entryMember(x.Elem)
		}
	}
	if t.Kind() == types.Range && (name == startMember || name == endMember) {
		return c.builtinMember(name, name, types.IntType)
	}
	return nil, nil
}

// recordMember is a field, else `id` or `retired` on a record used as a table's element.
func (c *checker) recordMember(r *types.RecordType, name string) (*Selection, types.Type) {
	c.completeRecord(r)
	if sel, t := c.fieldMember(r.Fields, name); sel != nil {
		return sel, t
	}
	if !c.tableOf[r] {
		return nil, nil
	}
	switch name {
	case idMember:
		return c.builtinMember(name, idMember, types.StringType)
	case retiredMember:
		return c.builtinMember(name, retiredMember, types.BoolType)
	}
	return nil, nil
}

func (c *checker) fieldMember(fields []*types.Field, name string) (*Selection, types.Type) {
	f := fieldNamed(fields, name)
	if f == nil {
		return nil, nil
	}
	return &Selection{Kind: SelField, Obj: c.fieldObjects[f]}, staticView(f.Type)
}

// builtinMember is the built-in member name when it is want, typed t.
func (c *checker) builtinMember(name, want string, t types.Type) (*Selection, types.Type) {
	if name != want {
		return nil, nil
	}
	return &Selection{Kind: SelBuiltinMember, Obj: c.builtins[name]}, t
}

// enumBuiltin is `.name`, `.index`, `.wire`, and `.code` with `@codes` (TYPES.md §8.1).
func (c *checker) enumBuiltin(e *types.EnumType, name string) (*Selection, types.Type) {
	switch name {
	case nameMember, wireMember:
		return c.builtinMember(name, name, types.StringType)
	case indexMember:
		return c.builtinMember(name, name, types.IntType)
	case codeMember:
		if e.Codes != nil {
			return c.builtinMember(name, name, types.IntType)
		}
	}
	return nil, nil
}

// entryMember is the entry of a table or a String-keyed list (TYPES.md §3.5); a missing key is E4002 at evaluation.
func entryMember(elem types.Type) (*Selection, types.Type) {
	return &Selection{Kind: SelEntry}, elem
}

// staticEntry is the object of the entry `x.name` when x names a let whose keys are known
// statically; nil otherwise.
func (c *checker) staticEntry(x syntax.Expr, name string) Object {
	id, ok := x.(*syntax.IdentExpr)
	if !ok {
		return nil
	}
	o, isObj := c.info.Uses[id].(*object)
	if !isObj || o.kind != ObjLet || o.keys == nil || o.keys.byName[name] == nil {
		return nil
	}
	return o.keys.byName[name]
}

// unknownMember is E3003, E3804 on a dependent value, E3016 for a method used as a value (TYPES.md §12.3).
func (c *checker) unknownMember(env *env, s *syntax.SelectorExpr, t types.Type) {
	kind := diag.KindField
	switch t.Base().Kind() {
	case types.DepUnion:
		c.report(env, diag.E3804.At(env.span(s.Name), dot+s.Name.Name, depName(t)))
		return
	case types.Enum:
		kind = diag.KindMember
	default:
	}
	switch name := s.Name.Name; {
	case c.userMethod(t, name) != nil:
		c.report(env, diag.E3016.At(env.span(s.Name), diag.KindMethod, name))
	case len(c.methodRows(t, nil, name, newBinding())) > 0:
		c.report(env, diag.E3016.At(env.span(s.Name), diag.KindBuiltin, name))
	default:
		c.report(env, diag.E3003.At(env.span(s.Name), t, kind, name))
	}
}

// depName is the type function of a dependent value, for E3804.
func depName(t types.Type) string {
	switch d := t.Base().(type) {
	case *types.DepUnionType:
		return d.Fn.Name
	case *types.TypeAppType:
		return d.Fn.Name
	default:
		return ""
	}
}

// force is `x!` (TYPES.md §6.5): x's present type; W3401 when x is never none.
func (c *checker) force(env *env, f *syntax.ForceExpr) (types.Type, bool) {
	t, opt := c.recv(env, f.X)
	switch t.Base().Kind() {
	case types.Error:
		return t, opt
	case types.Optional:
		return t.Base().(*types.OptionalType).Elem, opt
	case types.None:
		return types.NeverType, opt
	default:
	}
	c.warn(env, diag.W3401.At(env.span(f.X), env.span(f.X), forceText))
	return t, opt
}

package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// viewKey is a view's target: a record, variant, case or enum type, or a define-table let.
type viewKey struct {
	typ types.Type
	let *object
}

// viewCtx is a view being checked: its expressions' scope, what its items name, its templates.
type viewCtx struct {
	env     *env
	field   *object            // the field of the member item whose properties are checked
	body    *recordCtx         // a record, case or define-table target
	fields  []*types.Field     // the target's fields, searched for case fields (§3.3 rule 3)
	variant *types.VariantType // a variant target
	enum    *types.EnumType    // an enum target
	texts   map[string]bool    // title, subtitle, show.<id>
	unnamed int                // the unnamed show lines so far (VIEWMODEL.md G17)
}

// ViewBroken reports whether d holds an error: the mark, or a parser recovery node (ADR-0009).
func ViewBroken(info *Info, d *syntax.ViewDecl) bool {
	if info.BrokenViews[d] {
		return true
	}
	bad := false
	syntax.Inspect(d, func(n syntax.Node) bool {
		switch n.(type) {
		case *syntax.BadExpr, *syntax.BadDecl, *syntax.BadType, *syntax.BadStmt:
			bad = true
		}
		return !bad
	})
	return bad
}

// checkErrorTypedViews marks a view broken when one of its expressions names a broken object.
func (c *checker) checkErrorTypedViews() {
	for _, p := range c.sorted {
		for _, f := range p.files {
			c.checkErrorTypedViewsIn(f)
		}
	}
}

// checkErrorTypedViewsIn is checkErrorTypedViews for one file's own view declarations.
func (c *checker) checkErrorTypedViewsIn(f *syntax.File) {
	if f.FileKind != syntax.FileSource {
		return
	}
	for _, d := range f.Decls {
		if vd, ok := d.(*syntax.ViewDecl); ok {
			c.markIfErrorTyped(vd)
		}
	}
}

// markIfErrorTyped marks d once, on its first error-typed expression or reference to a broken one.
func (c *checker) markIfErrorTyped(d *syntax.ViewDecl) {
	if c.info.BrokenViews[d] {
		return
	}
	syntax.Inspect(d, func(n syntax.Node) bool {
		if c.namesBroken(n) {
			c.info.BrokenViews[d] = true
		}
		return !c.info.BrokenViews[d]
	})
}

// namesBroken reports n as error-typed, or naming a broken declaration bare, qualified or called.
func (c *checker) namesBroken(n syntax.Node) bool {
	if e, ok := n.(syntax.Expr); ok && isErrorTyped(c.info.Types[e]) {
		return true
	}
	switch x := n.(type) {
	case *syntax.IdentExpr:
		return c.info.Broken[c.info.Uses[x]]
	case *syntax.SelectorExpr:
		return c.info.Broken[c.info.ObjectOf(x.Name)]
	case *syntax.CallExpr:
		callee := c.info.Calls[x]
		return callee != nil && c.info.Broken[callee.Obj]
	}
	return false
}

// isErrorTyped reports the error type (TYPES.md §1).
func isErrorTyped(t types.Type) bool { return t != nil && t.Kind() == types.Error }

// checkPresentation resolves and types every view, then every translation file (DECISIONS 221).
func (c *checker) checkPresentation() {
	if c.ctx.Err() != nil {
		return
	}
	for _, p := range c.order {
		c.checkViews(p)
	}
	for _, p := range c.order {
		c.checkTranslations(p)
	}
}

// checkViews checks the views and `@menu` annotations of p's source files in path and source order.
func (c *checker) checkViews(p *pkgState) {
	for _, f := range p.files {
		if f.FileKind != syntax.FileSource {
			continue
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *syntax.ViewDecl:
				c.checkView(p, f, d)
			case *syntax.LetDecl:
				c.menuAnnotation(p, f, d)
			}
		}
	}
}

// checkView resolves a view's target and checks its items; E2102, E2110 stop it (VIEWMODEL.md §3.2).
func (c *checker) checkView(p *pkgState, f *syntax.File, d *syntax.ViewDecl) {
	prev := c.curView
	c.curView = d
	defer func() { c.curView = prev }()
	if c.holdsSyntaxError(d) {
		c.info.BrokenViews[d] = true
	}
	env := c.fileEnv(p, f, nil)
	o := c.lookupGlobal(env, d.Type.Name)
	if o == nil {
		c.unknownName(env, d.Type, d.Type.Name)
		return
	}
	c.info.NameUses[d.Type] = o
	vc, key := c.viewTarget(env, d, o)
	if vc == nil {
		return
	}
	if _, dup := c.views[key]; !dup {
		c.views[key] = vc
	}
	for _, it := range d.Items {
		c.viewItem(vc, it)
	}
}

// viewTarget is the checked view of the type or let o, and its key; nil for another target: a
// function or constant is E2110, a let that is no define table views' E1626.
func (c *checker) viewTarget(env *env, d *syntax.ViewDecl, o *object) (*viewCtx, viewKey) {
	switch o.kind {
	case ObjTypeName:
		return c.typeView(env.pkg, env.file, d, o)
	case ObjLet:
		if d.Case == nil && definesTable(c.letType(o)) {
			return c.defineView(env.pkg, env.file, d), viewKey{let: o}
		}
	case ObjFn, ObjConst:
		c.report(env, diag.E2110.AtValue(env.span(d.Type), o.name))
	default:
	}
	return nil, viewKey{}
}

// definesTable reports the type of a `load.defines` table (VIEWMODEL.md §3.2).
func definesTable(t types.Type) bool {
	tt, ok := t.Base().(*types.TableType)
	return ok && tt.Elem == types.DefineType
}

// typeView is the view of a record, a variant, a variant's case or an enum.
func (c *checker) typeView(p *pkgState, f *syntax.File, d *syntax.ViewDecl, o *object) (*viewCtx, viewKey) {
	t := viewedType(c.resolveTypeName(o))
	if d.Case != nil {
		return c.caseView(c.fileEnv(p, f, nil), d, t)
	}
	switch x := t.(type) {
	case *types.RecordType:
		c.completeRecord(x)
		vc := c.newView(p, f, d, x, c.bodyOf(x))
		vc.fields = x.Fields
		return vc, viewKey{typ: x}
	case *types.VariantType:
		c.completeVariant(x)
		vc := c.newView(p, f, d, x, nil)
		vc.variant = x
		return vc, viewKey{typ: x}
	case *types.EnumType:
		vc := c.newView(p, f, d, x, nil)
		vc.enum = x
		return vc, viewKey{typ: x}
	}
	return nil, viewKey{}
}

// viewedType is t, or the record, variant or enum an alias names (log-2026-09-28 U5 round 2); nil
// for another type, whose view is views' E1626.
func viewedType(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	switch b := t.Base().(type) {
	case *types.RecordType, *types.VariantType, *types.EnumType:
		return b
	}
	return nil
}

// caseView is `view V.c`, a case of V; another name is E3003, t in error aside (VIEWMODEL.md §3.2).
func (c *checker) caseView(env *env, d *syntax.ViewDecl, t types.Type) (*viewCtx, viewKey) {
	var co *object
	if v, ok := t.(*types.VariantType); ok {
		co = c.caseObject(v, d.Case.Name)
	}
	if co == nil {
		if t != nil {
			c.report(env, diag.E3003.At(env.span(d.Case), t, diag.KindCase, d.Case.Name))
		}
		return nil, viewKey{}
	}
	c.info.NameUses[d.Case] = co
	ct := co.typ.(*types.CaseType)
	vc := c.newView(env.pkg, env.file, d, ct, c.caseBodies[ct])
	vc.fields = ct.Fields
	return vc, viewKey{typ: ct}
}

// defineView is the view of a define table: `id`, `value` and the package (VIEWMODEL.md G7).
func (c *checker) defineView(p *pkgState, f *syntax.File, d *syntax.ViewDecl) *viewCtx {
	body := &recordCtx{self: types.DefineType, fields: map[string]*object{}, methods: map[string]*object{}}
	for _, fd := range types.DefineType.Fields {
		fo := c.fieldObjects[fd]
		body.fields[fd.Name] = fo
		body.order = append(body.order, fo)
	}
	env := c.fileEnv(p, f, nil)
	env.rec = body
	env = env.push()
	vc := &viewCtx{env: env, body: body, fields: types.DefineType.Fields, texts: map[string]bool{}}
	vc.env.magic = map[string]*object{idMember: c.magicName(p, f, d, idMember, types.StringType)}
	return vc
}

// newView is the view of target: its body (nil for a variant or an enum) and magic names in scope.
func (c *checker) newView(p *pkgState, f *syntax.File, d *syntax.ViewDecl, target types.Type, body *recordCtx) *viewCtx {
	vc := &viewCtx{body: body, texts: map[string]bool{}}
	if body == nil {
		body = &recordCtx{self: target, fields: map[string]*object{}, methods: map[string]*object{}}
	}
	vc.env = c.bodyEnv(p, f, nil, body)
	vc.env.magic = c.magicNames(p, f, d, target)
	return vc
}

// memberName is what a member item names: a field, a method, a case field, a case, a member
// of the target; nil when none, E1602 being views' (VIEWMODEL.md G8).
func (c *checker) memberName(vc *viewCtx, name string) *object {
	switch {
	case vc.enum != nil:
		return c.memberObject(vc.enum, name)
	case vc.variant != nil:
		return c.caseObject(vc.variant, name)
	case vc.body == nil:
		return nil
	}
	if o := vc.body.fields[name]; o != nil {
		return o
	}
	if o := vc.body.methods[name]; o != nil {
		return o
	}
	return c.inlineField(vc.fields, name, map[*types.VariantType]bool{})
}

// columnName is what a column or filter names: as a member item, but a variant's case fields
// (VIEWMODEL.md T6a).
func (c *checker) columnName(vc *viewCtx, name string) *object {
	if vc.variant != nil {
		return c.casesField(vc.variant, name, map[*types.VariantType]bool{})
	}
	if vc.enum != nil {
		return nil
	}
	return c.memberName(vc, name)
}

// filterName is what a filter names: a column's, or a variant view's `kind`, the case filter (VIEWMODEL.md T6a).
func (c *checker) filterName(vc *viewCtx, name string) *object {
	if vc.variant != nil && name == kindMember {
		return c.builtins[kindMember]
	}
	return c.columnName(vc, name)
}

// inlineField is the first field named name of a case of an `@json(inline)` variant field, depth first.
func (c *checker) inlineField(fields []*types.Field, name string, seen map[*types.VariantType]bool) *object {
	for _, f := range fields {
		v, ok := f.Type.Base().(*types.VariantType)
		if !f.Inline || !ok {
			continue
		}
		if o := c.casesField(v, name, seen); o != nil {
			return o
		}
	}
	return nil
}

// casesField is the first field named name of v's cases, in case order, nested inline variants
// searched depth first; seen stops a variant inlining itself.
func (c *checker) casesField(v *types.VariantType, name string, seen map[*types.VariantType]bool) *object {
	if seen[v] {
		return nil
	}
	seen[v] = true
	c.completeVariant(v)
	for _, ct := range v.Cases {
		if f := fieldNamed(ct.Fields, name); f != nil {
			return c.fieldObjects[f]
		}
		if o := c.inlineField(ct.Fields, name, seen); o != nil {
			return o
		}
	}
	return nil
}

package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// checkPackage is TYPES.md §1 step 3 for p.
func (c *checker) checkPackage(p *pkgState) {
	c.bodies = true
	for _, o := range p.all {
		c.checkDecl(o)
	}
	c.checkListKeys(p)
	c.checkEmits(p)
	c.checkLayers(p)
	c.checkDocs(p)
	c.checkInputs(p)
	var rest []whereJob
	for _, j := range c.wheres {
		if j.env.pkg == p {
			c.checkWhere(j)
		} else {
			rest = append(rest, j)
		}
	}
	c.wheres = rest
}

// markBodyTables records the elements of tables written in bodies first, so E2105 is order-free (TYPES.md §3.6).
func (c *checker) markBodyTables() {
	for _, p := range c.sorted {
		for _, f := range p.files {
			env := c.fileEnv(p, f, nil)
			syntax.Inspect(f, func(n syntax.Node) bool {
				if t, ok := n.(*syntax.TableType); ok && c.info.TypeExprs[t] == nil {
					c.markTable(env, t)
				}
				return true
			})
		}
	}
}

// markTable records the record a table type names, reporting nothing: the body reports.
func (c *checker) markTable(env *env, t *syntax.TableType) {
	o := c.lookupType(env, t.Name.Parts[0].Name)
	for _, part := range t.Name.Parts[1:] {
		if o == nil || o.kind != ObjPackage {
			return
		}
		m, ok := o.target.names[part.Name]
		if !ok || m.local {
			return
		}
		o = m
	}
	if o == nil || o.kind != ObjTypeName || c.resolveTypeName(o) == nil {
		return
	}
	if rec, ok := c.resolveTypeName(o).Base().(*types.RecordType); ok {
		c.tableOf[rec] = true
		c.stableOf[rec] = c.stableOf[rec] || t.Stable.Valid()
	}
}

// checkDecl checks the body of one declaration.
func (c *checker) checkDecl(o *object) {
	switch o.kind {
	case ObjConst:
		c.constType(o)
	case ObjLet:
		c.letInit(o)
	case ObjFn:
		c.fnBody(c.declEnv(o), o)
	case ObjMethod:
		c.methodBody(o)
	case ObjTypeName:
		c.typeBody(o)
	case ObjCheck:
		c.checkBody(o)
	case ObjTest:
		c.testBody(o)
	case ObjEntry:
		c.entryDecl(o)
	default:
	}
}

// letInit checks a top-level let's initializer against its annotation, or synthesizes it.
func (c *checker) letInit(o *object) {
	if c.initDone[o] {
		return
	}
	d := o.decl.(*syntax.LetDecl)
	if d.Type == nil {
		c.letType(o)
		return
	}
	c.initDone[o] = true
	c.expr(c.declEnv(o).storing(o), d.Value, c.letType(o))
}

// fnBody checks a function's parameter defaults (constant, E3015) and body: every path that
// completes normally must return (E3006). The parameters share the body's top scope (E2107).
func (c *checker) fnBody(env *env, o *object) {
	fn := o.decl.(*syntax.FnDecl)
	ft, _ := o.typ.(*types.FuncType)
	if ft == nil || fn.Body == nil {
		return
	}
	pe := env.push()
	pe.fn = &fnCtx{name: o.name, result: ft.Result}
	for i, po := range c.fnParams[o] {
		p := fn.Params[i]
		if p.Default != nil {
			c.expr(pe.constant(diag.KindParameterDefault), p.Default, ft.Params[i])
		}
		if _, dup := pe.scope.names[po.name]; !dup {
			pe.scope.names[po.name] = po
		}
	}
	if c.stmtsIn(pe, fn.Body) {
		c.report(env, diag.E3006.At(env.span(fn.Name), o.name, ft.Result))
	}
}

// stmtsIn checks a block's statements in env's own scope.
func (c *checker) stmtsIn(env *env, b *syntax.Block) bool {
	completes := true
	be := env
	for _, s := range b.Stmts {
		var ok bool
		be, ok = c.stmt(be, s)
		completes = completes && ok
	}
	return completes
}

// methodBody checks a method in its record or case body, `self` its value (TYPES.md §3.3, §12.1).
func (c *checker) methodBody(o *object) {
	env := c.bodyEnv(c.pkgs[o.pkg], o.file, o, o.body)
	c.fnBody(env, o)
}

// bodyEnv is the env of a record or case body: its fields and methods, and the record's
// parameters in scope.
func (c *checker) bodyEnv(p *pkgState, f *syntax.File, owner *object, body *recordCtx) *env {
	env := c.fileEnv(p, f, owner)
	env.rec = body
	env = env.push()
	if r, ok := body.self.(*types.RecordType); ok && len(r.Decl.Params) > 0 {
		env.params = map[string]*object{}
		for _, po := range c.recordParams(r) {
			env.scope.names[po.name] = po
			env.params[po.name] = po
		}
	}
	return env
}

// recordParams are the parameter objects of a record, from its declaration.
func (c *checker) recordParams(r *types.RecordType) []*object {
	var out []*object
	for _, p := range r.Decl.Params {
		if po, ok := c.info.Defs[p.Name].(*object); ok {
			out = append(out, po)
		}
	}
	return out
}

// typeBody checks what a record or variant declares beyond its types: field defaults, the
// reserved names of table elements (E2105), and the cases' defaults.
func (c *checker) typeBody(o *object) {
	env := c.declEnv(o)
	switch t := o.typ.(type) {
	case *types.RecordType:
		c.fieldDefaults(o, o.body)
		c.reservedEntryNames(o, t)
		c.stableFields(o, t)
		c.wireBody(env, o.body)
	case *types.VariantType:
		for _, co := range c.cases[t] {
			c.fieldDefaults(o, co.body)
			c.wireBody(env, co.body)
		}
		c.checkTag(env, t)
	}
}

// wireBody checks the wire keys of a record or case body (WIRE.md §4.2, §5.14).
func (c *checker) wireBody(env *env, body *recordCtx) {
	if body == nil {
		return
	}
	c.checkWireKeys(env, body)
	c.checkInline(env, body)
	c.checkPairsDefault(env, body)
}

// fieldDefaults checks each default against its field's type, with E3010's limits (TYPES.md §15).
func (c *checker) fieldDefaults(o *object, body *recordCtx) {
	if body == nil {
		return
	}
	for _, fo := range body.order {
		d := fo.decl.(*syntax.FieldDecl)
		if d.Default == nil || fo.field.Input != nil {
			continue
		}
		env := c.bodyEnv(c.pkgs[o.pkg], fo.file, o, body)
		env.fields = fo.field.Index
		c.expr(env.storing(fo), d.Default, fo.field.Type)
	}
}

// stableFields is E6003 for `@stable` off a stable table's element or on another type (LOCK.md §1).
func (c *checker) stableFields(o *object, r *types.RecordType) {
	env := c.declEnv(o)
	for _, fo := range o.body.order {
		if !fo.field.Stable {
			continue
		}
		at := env.span(annotation(fo.decl.(*syntax.FieldDecl).Annotations, annotStable))
		switch k := fo.field.Type.Base().Kind(); {
		case !c.stableOf[r]:
			c.report(env, diag.E6003.AtField(at))
		case k != types.Int && k != types.String:
			c.report(env, diag.E6003.AtType(at, fo.field.Type))
		}
	}
}

// reservedEntryNames is E2105: a table's element may not declare `id` or `retired`.
func (c *checker) reservedEntryNames(o *object, r *types.RecordType) {
	if !c.tableOf[r] || o.body == nil {
		return
	}
	env := c.declEnv(o)
	for _, it := range r.Decl.Body.Items {
		var n *syntax.Ident
		switch it := it.(type) {
		case *syntax.FieldDecl:
			n = it.Name
		case *syntax.FnDecl:
			n = it.Name
		}
		if n != nil && (n.Name == idMember || n.Name == retiredMember) {
			c.report(env, diag.E2105.AtEntry(env.span(n), n.Name))
		}
	}
}

// checkBody checks a check or warn: a block, or a condition and its message under F (TYPES.md §6.6).
func (c *checker) checkBody(o *object) {
	d := o.decl.(*syntax.CheckDecl)
	env := c.declEnv(o)
	if body := c.checkOwnerBody(o); body != nil {
		env = c.bodyEnv(env.pkg, o.file, o, body)
	}
	if d.Body != nil {
		c.block(env, d.Body)
		return
	}
	_, ff := c.cond(env, d.Cond)
	if d.At != nil {
		c.checkAt(env, d.At)
	}
	if s, ok := d.Message.(*syntax.StringLit); ok {
		c.interpolations(env.withFacts(ff), s)
	}
	if d.Message != nil {
		c.info.Types[d.Message] = types.StringType
	}
}

// checkOwnerBody is the body of the record or case a member check belongs to, nil at package
// level.
func (c *checker) checkOwnerBody(o *object) *recordCtx {
	switch t := o.owner.(type) {
	case *types.RecordType:
		return c.bodyOf(t)
	case *types.CaseType:
		return c.caseBodies[t]
	}
	return nil
}

// checkAt is `at f`: a field of the same record or case (E3003 otherwise).
func (c *checker) checkAt(env *env, at *syntax.Ident) {
	if env.rec != nil {
		if fo, ok := env.rec.fields[at.Name]; ok {
			c.info.NameUses[at] = fo
			return
		}
		c.report(env, diag.E3003.At(env.span(at), env.rec.self, diag.KindField, at.Name))
	}
}

// testBody checks a test's block (EVALUATION.md §10).
func (c *checker) testBody(o *object) {
	d := o.decl.(*syntax.TestDecl)
	env := c.declEnv(o)
	c.info.Types[d.Name] = types.StringType
	c.block(env, d.Body)
}

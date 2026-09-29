package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// resolvePackage is TYPES.md §1 step 2 for p.
func (c *checker) resolvePackage(p *pkgState) {
	for _, o := range p.all {
		switch o.kind {
		case ObjTypeName:
			c.completeTypeName(o)
		case ObjFn:
			c.resolveSignature(o)
		case ObjLet:
			c.resolveLetAnnotation(o)
		case ObjWidget:
			c.resolveWidget(o)
		default:
		}
	}
	c.resolvePending(p)
	c.checkOptionals(p)
	c.checkUnions(p)
	c.checkSelfContaining(p)
	c.checkExposure(p)
}

// resolveTypeName is the type a type declaration names, resolved on demand; records, enums
// and variants are shells until completed.
func (c *checker) resolveTypeName(o *object) types.Type {
	td, isAlias := o.decl.(*syntax.TypeDecl)
	if !isAlias || o.state == stateDone {
		return o.typ
	}
	if len(td.Params) > 0 {
		return c.resolveTypeFunc(o, td)
	}
	return c.resolveAlias(o, td)
}

// completeTypeName resolves a type declaration and fills its record, enum or variant.
func (c *checker) completeTypeName(o *object) {
	switch t := c.resolveTypeName(o).(type) {
	case *types.RecordType:
		c.completeRecord(t)
	case *types.EnumType:
		c.completeEnum(o, t)
	case *types.VariantType:
		c.completeVariant(t)
	}
}

// resolveAlias is `type A = T` (TYPES.md §13.1); an alias reaching itself through aliases only is E3021.
func (c *checker) resolveAlias(o *object, td *syntax.TypeDecl) types.Type {
	switch o.state {
	case stateResolving:
		p := c.pkgs[o.pkg]
		c.deliver(p, origin{decl: o}, diag.E3021.At(o.file.Span(td.Name), o.name).Report)
		c.breakObj(o)
		o.state = stateDone
		o.typ = types.ErrorType
		return o.typ
	case stateDone:
		return o.typ
	default:
	}
	o.state = stateResolving
	a := &types.Alias{Pkg: o.pkg, Name: o.name, Doc: docText(td.Doc), Decl: td}
	env := c.declEnv(o)
	def := c.resolveType(&typeCtx{env: env}, td.Type)
	if o.state == stateDone {
		return o.typ
	}
	a.Def = def
	o.typ, o.state = a, stateDone
	return a
}

// declEnv is the env of a top-level declaration.
func (c *checker) declEnv(o *object) *env {
	return c.fileEnv(c.pkgs[o.pkg], o.file, o)
}

// completeRecord fills a record's parameters, fields, methods and checks, once.
func (c *checker) completeRecord(r *types.RecordType) {
	o := c.typeObjects[r]
	if o == nil || o.state != stateNone {
		return
	}
	o.state = stateResolving
	c.records++
	defer func() { c.records-- }()
	d := r.Decl
	env := c.declEnv(o)
	tc := &typeCtx{env: env, encl: r, scope: map[string]typeArgRoot{}}
	r.Params = c.typeParams(tc, d.Params)
	body := &recordCtx{self: r, fields: map[string]*object{}, methods: map[string]*object{}}
	o.body = body
	r.Fields = c.declareFields(env, r, body, d.Body.Items)
	c.resolveFields(tc, body, recordCase(d.Annotations))
	r.Methods, r.Checks = c.declareMembers(env, r, body, d.Body.Items)
	o.state = stateDone
}

// typeParams are the value parameters of a record or type function (TYPES.md §11.1).
func (c *checker) typeParams(tc *typeCtx, params []*syntax.Param) []*types.Param {
	var out []*types.Param
	seen := map[string]*syntax.Param{}
	env := tc.env
	env.params = map[string]*object{}
	for i, p := range params {
		declared := c.resolveType(tc, p.Type)
		t := c.paramType(env, p, declared)
		tp := &types.Param{Name: p.Name.Name, Index: i, Type: t}
		if t != declared {
			c.refused[tp] = declared
		}
		po := c.newObject(ObjParam, p.Name.Name, env.pkg, p, env.file)
		po.typ = t
		c.info.Defs[p.Name] = po
		if first, dup := seen[p.Name.Name]; dup {
			c.report(env, diag.E2106.At(env.span(p.Name), p.Name.Name, env.span(first.Name)))
			continue
		}
		seen[p.Name.Name] = p
		env.params[p.Name.Name] = po
		tc.scope[p.Name.Name] = typeArgRoot{param: tp, obj: po}
		out = append(out, tp)
	}
	return out
}

// paramType is t for a record, ref, enum or Bool parameter, else E3806 and the error type (TYPES.md §11.1).
func (c *checker) paramType(env *env, p *syntax.Param, t types.Type) types.Type {
	if k := t.Base().Kind(); k == types.Record || k == types.Ref || matchable(t) || k == types.Error {
		return t
	}
	c.report(env, diag.E3806.AtParam(env.span(p.Type), p.Name.Name, t))
	return types.ErrorType
}

// declareMembers declares the methods and checks of a record or case body: E2104 when a
// method has a field's name, E2106 for two methods of one name.
func (c *checker) declareMembers(env *env, owner types.Type, body *recordCtx, items []syntax.RecordItem) ([]*types.Method, []*syntax.CheckDecl) {
	var methods []*types.Method
	var checks []*syntax.CheckDecl
	for _, it := range items {
		switch it := it.(type) {
		case *syntax.FnDecl:
			if m := c.declareMethod(env, owner, body, it); m != nil {
				methods = append(methods, m)
			}
		case *syntax.CheckDecl:
			checks = append(checks, it)
			o := c.newObject(ObjCheck, identName(it.Name), env.pkg, it, env.file)
			o.owner = owner
			if it.Name != nil {
				c.info.Defs[it.Name] = o
			}
			env.pkg.all = append(env.pkg.all, o)
			c.memberChecks[it] = o
			// TYPES.md §1, DECISIONS 209: a broken check breaks the record or variant it runs through.
			c.dependsOn(env, o)
		}
	}
	return methods, checks
}

// declareMethod makes a method's object and signature; `self` excluded from its type.
func (c *checker) declareMethod(env *env, owner types.Type, body *recordCtx, fn *syntax.FnDecl) *types.Method {
	o := c.newObject(ObjMethod, fn.Name.Name, env.pkg, fn, env.file)
	o.owner = owner
	o.body = body
	c.info.Defs[fn.Name] = o
	env.pkg.all = append(env.pkg.all, o)
	sig := c.signature(c.fileEnv(env.pkg, env.file, o), o, fn)
	o.typ = sig
	m := &types.Method{Name: fn.Name.Name, Export: fn.Mods != nil && fn.Mods.Export.Valid(), Type: sig}
	if f, clash := body.fields[fn.Name.Name]; clash {
		c.report(env, diag.E2104.At(env.span(fn.Name), env.localName(owner.String(), ownerPkg(owner)), fn.Name.Name))
		c.breakObj(f)
		return m
	}
	if first, dup := body.methods[fn.Name.Name]; dup {
		c.report(env, diag.E2106.At(env.span(fn.Name), fn.Name.Name, declSpan(first)))
		return m
	}
	body.methods[fn.Name.Name] = o
	return m
}

// ownerPkg is the package of a record or of a case's variant.
func ownerPkg(t types.Type) string {
	switch x := t.(type) {
	case *types.RecordType:
		return x.Pkg
	case *types.CaseType:
		return x.Variant.Pkg
	default:
		return ""
	}
}

// completeEnum fills an enum's members (TYPES.md §8.1).
func (c *checker) completeEnum(o *object, e *types.EnumType) {
	if o.state != stateNone {
		return
	}
	o.state = stateDone
	env := c.declEnv(o)
	d := e.Decl
	c.enumAnnotations(e, d.Annotations)
	seen := map[string]*object{}
	for i, m := range d.Members {
		mem := c.enumMember(env, e, i, m)
		mo := c.newObject(ObjMember, m.Name.Name, env.pkg, m, env.file)
		mo.typ, mo.member, mo.owner = e, mem, e
		c.info.Defs[m.Name] = mo
		if first, dup := seen[m.Name.Name]; dup {
			c.report(env, diag.E2106.At(env.span(m.Name), m.Name.Name, declSpan(first)))
			continue
		}
		seen[m.Name.Name] = mo
		c.members[e] = append(c.members[e], mo)
		e.Members = append(e.Members, mem)
	}
}

// memberObject is the object of an enum member by name, or nil.
func (c *checker) memberObject(e *types.EnumType, name string) *object {
	c.completeEnum(c.typeObjects[e], e)
	for _, m := range c.members[e] {
		if m.name == name {
			return m
		}
	}
	return nil
}

// completeVariant fills a variant's cases, their fields, methods and checks (TYPES.md §8).
func (c *checker) completeVariant(v *types.VariantType) {
	o := c.typeObjects[v]
	if o == nil || o.state != stateNone {
		return
	}
	o.state = stateResolving
	c.records++
	defer func() { c.records-- }()
	env := c.declEnv(o)
	d := v.Decl
	c.variantAnnotations(v, d.Annotations)
	c.variantBody[v] = &recordCtx{self: v, fields: map[string]*object{}, methods: map[string]*object{}}
	seen := map[string]*object{}
	for _, it := range d.Items {
		vc, ok := it.(*syntax.VariantCase)
		if !ok {
			continue
		}
		co := c.declareCase(env, v, vc, len(v.Cases))
		if first, dup := seen[vc.Name.Name]; dup {
			c.report(env, diag.E2106.At(env.span(vc.Name), vc.Name.Name, declSpan(first)))
			continue
		}
		seen[vc.Name.Name] = co
		c.cases[v] = append(c.cases[v], co)
		v.Cases = append(v.Cases, co.typ.(*types.CaseType))
	}
	for _, co := range c.cases[v] {
		c.completeCase(env, co)
	}
	c.variantMembers(env, v, d)
	o.state = stateDone
}

// variantMembers declares a variant's methods and checks written outside any case, `self: V` (TYPES.md §12.1).
func (c *checker) variantMembers(env *env, v *types.VariantType, d *syntax.VariantDecl) {
	var items []syntax.RecordItem
	for _, it := range d.Items {
		if ri, ok := it.(syntax.RecordItem); ok {
			items = append(items, ri)
		}
	}
	body := c.variantBody[v]
	c.declareMembers(env, v, body, items)
	for _, it := range items {
		fn, ok := it.(*syntax.FnDecl)
		if !ok {
			continue
		}
		// A method refused as a duplicate has its E2106 already, as a record's (TYPES.md §12.1).
		if m := body.methods[fn.Name.Name]; m != nil && m.decl == fn {
			c.sharedName(env, v, fn)
		}
	}
}

// VariantChecks are the checks of a variant body outside any case, in source order (TYPES.md §12.1); nil for a nil declaration.
func VariantChecks(d *syntax.VariantDecl) []*syntax.CheckDecl {
	if d == nil {
		return nil
	}
	var out []*syntax.CheckDecl
	for _, it := range d.Items {
		if c, ok := it.(*syntax.CheckDecl); ok {
			out = append(out, c)
		}
	}
	return out
}

// sharedName is E2105, E2104 or E2106 for a variant-level method sharing a case's name (TYPES.md §12.1).
func (c *checker) sharedName(env *env, v *types.VariantType, fn *syntax.FnDecl) {
	name := fn.Name.Name
	if name == kindMember {
		c.report(env, diag.E2105.AtCase(env.span(fn.Name), kindMember))
		return
	}
	at := env.span(fn.Name)
	for _, co := range c.cases[v] {
		body := co.body
		if body == nil {
			continue
		}
		if f := body.fields[name]; f != nil {
			c.report(env, diag.E2104.At(at, env.localName(co.typ.String(), v.Pkg), name))
			c.breakObj(f)
		}
		if m := body.methods[name]; m != nil {
			first, second := fileSpan{m.file, declSpan(m)}, fileSpan{env.file, at}
			if compareFileSpans(first, second) > 0 {
				first, second = second, first
			}
			c.report(env, diag.E2106.At(second.span, name, first.span))
		}
	}
}

// caseObject is the object of a variant's case by name, or nil.
func (c *checker) caseObject(v *types.VariantType, name string) *object {
	c.completeVariant(v)
	for _, co := range c.cases[v] {
		if co.name == name {
			return co
		}
	}
	return nil
}

// declareCase makes a case's type and object; its body is completed after every case exists.
func (c *checker) declareCase(env *env, v *types.VariantType, vc *syntax.VariantCase, index int) *object {
	ct := &types.CaseType{Variant: v, Name: vc.Name.Name, Wire: vc.Name.Name, Index: index, Doc: docText(vc.Doc)}
	ct.Retired = vc.Mods != nil && vc.Mods.Retired.Valid()
	c.caseAnnotations(ct, vc)
	co := c.newObject(ObjCase, vc.Name.Name, env.pkg, vc, env.file)
	co.typ, co.owner = ct, v
	c.info.Defs[vc.Name] = co
	return co
}

// completeCase fills a case body like a record's; `kind` is reserved on cases (E2105).
func (c *checker) completeCase(env *env, co *object) {
	ct := co.typ.(*types.CaseType)
	body := &recordCtx{self: ct, fields: map[string]*object{}, methods: map[string]*object{}, outer: c.variantBody[ct.Variant]}
	co.body = body
	c.caseBodies[ct] = body
	vc := co.decl.(*syntax.VariantCase)
	if vc.Body == nil {
		return
	}
	tc := &typeCtx{env: env, encl: ct, scope: map[string]typeArgRoot{}}
	ct.Fields = c.declareFields(env, ct, body, vc.Body.Items)
	c.resolveFields(tc, body, c.caseWireCase(ct))
	ct.Methods, ct.Checks = c.declareMembers(env, ct, body, vc.Body.Items)
	for _, it := range vc.Body.Items {
		var n *syntax.Ident
		switch it := it.(type) {
		case *syntax.FieldDecl:
			n = it.Name
		case *syntax.FnDecl:
			n = it.Name
		}
		if n != nil && n.Name == kindMember {
			c.report(env, diag.E2105.AtCase(env.span(n), kindMember))
		}
	}
}

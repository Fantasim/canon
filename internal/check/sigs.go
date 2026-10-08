package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// resolveSignature types a top-level function (TYPES.md §12.1).
func (c *checker) resolveSignature(o *object) {
	if o.typ != nil {
		return
	}
	o.typ = c.signature(c.declEnv(o), o, o.decl.(*syntax.FnDecl))
}

// signature is a function's type: parameter types (a function type allowed, E3306 elsewhere),
// then the result; its parameters' objects are kept for the body. E2106 for a parameter
// declared twice.
func (c *checker) signature(env *env, o *object, fn *syntax.FnDecl) *types.FuncType {
	ft := &types.FuncType{}
	ptc := &typeCtx{env: env, pos: posFn}
	seen := map[string]*object{}
	var params []*object
	for _, p := range fn.Params {
		t := c.resolveType(ptc, p.Type)
		po := c.newObject(ObjParam, p.Name.Name, env.pkg, p, env.file)
		po.typ = t
		c.info.Defs[p.Name] = po
		ft.Params = append(ft.Params, t)
		params = append(params, po)
		if first, dup := seen[p.Name.Name]; dup {
			c.report(env, diag.E2106.At(env.span(p.Name), p.Name.Name, declSpan(first)))
			continue
		}
		seen[p.Name.Name] = po
	}
	ft.Result = c.resolveType(&typeCtx{env: env}, fn.Result)
	c.fnParams[o] = params
	return ft
}

// resolveLetAnnotation types a top-level let's annotation: `stable table` allowed as the whole type (LOCK.md §1).
func (c *checker) resolveLetAnnotation(o *object) {
	d := o.decl.(*syntax.LetDecl)
	if d.Type == nil {
		if !o.local {
			c.report(c.declEnv(o), diag.E3001.At(o.file.Span(d.Name), o.name))
		}
		return
	}
	if o.typ != nil || o.state == stateResolving { // an annotation naming its own let ends (log 2026-09-24, check C2 review)
		return
	}
	o.state = stateResolving
	env := c.declEnv(o)
	env.sig = true
	t := c.resolveType(&typeCtx{env: env, pos: posStable}, d.Type)
	o.typ, o.state = t, stateDone
	if _, isTable := o.typ.Base().(*types.TableType); !isTable {
		o.keys = nil
	}
}

// annotating reports a let whose annotation is being resolved: its type is not known yet.
func annotating(o *object) bool {
	d, ok := o.decl.(*syntax.LetDecl)
	return ok && d.Type != nil && o.state == stateResolving
}

// letType is a let's annotation, else its initializer synthesized when first needed (TYPES.md §15).
func (c *checker) letType(o *object) types.Type {
	if o.typ != nil {
		return o.typ
	}
	d := o.decl.(*syntax.LetDecl)
	if d.Type != nil {
		c.resolveLetAnnotation(o)
		if o.typ == nil { // its annotation is being resolved
			return types.ErrorType
		}
		return o.typ
	}
	env := c.declEnv(o)
	if o.state == stateResolving {
		c.report(env, diag.E3008.At(env.span(d.Name)))
		return types.ErrorType
	}
	o.state = stateResolving
	c.inferring = append(c.inferring, o)
	c.buffered[o] = nil
	env.bind = &bindCtx{field: types.AnyType} // evaluation reads the inferred type, a static view
	t := c.expr(env, d.Value, nil)
	c.inferring = c.inferring[:len(c.inferring)-1]
	o.state = stateDone
	c.settle(o)
	if o.typ == nil {
		o.typ = inferred(t)
	}
	if _, isTable := o.typ.Base().(*types.TableType); !isTable {
		o.keys = nil
	}
	c.initDone[o] = true
	return o.typ
}

// resolveWidget types a widget's parameters: `_` is allowed there (TYPES.md §13.6).
func (c *checker) resolveWidget(o *object) {
	d := o.decl.(*syntax.WidgetDecl)
	env := c.declEnv(o)
	tc := &typeCtx{env: env, pos: posAny}
	for i, p := range d.Params {
		t := c.resolveType(tc, p.Type)
		po := c.newObject(ObjParam, p.Name.Name, env.pkg, p, env.file)
		po.typ = t
		c.info.Defs[p.Name] = po
		if i == 0 {
			o.typ = t
		}
	}
	if o.typ == nil {
		o.typ = types.ErrorType
	}
}

// checkSelfContaining is E3022: a record holding itself through required fields (TYPES.md §13.1).
func (c *checker) checkSelfContaining(p *pkgState) {
	for _, o := range p.all {
		rec, ok := o.typ.(*types.RecordType)
		if o.kind != ObjTypeName || !ok {
			continue
		}
		if reaches(rec, rec, map[*types.RecordType]bool{}) {
			env := c.declEnv(o)
			c.report(env, diag.E3022.At(env.span(rec.Decl.Name), o.name))
		}
	}
}

// reaches reports that a value of from needs a value of to through required record fields.
func reaches(from, to *types.RecordType, seen map[*types.RecordType]bool) bool {
	if seen[from] {
		return false
	}
	seen[from] = true
	for _, f := range from.Fields {
		next := requiredRecord(f.Type)
		if next == nil || f.Input != nil {
			continue
		}
		if next == to || reaches(next, to, seen) {
			return true
		}
	}
	return false
}

// selfContaining reports a field its record contains itself through: E3022's alone (TYPES.md §1, §13.1).
func selfContaining(owner *types.RecordType, f *types.Field) bool {
	next := requiredRecord(f.Type)
	if next == nil || f.Input != nil || owner == nil {
		return false
	}
	return next == owner || reaches(next, owner, map[*types.RecordType]bool{})
}

// requiredRecord is the record a field type needs directly: not optional, not in a list, map,
// table or ref.
func requiredRecord(t types.Type) *types.RecordType {
	switch x := t.Base().(type) {
	case *types.RecordType:
		return x
	case *types.AppliedRecord:
		return x.Rec
	}
	return nil
}

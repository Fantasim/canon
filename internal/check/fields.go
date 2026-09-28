package check

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// declareFields makes the fields of a record or case body, their names only: E2106 for a
// field declared twice.
func (c *checker) declareFields(env *env, owner types.Type, body *recordCtx, items []syntax.RecordItem) []*types.Field {
	var out []*types.Field
	for _, it := range items {
		fd, ok := it.(*syntax.FieldDecl)
		if !ok {
			continue
		}
		name := fd.Name.Name
		f := &types.Field{Name: name, Index: len(out), Wire: name, WirePath: []string{name}, Doc: docText(fd.Doc), Default: fd.Default, Annotations: fd.Annotations}
		fo := c.newObject(ObjField, name, env.pkg, fd, env.file)
		fo.field, fo.owner = f, owner
		c.info.Defs[fd.Name] = fo
		if first, dup := body.fields[name]; dup {
			c.report(env, diag.E2106.At(env.span(fd.Name), name, declSpan(first)))
			continue
		}
		body.fields[name] = fo
		body.order = append(body.order, fo)
		c.fieldObjects[f] = fo
		out = append(out, f)
	}
	return out
}

// resolveFields types the fields in order, only earlier ones as type-argument roots (TYPES.md §11.1).
func (c *checker) resolveFields(tc *typeCtx, body *recordCtx, wireCase string) {
	later := map[string]bool{}
	for _, fo := range body.order {
		later[fo.name] = true
		c.fieldJobs[fo.field] = func() {
			ftc := *tc
			ftc.later = later
			c.resolveField(&ftc, fo, wireCase)
			delete(later, fo.name)
		}
	}
	for _, fo := range body.order {
		c.fieldType(fo.field)
	}
}

// fieldType resolves a field's type now if it is still to be resolved: a key field named by a
// keyed list of its own record is needed before its turn.
func (c *checker) fieldType(f *types.Field) types.Type {
	if job, ok := c.fieldJobs[f]; ok {
		delete(c.fieldJobs, f)
		job()
	}
	if f.Type == nil {
		return types.ErrorType
	}
	return f.Type
}

// resolveField types one field, then its wire mapping and input, unjudged when its type is in error (TYPES.md §1).
func (c *checker) resolveField(tc *typeCtx, fo *object, wireCase string) {
	fd := fo.decl.(*syntax.FieldDecl)
	f := fo.field
	f.Type = c.resolveType(tc, fd.Type)
	fo.typ = f.Type
	f.DependsOn = dependsOn(f.Type)
	c.fieldAnnotations(tc.env, f, fd, wireCase, c.typeInError(fd.Type, f.Type))
	c.fieldInput(tc.env, f, fd)
	tc.scope[fo.name] = typeArgRoot{field: f, obj: fo}
}

// dependsOn lists the earlier fields a field type's arguments read (TYPES.md §11).
func dependsOn(t types.Type) []int {
	var out []int
	walkArgs(t, func(a *types.Arg) {
		if a.Source == types.ArgField && len(a.Path) > 0 {
			out = append(out, a.Path[0].Index)
		}
	})
	return out
}

// walkArgs calls f on every type argument of t, through lists, maps, dependent maps and optionals.
func walkArgs(t types.Type, f func(*types.Arg)) {
	switch x := t.Base().(type) {
	case *types.TypeAppType:
		for _, a := range x.Args {
			f(a)
		}
	case *types.AppliedRecord:
		for _, a := range x.Args {
			f(a)
		}
	case *types.OptionalType:
		walkArgs(x.Elem, f)
	case *types.ListType:
		walkArgs(x.Elem, f)
	case *types.MapType:
		walkArgs(x.Key, f)
		walkArgs(x.Value, f)
	case *types.LitUnionType:
		walkArgs(x.Of, f)
	case *types.DepMapType:
		walkArgs(x.Value, f)
	}
}

// fieldInput is `input T from env "NAME"` (EVALUATION.md §11.1).
func (c *checker) fieldInput(env *env, f *types.Field, fd *syntax.FieldDecl) {
	if !fd.Input.Valid() {
		return
	}
	name := ""
	if fd.Env != nil {
		name = constText(fd.Env)
		c.info.Types[fd.Env] = types.StringType
	}
	f.Input = &types.Input{Env: name}
	if fd.Default != nil {
		c.report(env, diag.E1907.At(env.span(fd.Default), f.Name))
	}
	if fd.Env != nil && !envName.MatchString(name) {
		c.report(env, diag.E1911.At(env.span(fd.Env), name))
	}
	if !inputType(f.Type) {
		c.report(env, diag.E1910.At(env.span(fd.Type), f.Name, f.Type))
	}
	c.portable(env, f, fd.Type)
}

var envName = regexp.MustCompile(envNamePattern)

// inputType reports a type an input may have: a scalar or an enum, optional or not, refined
// by ranges, lengths and patterns only; a type in error anywhere is not judged again.
func inputType(t types.Type) bool {
	for {
		if t.Kind() == types.Error {
			return true
		}
		switch x := t.(type) {
		case *types.Alias:
			t = x.Def
		case *types.OptionalType:
			t = x.Elem
		case *types.Refined:
			if x.Where != nil || x.Asset != nil {
				return false
			}
			t = x.Of
		case types.Basic:
			return true
		case *types.EnumType:
			return true
		default:
			return false
		}
	}
}

// typeInError reports a field's own type holding the error type, a dropped refinement or a lexer error (TYPES.md §1).
func (c *checker) typeInError(expr syntax.Type, t types.Type) bool {
	if holds(t, types.Error) {
		return true
	}
	bad := false
	syntax.Inspect(expr, func(n syntax.Node) bool {
		bad = bad || c.unrefined[n] || c.lexError(n)
		return !bad
	})
	return bad
}

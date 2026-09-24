package check

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// wireLocation is a key path a field writes, with the field that writes it.
type wireLocation struct {
	path  []string
	field *types.Field
}

// checkWireKeys is E3316 over one record or case body (WIRE.md §4.2).
func (c *checker) checkWireKeys(env *env, body *recordCtx) {
	var seen []wireLocation
	for _, fo := range body.order {
		f := fo.field
		if f.Input != nil || f.Inline {
			continue
		}
		at := env.span(fo.decl.(*syntax.FieldDecl).Name)
		for _, path := range fieldKeyPaths(f) {
			if strings.HasPrefix(path[0], dollar) {
				c.report(env, diag.E3316.AtDollar(at, path[0], f.Name))
				continue
			}
			if other := clash(seen, path); other != nil {
				c.reportClash(env, at, f, other, path)
				continue
			}
			seen = append(seen, wireLocation{path: path, field: f})
		}
	}
}

// fieldKeyPaths are the key paths a field writes: its path, or each expanded key of pairs.
func fieldKeyPaths(f *types.Field) [][]string {
	if f.Pairs == nil {
		return [][]string{f.WirePath}
	}
	var out [][]string
	for i := range f.Pairs.Slots {
		for _, tpl := range f.Pairs.Keys {
			out = append(out, []string{strings.Replace(tpl, pairSlot, strconv.Itoa(i), 1)})
		}
	}
	return out
}

// clash is an earlier location equal to path or a prefix of it (or path a prefix of it).
func clash(seen []wireLocation, path []string) *wireLocation {
	for i := range seen {
		a, b := seen[i].path, path
		n := min(len(a), len(b))
		if slices.Equal(a[:n], b[:n]) {
			return &seen[i]
		}
	}
	return nil
}

// reportClash is E3316: the same key twice, or one key path a prefix of another.
func (c *checker) reportClash(env *env, at source.Span, f *types.Field, other *wireLocation, path []string) {
	switch {
	case len(other.path) == len(path):
		c.report(env, diag.E3316.AtCollision(at, path[len(path)-1], f.Name, other.field.Name))
	case len(path) < len(other.path):
		c.report(env, diag.E3316.AtPrefix(at, f.Name, other.field.Name))
	default:
		c.report(env, diag.E3316.AtPrefix(at, other.field.Name, f.Name))
	}
}

// checkInline is E3318 (WIRE.md §4.2).
func (c *checker) checkInline(env *env, body *recordCtx) {
	var inline []*object
	for _, fo := range body.order {
		if fo.field.Inline {
			inline = append(inline, fo)
		}
	}
	if len(inline) > 1 {
		c.report(env, diag.E3318.AtTwoInline(env.span(inline[1].decl.(*syntax.FieldDecl).Name)))
	}
	for _, fo := range inline {
		v, ok := fo.field.Type.Base().(*types.VariantType)
		if ok {
			c.inlineKeys(env, body, fo, v)
		}
	}
}

// inlineKeys compares an inline variant's tag and case keys with the parent's keys.
func (c *checker) inlineKeys(env *env, body *recordCtx, inline *object, v *types.VariantType) {
	keys := []string{v.Tag}
	for _, ct := range v.Cases {
		for _, f := range ct.Fields {
			keys = append(keys, f.WirePath[0])
		}
	}
	at := env.span(inline.decl.(*syntax.FieldDecl).Name)
	for _, fo := range body.order {
		if fo == inline || fo.field.Input != nil {
			continue
		}
		if slices.Contains(keys, fo.field.WirePath[0]) {
			c.report(env, diag.E3318.AtInline(at, fo.field.WirePath[0], v.String(), fo.name))
			return
		}
	}
}

// checkTag is E3318: a case field whose wire name is its variant's tag key.
func (c *checker) checkTag(env *env, v *types.VariantType) {
	for _, co := range c.cases[v] {
		for _, fo := range co.body.order {
			if fo.field.Input == nil && fo.field.WirePath[0] == v.Tag {
				c.report(env, diag.E3318.AtTag(env.span(fo.decl.(*syntax.FieldDecl).Name), fo.name, v.Tag))
			}
		}
	}
}

// checkPairsDefault is E3316: a pairs field takes no default but `[]` (WIRE.md §5.14).
func (c *checker) checkPairsDefault(env *env, body *recordCtx) {
	for _, fo := range body.order {
		f := fo.field
		if f.Pairs == nil || f.Default == nil {
			continue
		}
		if l, ok := f.Default.(*syntax.ListLit); ok && len(l.Elems) == 0 {
			continue
		}
		c.report(env, diag.E3316.AtPairsDefault(env.span(f.Default), f.Name))
	}
}

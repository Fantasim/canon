package check

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// wireLocation is what a field writes: a key path, or the keys a `pairs:` template expands to.
type wireLocation struct {
	path  []string
	tpl   *pairTemplate
	field *types.Field
}

// checkWireKeys is E3316 over a body; pairs compare by template, whatever the bound (WIRE.md §4.2).
func (c *checker) checkWireKeys(env *env, body *recordCtx) {
	var seen []wireLocation
	for _, fo := range body.order {
		f, fd := fo.field, fo.decl.(*syntax.FieldDecl)
		if f.Input != nil || f.Inline || c.wireUnknown(fd) {
			continue
		}
		at := env.span(fd.Name)
		for _, loc := range fieldLocations(f) {
			if c.place(env, at, loc, seen) {
				seen = append(seen, loc)
			}
		}
	}
}

// wireUnknown reports a field whose keys come from a `@json` argument holding a lexer error (its
// wire name, path or pairs templates): they are not known, so they clash with nothing (DECISIONS 215).
func (c *checker) wireUnknown(fd *syntax.FieldDecl) bool {
	j := annotation(fd.Annotations, annotJSON)
	if j == nil {
		return false
	}
	return slices.ContainsFunc([]syntax.AnnValue{positional(j), named(j, jsonPath), named(j, jsonPairsName)}, func(v syntax.AnnValue) bool {
		return v != nil && c.holdsLexError(v)
	})
}

// place reports what is wrong with loc against the earlier locations, and whether later ones
// must still meet it: a clashing template stays, so a later field meets its keys too.
func (c *checker) place(env *env, at source.Span, loc wireLocation, seen []wireLocation) bool {
	if loc.tpl != nil && strings.HasPrefix(loc.tpl.pre, dollar) {
		c.report(env, diag.E3316.AtPairsTemplate(at, loc.tpl.text()))
		return false
	}
	if loc.tpl == nil && strings.HasPrefix(loc.path[0], dollar) {
		c.report(env, diag.E3316.AtDollar(at, loc.path[0], loc.field.Name))
		return false
	}
	m, found := clash(seen, loc)
	switch {
	case !found:
		return true
	case loc.tpl != nil && m.other == loc.field:
		c.report(env, diag.E3316.AtPairsTemplate(at, loc.tpl.text()))
	default:
		c.reportClash(env, at, loc.field, m)
	}
	return loc.tpl != nil
}

// fieldLocations are a field's key path, or one location per `pairs:` template.
func fieldLocations(f *types.Field) []wireLocation {
	if f.Pairs == nil {
		return []wireLocation{{path: f.WirePath, field: f}}
	}
	var out []wireLocation
	for _, tpl := range f.Pairs.Keys {
		if f.Pairs.Slots > 0 {
			out = append(out, wireLocation{tpl: newPairTemplate(tpl, f.Pairs.Slots), field: f})
		}
	}
	return out
}

// meeting is an earlier field whose key path equals, or is a prefix of, the other's, or the reverse.
type meeting struct {
	other        *types.Field
	theirs, mine []string
}

// clash is the first earlier location meeting loc.
func clash(seen []wireLocation, loc wireLocation) (meeting, bool) {
	for i := range seen {
		if theirs, mine, ok := meet(seen[i], loc); ok {
			return meeting{other: seen[i].field, theirs: theirs, mine: mine}, true
		}
	}
	return meeting{}, false
}

// meet reports two locations with a key path equal to, or a prefix of, the other's.
func meet(a, b wireLocation) (pa, pb []string, ok bool) {
	switch {
	case a.tpl != nil && b.tpl != nil:
		k, found := commonKey(*a.tpl, *b.tpl)
		return []string{k}, []string{k}, found
	case a.tpl != nil:
		return b.path[:1], b.path, a.tpl.matches(b.path[0])
	case b.tpl != nil:
		return a.path, a.path[:1], b.tpl.matches(a.path[0])
	}
	n := min(len(a.path), len(b.path))
	return a.path, b.path, slices.Equal(a.path[:n], b.path[:n])
}

// reportClash is E3316: the same key twice, or one key path a prefix of another.
func (c *checker) reportClash(env *env, at source.Span, f *types.Field, m meeting) {
	switch {
	case len(m.theirs) == len(m.mine):
		c.report(env, diag.E3316.AtCollision(at, m.mine[len(m.mine)-1], f.Name, m.other.Name))
	case len(m.mine) < len(m.theirs):
		c.report(env, diag.E3316.AtPrefix(at, f.Name, m.other.Name))
	default:
		c.report(env, diag.E3316.AtPrefix(at, m.other.Name, f.Name))
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

// checkPairsDefault is E3316: a pairs field takes no default but `[]`, one of the error type unjudged (WIRE.md §5.14).
func (c *checker) checkPairsDefault(env *env, body *recordCtx) {
	for _, fo := range body.order {
		f := fo.field
		if f.Pairs == nil || f.Default == nil || c.errorTyped(f.Default) {
			continue
		}
		if l, ok := f.Default.(*syntax.ListLit); ok && len(l.Elems) == 0 {
			continue
		}
		c.report(env, diag.E3316.AtPairsDefault(env.span(f.Default), f.Name))
	}
}

// errorTyped reports an expression the checker gave the error type (TYPES.md §1).
func (c *checker) errorTyped(e syntax.Expr) bool {
	t := c.info.Types[e]
	return t != nil && t.Kind() == types.Error
}

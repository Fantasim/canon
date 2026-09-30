package load

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/wire"
)

// parsedCall is a load call's literal path and named options, read off the syntax tree (WIRE.md §6.1).
type parsedCall struct {
	path    string
	names   []string // every named option given, in call order, valid or not (checkOptions's E7006)
	at      *string
	format  *string
	prefix  *string
	partial *bool
	header  *bool
}

// parseCall reads e's positional path and named options; ok is false when the call is not
// literal enough for this milestone to read without the evaluator.
func parseCall(e *syntax.LoadExpr) (parsedCall, bool) {
	if len(e.Args) == 0 || e.Args[0].Name != nil {
		return parsedCall{}, false
	}
	path, ok := pathOf(e.Args[0].Value)
	if !ok {
		return parsedCall{}, false
	}
	c := parsedCall{path: path}
	for _, a := range e.Args[1:] {
		if a.Name == nil || !c.setOption(a) {
			return parsedCall{}, false
		}
	}
	return c, true
}

func pathOf(e syntax.Expr) (string, bool) {
	s, ok := e.(*syntax.StringLit)
	if !ok {
		return "", false
	}
	return plainString(s)
}

// setOption reads one named option's literal value; false only when a recognized option's
// value is not a plain literal. An unrecognized name is still recorded, for checkOptions's E7006.
func (c *parsedCall) setOption(a *syntax.Arg) bool {
	c.names = append(c.names, a.Name.Name)
	switch a.Name.Name {
	case optionAt:
		return c.setString(&c.at, a.Value)
	case optFormat:
		return c.setSymbol(&c.format, a.Value)
	case optionPrefix:
		return c.setString(&c.prefix, a.Value)
	case optPartial:
		return c.setBool(&c.partial, a.Value)
	case optHeader:
		return c.setBool(&c.header, a.Value)
	default:
		return true
	}
}

func (c *parsedCall) setString(dst **string, e syntax.Expr) bool {
	s, ok := pathOf(e)
	if ok {
		*dst = &s
	}
	return ok
}

func (c *parsedCall) setSymbol(dst **string, e syntax.Expr) bool {
	id, ok := e.(*syntax.IdentExpr)
	if ok {
		*dst = &id.Name
	}
	return ok
}

func (c *parsedCall) setBool(dst **bool, e syntax.Expr) bool {
	b, ok := e.(*syntax.BoolLit)
	if ok {
		*dst = &b.Value
	}
	return ok
}

// checkOptions is WIRE.md §6.1's E7006 (form or format), formOptions and formatInvalid the data.
func checkOptions(form string, c parsedCall, format types.LoadFormat, req Request) bool {
	ok := true
	for _, name := range c.names {
		if !formOptions[form][name] || formatInvalid(name, format) {
			diag.E7006.AtOption(req.Span, name, formLabel(form), formatText(form, format)).Report(req.Bag)
			ok = false
		}
	}
	return ok
}

// formatInvalid is WIRE.md §6.1's format-specific E7006 causes.
func formatInvalid(option string, format types.LoadFormat) bool {
	switch option {
	case optionAt:
		return format == types.FormatCSV || format == types.FormatText
	case optHeader:
		return format != types.FormatCSV
	case optPartial:
		return format == types.FormatText
	default:
		return false
	}
}

// formLabel is form's own call text for a finding: "load" or "load.dir" (WIRE.md §6.1).
func formLabel(form string) string {
	if form == loadForm {
		return wordLoad
	}
	return wordLoad + dotSeg + form
}

// notLiteral is form's refusal of a call whose path or options are not plain literals.
func notLiteral(form string) error {
	return unsupported(causeArticle + formLabel(form) + causeNotLiteral)
}

// formatText is the format E7006 names: a fixed form's own, else the bare or dir call's format,
// a name types itself does not provide.
func formatText(form string, format types.LoadFormat) string {
	switch form {
	case methodCSV:
		return methodCSV
	case methodText:
		return methodText
	case formDefines:
		return formDefines
	default:
		return formatNames[format]
	}
}

// detectFormat is display's format from its extension (WIRE.md §6.2); ok is false after E7007.
func detectFormat(display string, req Request) (types.LoadFormat, bool) {
	f := types.FormatOfPath(display)
	if f == types.FormatUnknown {
		diag.E7007.At(req.Span, display).Report(req.Bag)
		return types.FormatUnknown, false
	}
	return f, true
}

// resolveCallFormat is c's format: c.format's symbol if given, else display's extension (WIRE.md §6.2).
func resolveCallFormat(c parsedCall, display string, req Request) (types.LoadFormat, bool) {
	if c.format != nil {
		f := types.FormatNamed(*c.format)
		if f == types.FormatUnknown {
			diag.E7006.AtFormat(req.Span, *c.format).Report(req.Bag)
			return types.FormatUnknown, false
		}
		return f, true
	}
	return detectFormat(display, req)
}

// applyAt selects path's value from root, E7106 at the JSON value where it failed (WIRE.md §6.3).
func applyAt(root *jsonsrc.Node, path string, req Request) (wire.Selection, bool) {
	steps, ok := wire.ParseAt(path)
	if !ok {
		diag.E7106.AtSyntax(req.Span, path).Report(req.Bag)
		return wire.Selection{}, false
	}
	return atApply(wire.Selection{Node: root}, steps, path, req)
}

func atApply(sel wire.Selection, steps []wire.AtStep, path string, req Request) (wire.Selection, bool) {
	if len(steps) == 0 {
		return sel, true
	}
	step := steps[0]
	switch step.Kind {
	case wire.AtName:
		return atApplyName(sel, step.Name, steps[1:], path, req)
	case wire.AtIndex:
		return atApplyIndex(sel, step.Index, steps[1:], path, req)
	default:
		return atApplyStar(sel, steps[1:], path, req)
	}
}

func atApplyName(sel wire.Selection, name string, rest []wire.AtStep, path string, req Request) (wire.Selection, bool) {
	n := sel.Node
	if n.Kind != jsonsrc.Object {
		diag.E7106.AtNotObject(n.Span, path, n.Pointer()).Report(req.Bag)
		return wire.Selection{}, false
	}
	for i := range n.Members {
		if n.Members[i].Key == name {
			return atApply(wire.Selection{Node: n.Members[i].Value}, rest, path, req)
		}
	}
	diag.E7106.AtMember(n.Span, path, name, n.Pointer()).Report(req.Bag)
	return wire.Selection{}, false
}

func atApplyIndex(sel wire.Selection, index int, rest []wire.AtStep, path string, req Request) (wire.Selection, bool) {
	n := sel.Node
	if n.Kind != jsonsrc.Array {
		diag.E7106.AtNotArray(n.Span, path, n.Pointer()).Report(req.Bag)
		return wire.Selection{}, false
	}
	if index >= len(n.Elems) {
		diag.E7106.AtIndex(n.Span, path, int64(index), n.Pointer()).Report(req.Bag)
		return wire.Selection{}, false
	}
	return atApply(wire.Selection{Node: n.Elems[index]}, rest, path, req)
}

func atApplyStar(sel wire.Selection, rest []wire.AtStep, path string, req Request) (wire.Selection, bool) {
	n := sel.Node
	switch n.Kind {
	case jsonsrc.Object:
		items, ok := atStarItems(len(n.Members), func(i int) *jsonsrc.Node { return n.Members[i].Value }, rest, path, req)
		return wire.Selection{Node: n, Star: true, Items: items}, ok
	case jsonsrc.Array:
		items, ok := atStarItems(len(n.Elems), func(i int) *jsonsrc.Node { return n.Elems[i] }, rest, path, req)
		return wire.Selection{Node: n, Star: true, Items: items}, ok
	default:
		diag.E7106.AtScalar(n.Span, path, n.Pointer()).Report(req.Bag)
		return wire.Selection{}, false
	}
}

// atStarItems applies rest to every one of n items, every one even after a failure (WIRE.md §6.3).
func atStarItems(n int, at func(int) *jsonsrc.Node, rest []wire.AtStep, path string, req Request) ([]wire.Selection, bool) {
	items, ok := make([]wire.Selection, n), true
	for i := range items {
		item, itemOK := atApply(wire.Selection{Node: at(i)}, rest, path, req)
		items[i], ok = item, ok && itemOK
	}
	return items, ok
}

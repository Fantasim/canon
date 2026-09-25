package load

import (
	"strconv"
	"strings"

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
func checkOptions(form string, c parsedCall, format wireFormat, req Request) bool {
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
func formatInvalid(option string, format wireFormat) bool {
	switch option {
	case optionAt:
		return format == fmtCSV || format == fmtText
	case optHeader:
		return format != fmtCSV
	case optPartial:
		return format == fmtText
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

// formatText is the format E7006 names: a fixed form's own, else the bare or dir call's wireFormat.
func formatText(form string, format wireFormat) string {
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
func detectFormat(display string, req Request) (wireFormat, bool) {
	f := formatOf(display)
	if f == fmtUnknown {
		diag.E7007.At(req.Span, display).Report(req.Bag)
		return fmtUnknown, false
	}
	return f, true
}

// formatSymbol is format:'s value as a wireFormat; false for a name this milestone refuses (WIRE.md §6.2).
func formatSymbol(s string) (wireFormat, bool) {
	for f, name := range formatNames {
		if wireFormat(f) != fmtUnknown && name == s {
			return wireFormat(f), true
		}
	}
	return fmtUnknown, false
}

// resolveCallFormat is c's format: c.format's symbol if given, else display's extension (WIRE.md §6.2).
func resolveCallFormat(c parsedCall, display string, req Request) (wireFormat, bool) {
	if c.format != nil {
		f, ok := formatSymbol(*c.format)
		if !ok {
			diag.E7006.AtFormat(req.Span, *c.format).Report(req.Bag)
			return fmtUnknown, false
		}
		return f, true
	}
	return detectFormat(display, req)
}

// formatFits is whether format, once known, can produce t: a bare load's runtime-detected format is not visible to internal/check (WIRE.md §6.1).
func formatFits(format wireFormat, header bool, t types.Type) bool {
	switch format {
	case fmtText:
		return t.Base().Kind() == types.String
	case fmtCSV:
		return csvFits(header, t)
	default:
		return true
	}
}

// csvFits is WIRE.md §6.6: header:true builds a collection of records, header:false [[String]].
func csvFits(header bool, t types.Type) bool {
	if !header {
		return stringRows(t)
	}
	return recordRows(t)
}

// stringRows reports [[String]], what a headerless csv (detected or load.csv) builds (WIRE.md §6.6).
func stringRows(t types.Type) bool {
	rows, ok := t.Base().(*types.ListType)
	if !ok || rows.KeyedBy != nil {
		return false
	}
	row, ok := rows.Elem.Base().(*types.ListType)
	return ok && row.KeyedBy == nil && types.Identical(row.Elem, types.StringType)
}

// recordRows reports a List or Table of Record elements, what a headered csv builds (WIRE.md §6.6).
func recordRows(t types.Type) bool {
	switch b := t.Base().(type) {
	case *types.ListType:
		return isRecordKind(b.Elem)
	case *types.TableType:
		return isRecordKind(b.Elem)
	default:
		return false
	}
}

// isRecordKind reports a plain or applied record type.
func isRecordKind(t types.Type) bool {
	switch t.Base().(type) {
	case *types.RecordType, *types.AppliedRecord:
		return true
	default:
		return false
	}
}

// atStep is one step of a parsed `at:` path: a member name, an array index, or `*` (WIRE.md §6.3).
type atStep struct {
	kind  atStepKind
	name  string
	index int
}

// parseAt reads path per WIRE.md §6.3's grammar; ok is false for an empty or malformed path.
func parseAt(path string) ([]atStep, bool) {
	if path == "" {
		return nil, false
	}
	step, rest, ok := atFirst(path)
	if !ok {
		return nil, false
	}
	steps := []atStep{step}
	for rest != "" {
		next, tail, ok := atStepAfter(rest)
		if !ok {
			return nil, false
		}
		steps, rest = append(steps, next), tail
	}
	return steps, true
}

func atFirst(s string) (atStep, string, bool) {
	switch s[0] {
	case '*':
		return atStep{kind: atStar}, s[1:], true
	case atOpenIndex:
		return atIndexOf(s)
	default:
		return atNameOf(s)
	}
}

// atStepAfter is one "next" step: "." (name|"*"), or "[n]" directly (WIRE.md §6.3).
func atStepAfter(s string) (atStep, string, bool) {
	switch s[0] {
	case '.':
		return atDotted(s[1:])
	case atOpenIndex:
		return atIndexOf(s)
	default:
		return atStep{}, "", false
	}
}

func atDotted(s string) (atStep, string, bool) {
	if s == "" {
		return atStep{}, "", false
	}
	if s[0] == '*' {
		return atStep{kind: atStar}, s[1:], true
	}
	return atNameOf(s)
}

// atIndexOf reads a leading "[0]" or "[nonzero digits]" (WIRE.md §6.3); s[0] is "[".
func atIndexOf(s string) (atStep, string, bool) {
	end, ok := atIndexEnd(s)
	if !ok {
		return atStep{}, "", false
	}
	n, err := strconv.Atoi(s[1:end])
	if err != nil {
		return atStep{}, "", false
	}
	return atStep{kind: atIndex, index: n}, s[end+1:], true
}

// atIndexEnd is the index of "]" closing s's leading digits, or false when malformed.
func atIndexEnd(s string) (int, bool) {
	i := 1
	switch {
	case i >= len(s) || !isDigit(s[i]):
		return 0, false
	case s[i] == atZeroDigit:
		i++
	default:
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	}
	if i >= len(s) || s[i] != atCloseIndex {
		return 0, false
	}
	return i, true
}

// atNameOf reads a leading name, `\` escaping one of ". * [ ] \" (WIRE.md §6.3).
func atNameOf(s string) (atStep, string, bool) {
	var b strings.Builder
	i := 0
	for i < len(s) && !strings.ContainsRune(atStopChars, rune(s[i])) {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			i++
			continue
		}
		if i+1 >= len(s) || !strings.ContainsRune(atEscapable, rune(s[i+1])) {
			return atStep{}, "", false
		}
		b.WriteByte(s[i+1])
		i += twoBytes
	}
	if b.Len() == 0 {
		return atStep{}, "", false
	}
	return atStep{kind: atName, name: b.String()}, s[i:], true
}

// applyAt selects path's value from root, E7106 at the JSON value where it failed (WIRE.md §6.3).
func applyAt(root *jsonsrc.Node, path string, req Request) (wire.Selection, bool) {
	steps, ok := parseAt(path)
	if !ok {
		diag.E7106.AtSyntax(req.Span, path).Report(req.Bag)
		return wire.Selection{}, false
	}
	return atApply(wire.Selection{Node: root}, steps, path, req)
}

func atApply(sel wire.Selection, steps []atStep, path string, req Request) (wire.Selection, bool) {
	if len(steps) == 0 {
		return sel, true
	}
	step := steps[0]
	switch step.kind {
	case atName:
		return atApplyName(sel, step.name, steps[1:], path, req)
	case atIndex:
		return atApplyIndex(sel, step.index, steps[1:], path, req)
	default:
		return atApplyStar(sel, steps[1:], path, req)
	}
}

func atApplyName(sel wire.Selection, name string, rest []atStep, path string, req Request) (wire.Selection, bool) {
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

func atApplyIndex(sel wire.Selection, index int, rest []atStep, path string, req Request) (wire.Selection, bool) {
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

func atApplyStar(sel wire.Selection, rest []atStep, path string, req Request) (wire.Selection, bool) {
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
func atStarItems(n int, at func(int) *jsonsrc.Node, rest []atStep, path string, req Request) ([]wire.Selection, bool) {
	items, ok := make([]wire.Selection, n), true
	for i := range items {
		item, itemOK := atApply(wire.Selection{Node: at(i)}, rest, path, req)
		items[i], ok = item, ok && itemOK
	}
	return items, ok
}

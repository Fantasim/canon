package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// emitOpen is `open: [E, …]` of a go emit: in types mode only, a non-empty list of the package's public enums, each listed once and none ordered (E8009, DECISIONS 339).
func (c *checker) emitOpen(env *env, e *syntax.EmitDecl, fi *syntax.FieldItem) {
	if mode, known := emitModeWord(e); known && mode != ModeTypes {
		c.report(env, diag.E8009.AtOpenMode(env.span(fi.Name), mode))
	}
	list, ok := fi.Value.(*syntax.ListLit)
	if !ok {
		c.report(env, diag.E8009.AtKind(env.span(fi.Value), fi.Name.Name, TargetGo, diag.KindEnumNames))
		return
	}
	if len(list.Elems) == 0 { // omit open to keep every enum closed
		c.report(env, diag.E8009.AtOpenEmpty(env.span(list)))
		return
	}
	seen := map[string]bool{}
	for _, x := range list.Elems {
		c.openName(env, x, fi.Name.Name, seen)
	}
}

// openName is one name of open: a word naming a public enum of the package, not listed before, not ordered (E8009 `open`, `openTwice`, `openOrdered`; DECISIONS 339).
func (c *checker) openName(env *env, x syntax.Expr, option string, seen map[string]bool) {
	id, isName := x.(*syntax.IdentExpr)
	if !isName {
		c.report(env, diag.E8009.AtKind(env.span(x), option, TargetGo, diag.KindEnumNames))
		return
	}
	o := env.pkg.names[id.Name]
	d, isEnum := publicEnum(o)
	if isEnum {
		c.info.Uses[id] = o // recorded for the name index, as values' names are (DECISIONS 275)
	}
	switch {
	case !isEnum:
		c.report(env, diag.E8009.AtOpen(env.span(id), id.Name))
	case seen[id.Name]:
		c.report(env, diag.E8009.AtOpenTwice(env.span(id), id.Name))
	case d.Ordered.Valid(): // a string type would compare its members wrongly
		c.report(env, diag.E8009.AtOpenOrdered(env.span(id), id.Name))
	}
	seen[id.Name] = true
}

// publicEnum is o's declaration when o is a public enum of the package.
func publicEnum(o *object) (*syntax.EnumDecl, bool) {
	if o == nil || o.kind != ObjTypeName || o.local {
		return nil, false
	}
	d, ok := o.decl.(*syntax.EnumDecl)
	return d, ok
}

// emitModeWord is the mode a go emit is in: its written mode, else the default baked; false for a mode check refuses (E8009 `mode`).
func emitModeWord(e *syntax.EmitDecl) (string, bool) {
	opt := emitOption(e, OptMode)
	if opt == nil {
		return ModeBaked, true
	}
	id, ok := opt.(*syntax.IdentExpr)
	if !ok || !slices.Contains(emitSpecs[TargetGo].modes, id.Name) {
		return "", false
	}
	return id.Name, true
}

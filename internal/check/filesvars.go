package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// recordFilesVars records the fields each `@files` template of p names: `{f}` in Uses, each
// `.g` of `{f.g}` in NameUses; `{id}` is the key and names nothing (API.md N2, DECISIONS 275).
func (c *checker) recordFilesVars(p *pkgState) {
	for _, o := range p.all {
		if d, isLet := o.decl.(*syntax.LetDecl); isLet && o.typ != nil {
			c.filesVars(d, o.typ)
		}
	}
}

// filesVars records the names of the `@files` template of d, a let of type t, if it has one.
func (c *checker) filesVars(d *syntax.LetDecl, t types.Type) {
	tpl, isStr := firstArg(annotation(d.Annotations, syntax.AnnFiles)).(*syntax.StringLit)
	elem, _, isColl := collectionElem(t)
	if !isStr || !isColl {
		return
	}
	for _, part := range tpl.Parts {
		if part.Interp != nil {
			c.templatePath(templatePath(part.Interp.X), []types.Type{elem})
		}
	}
}

// templatePath records each name of path naming one field across the types it may be read on;
// a name several cases declare is recorded nowhere, and the names after it are read in each.
func (c *checker) templatePath(path []syntax.Node, ts []types.Type) {
	for _, n := range path {
		fields := pathFields(ts, nameText(n))
		if len(fields) == 1 && c.fieldObjects[fields[0]] != nil {
			switch n := n.(type) {
			case *syntax.IdentExpr:
				c.info.Uses[n] = c.fieldObjects[fields[0]]
			case *syntax.Ident:
				c.info.NameUses[n] = c.fieldObjects[fields[0]]
			}
		}
		ts = fieldTypes(fields)
	}
}

// templatePath is an interpolation's names, `f` then each `.g`; none for `{id}`, the key (API.md
// N2), or for anything but a name or a field path.
func templatePath(x syntax.Expr) []syntax.Node {
	path := namePath(x)
	if len(path) == 1 && nameText(path[0]) == idMember {
		return nil
	}
	return path
}

func namePath(x syntax.Expr) []syntax.Node {
	switch x := x.(type) {
	case *syntax.IdentExpr:
		return []syntax.Node{x}
	case *syntax.SelectorExpr:
		if inner := namePath(x.X); inner != nil && !x.Optional {
			return append(inner, x.Name)
		}
	}
	return nil
}

func nameText(n syntax.Node) string {
	switch n := n.(type) {
	case *syntax.IdentExpr:
		return n.Name
	case *syntax.Ident:
		return n.Name
	}
	return ""
}

// pathFields are the distinct fields name names on a value of any of ts, an optional's present
// value, a record, a case, or any case of a variant.
func pathFields(ts []types.Type, name string) []*types.Field {
	var out []*types.Field
	for _, t := range ts {
		for _, fields := range fieldSets(unwrapOptional(t)) {
			if f := fieldNamed(fields, name); f != nil && !slices.Contains(out, f) {
				out = append(out, f)
			}
		}
	}
	return out
}

func fieldTypes(fs []*types.Field) []types.Type {
	out := make([]types.Type, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Type)
	}
	return out
}

// fieldSets are the field lists a value of type t may have: a record's, a case's, or each case's
// of a variant.
func fieldSets(t types.Type) [][]*types.Field {
	switch x := t.Base().(type) {
	case *types.RecordType:
		return [][]*types.Field{x.Fields}
	case *types.AppliedRecord:
		return [][]*types.Field{x.Rec.Fields}
	case *types.CaseType:
		return [][]*types.Field{x.Fields}
	case *types.VariantType:
		out := make([][]*types.Field, 0, len(x.Cases))
		for _, cs := range x.Cases {
			out = append(out, cs.Fields)
		}
		return out
	}
	return nil
}

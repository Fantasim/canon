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
		c.filesVars(o)
	}
}

// filesVars records o's `@files` names; E2102 at a name out of scope (DECISIONS 277).
func (c *checker) filesVars(o *object) {
	d, isLet := o.decl.(*syntax.LetDecl)
	if !isLet || o.typ == nil {
		return
	}
	tpl, isStr := firstArg(annotation(d.Annotations, syntax.AnnFiles)).(*syntax.StringLit)
	elem, _, isColl := collectionElem(o.typ)
	if !isStr || !isColl {
		return
	}
	for _, part := range tpl.Parts {
		if part.Interp == nil {
			continue
		}
		if n, scope := c.templatePath(templatePath(part.Interp.X), []types.Type{elem}); n != nil {
			c.unknownHinted(c.declEnv(o), n, nameText(n), nearest(nameText(n), scope))
		}
	}
}

// templatePath records path's names read on ts, then the first name no candidate declares and
// the names in scope there; a name several cases declare is recorded nowhere.
func (c *checker) templatePath(path []syntax.Node, ts []types.Type) (syntax.Node, []string) {
	if len(path) > 1 && nameText(path[0]) == idMember {
		return path[1], nil // DECISIONS 280: the key has no fields, `x` of `{id.x}` is outside
	}
	for i, n := range path {
		if !fieldsListed(ts) {
			return nil, nil
		}
		fields := pathFields(ts, nameText(n), fieldSets)
		if len(fields) == 0 {
			return n, scopeNames(ts, i == 0)
		}
		c.recordTemplateName(n, fields)
		ts = fieldTypes(fields)
	}
	return nil, nil
}

// recordTemplateName records n under the field it names, when it names one.
func (c *checker) recordTemplateName(n syntax.Node, fields []*types.Field) {
	if len(fields) != 1 || c.fieldObjects[fields[0]] == nil {
		return
	}
	switch n := n.(type) {
	case *syntax.IdentExpr:
		c.info.Uses[n] = c.fieldObjects[fields[0]]
	case *syntax.Ident:
		c.info.NameUses[n] = c.fieldObjects[fields[0]]
	}
}

// fieldsListed: no error type, broken branch or `_` (TYPES.md §1, DECISIONS 280).
func fieldsListed(ts []types.Type) bool {
	for _, t := range ts {
		if holdsError(t) || brokenBranch(t, map[*types.TypeFunc]bool{}) || unwrapOptional(t).Base().Kind() == types.Any {
			return false
		}
	}
	return len(ts) > 0
}

// brokenBranch reports a dependent type t one of whose branches, through nested applications, is
// missing or holds the error type.
func brokenBranch(t types.Type, seen map[*types.TypeFunc]bool) bool {
	fn := typeFunc(unwrapOptional(t))
	if fn == nil || seen[fn] {
		return false
	}
	seen[fn] = true
	return slices.ContainsFunc(branchResults(fn), func(r types.Type) bool {
		return r == nil || holdsError(r) || brokenBranch(r, seen)
	})
}

// scopeNames are the names a template variable may use on a value of any of ts: their fields,
// and the key `id` first (API.md N2).
func scopeNames(ts []types.Type, first bool) []string {
	var out []string
	if first {
		out = append(out, idMember)
	}
	for _, t := range ts {
		for _, fields := range fieldSets(unwrapOptional(t)) {
			for _, f := range fields {
				out = append(out, f.Name)
			}
		}
	}
	return out
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
// value, read by sets: a record, a case, any case of a variant, and for fieldSets any branch.
func pathFields(ts []types.Type, name string, sets func(types.Type) [][]*types.Field) []*types.Field {
	var out []*types.Field
	for _, t := range ts {
		for _, fields := range sets(unwrapOptional(t)) {
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

// fieldSets are the field lists a value of type t may have in a `@files` path: caseFieldSets,
// or each branch's of a dependent type (DECISIONS 280).
func fieldSets(t types.Type) [][]*types.Field {
	return branchFieldSets(t, map[*types.TypeFunc]bool{})
}

// branchFieldSets is fieldSets, each type function's branches read once.
func branchFieldSets(t types.Type, seen map[*types.TypeFunc]bool) [][]*types.Field {
	fn := typeFunc(t)
	if fn == nil {
		return caseFieldSets(t)
	}
	if seen[fn] {
		return nil
	}
	seen[fn] = true
	var out [][]*types.Field
	for _, r := range branchResults(fn) {
		if r != nil {
			out = append(out, branchFieldSets(unwrapOptional(r), seen)...)
		}
	}
	return out
}

// caseFieldSets are the field lists a value of type t may have: a record's, a case's, or each
// case's of a variant.
func caseFieldSets(t types.Type) [][]*types.Field {
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

// typeFunc is the type function of a dependent type t, an application or its union; else nil.
func typeFunc(t types.Type) *types.TypeFunc {
	switch x := t.Base().(type) {
	case *types.TypeAppType:
		return x.Fn
	case *types.DepUnionType:
		return x.Fn
	}
	return nil
}

// branchResults are the result types of fn's branches: its body, or each arm's (nil if broken).
func branchResults(fn *types.TypeFunc) []types.Type {
	if fn.Body != nil {
		return []types.Type{fn.Body}
	}
	out := make([]types.Type, 0, len(fn.Arms))
	for _, a := range fn.Arms {
		out = append(out, a.Result)
	}
	return out
}

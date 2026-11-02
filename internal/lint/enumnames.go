package lint

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// enumNames reports W0003 (API.md §5.7, rule L4): a sound `type` that refines a String to the member names of one enum or of several.
func enumNames(p *pass, pkg *check.Package, bag *diag.Bag) {
	for _, f := range sources(pkg) {
		for _, d := range f.Decls {
			td, ok := d.(*syntax.TypeDecl)
			if !ok || td.Name == nil {
				continue
			}
			enums, found := p.limitedTo(td)
			span := f.Span(td.Name)
			switch {
			case !found:
			case len(enums) == 1:
				diag.W0003.AtSingle(span, td.Name.Name, enums[0]).Report(bag)
			default:
				diag.W0003.AtSeveral(span, td.Name.Name, enums).Report(bag)
			}
		}
	}
}

// limitedTo are the enums whose member names td limits a String to: `String where it in X` or `X.contains(it)`.
func (p *pass) limitedTo(td *syntax.TypeDecl) ([]string, bool) {
	where, ok := td.Type.(*syntax.WhereType)
	t := p.declared(td.Name)
	if !ok || t == nil || len(td.Params) > 0 || t.Kind() != types.String {
		return nil, false
	}
	list, ok := membershipList(where.Pred)
	if !ok {
		return nil, false
	}
	return p.enumsOf(list)
}

// membershipList is X in the predicates `it in X` and `X.contains(it)`.
func membershipList(pred syntax.Expr) (syntax.Expr, bool) {
	switch x := syntax.Unparen(pred).(type) {
	case *syntax.BinaryExpr:
		return x.Y, x.Op == syntax.KwIn && isName(x.X, valueName)
	case *syntax.CallExpr:
		sel, ok := x.Fun.(*syntax.SelectorExpr)
		if ok && sel.Name != nil && sel.Name.Name == containsName && len(x.Args) == 1 && isName(x.Args[0].Value, valueName) {
			return sel.X, true
		}
	}
	return nil, false
}

// isName reports x the name called name.
func isName(x syntax.Expr, name string) bool {
	id, ok := syntax.Unparen(x).(*syntax.IdentExpr)
	return ok && id.Name == name
}

// enumsOf are the enums whose member names make up the list or map x, in first-use order, and false when x holds anything else:
// `[m.name for m in E.members …]`, a `+` of such lists, `.unique()` of one, `{n: true for n in …}` over one, and a let of these.
func (p *pass) enumsOf(x syntax.Expr) ([]string, bool) {
	switch x := syntax.Unparen(x).(type) {
	case *syntax.ListComp:
		return p.comprehended(x.Elem, x.Clauses)
	case *syntax.BraceLit:
		return p.mapComprehended(x)
	case *syntax.BinaryExpr:
		return p.sumOf(x)
	case *syntax.CallExpr:
		return p.uniqueOf(x)
	case *syntax.IdentExpr:
		return p.letEnums(x)
	}
	return nil, false
}

// comprehended are the enums of a comprehension whose elements, one per member or per name, are a member's name: it loops
// over a list of an enum's members (`v.name`) or over a list of such names (`v`), and its other clauses are filters.
func (p *pass) comprehended(elem syntax.Expr, clauses []*syntax.CompClause) ([]string, bool) {
	if len(clauses) == 0 || clauses[0].Keyword != syntax.KwFor || len(clauses[0].Vars) != 1 {
		return nil, false
	}
	for _, c := range clauses[1:] {
		if c.Keyword != syntax.KwIf {
			return nil, false
		}
	}
	v := clauses[0].Vars[0].Name
	switch {
	case isMemberName(elem, v):
		return p.enumOfMembers(clauses[0].X)
	case isName(elem, v):
		return p.enumsOf(clauses[0].X)
	}
	return nil, false
}

// isMemberName reports x the selector `v.name`.
func isMemberName(x syntax.Expr, v string) bool {
	sel, ok := syntax.Unparen(x).(*syntax.SelectorExpr)
	return ok && !sel.Optional && sel.Name != nil && sel.Name.Name == enumMemberName && isName(sel.X, v)
}

// mapComprehended are the enums of a map `{n: true for n in …}` whose keys are the member names of enums.
func (p *pass) mapComprehended(x *syntax.BraceLit) ([]string, bool) {
	if len(x.Items) != 1 || len(x.Clauses) == 0 {
		return nil, false
	}
	item, ok := x.Items[0].(*syntax.MapItem)
	if !ok {
		return nil, false
	}
	if lit, isBool := item.Value.(*syntax.BoolLit); !isBool || !lit.Value {
		return nil, false
	}
	return p.comprehended(item.Key, x.Clauses)
}

// enumOfMembers is the enum whose members list is, a list the checker typed `[E]`.
func (p *pass) enumOfMembers(list syntax.Expr) ([]string, bool) {
	lt, ok := p.prog.Info.Types[list].(*types.ListType)
	if !ok {
		return nil, false
	}
	en, ok := lt.Elem.Base().(*types.EnumType)
	if !ok {
		return nil, false
	}
	return []string{en.Name}, true
}

// sumOf are the enums of `a + b`, both lists of member names.
func (p *pass) sumOf(x *syntax.BinaryExpr) ([]string, bool) {
	if x.Op != syntax.TokPlus {
		return nil, false
	}
	left, okLeft := p.enumsOf(x.X)
	right, okRight := p.enumsOf(x.Y)
	if !okLeft || !okRight {
		return nil, false
	}
	for _, name := range right {
		if !slices.Contains(left, name) {
			left = append(left, name)
		}
	}
	return left, true
}

// uniqueOf are the enums of `list.unique()`.
func (p *pass) uniqueOf(x *syntax.CallExpr) ([]string, bool) {
	sel, ok := x.Fun.(*syntax.SelectorExpr)
	if !ok || sel.Name == nil || sel.Name.Name != uniqueName || len(x.Args) > 0 {
		return nil, false
	}
	return p.enumsOf(sel.X)
}

// letEnums are the enums of the sound let that name x refers to.
func (p *pass) letEnums(x *syntax.IdentExpr) ([]string, bool) {
	info := p.prog.Info
	obj := info.Uses[x]
	if obj == nil || obj.Kind() != check.ObjLet || info.Broken[obj] {
		return nil, false
	}
	let, ok := obj.Decl().(*syntax.LetDecl)
	if !ok {
		return nil, false
	}
	if p.following[obj] { // a let that names itself, directly or through others, lists nothing
		return nil, false
	}
	p.following[obj] = true
	defer delete(p.following, obj)
	return p.enumsOf(let.Value)
}

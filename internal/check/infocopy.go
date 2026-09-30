package check

import (
	"maps"

	"github.com/fantasim/canonlang/internal/syntax"
)

// cloned is a copy of i whose maps Recheck may change, with broken and views as its Broken and
// BrokenViews: the program holding i keeps it as it was.
func (i *Info) cloned(broken map[Object]bool, views map[*syntax.ViewDecl]bool) *Info {
	return &Info{
		Types:       maps.Clone(i.Types),
		TypeExprs:   maps.Clone(i.TypeExprs),
		Defs:        maps.Clone(i.Defs),
		Uses:        maps.Clone(i.Uses),
		NameUses:    maps.Clone(i.NameUses),
		Selections:  maps.Clone(i.Selections),
		Conv:        maps.Clone(i.Conv),
		Keys:        maps.Clone(i.Keys),
		Symbols:     maps.Clone(i.Symbols),
		Calls:       maps.Clone(i.Calls),
		Literals:    maps.Clone(i.Literals),
		Matches:     maps.Clone(i.Matches),
		Broken:      maps.Clone(broken),
		BrokenViews: maps.Clone(views),

		BrokenTranslations: maps.Clone(i.BrokenTranslations),
	}
}

// forget drops every fact about n.
func (i *Info) forget(n syntax.Node) {
	if e, ok := n.(syntax.Expr); ok {
		delete(i.Types, e)
		delete(i.Conv, e)
		delete(i.Keys, e)
	}
	if t, ok := n.(syntax.Type); ok {
		delete(i.TypeExprs, t)
	}
	switch x := n.(type) {
	case *syntax.Ident:
		delete(i.Defs, x)
		delete(i.NameUses, x)
	case *syntax.IdentExpr:
		delete(i.Uses, x)
		delete(i.Symbols, x)
	case *syntax.SelectorExpr:
		delete(i.Selections, x)
	case *syntax.CallExpr:
		delete(i.Calls, x)
	case *syntax.BraceLit:
		delete(i.Literals, x)
	case *syntax.ViewDecl:
		delete(i.BrokenViews, x)
	case *syntax.TranslationEntry:
		delete(i.BrokenTranslations, x)
	}
	delete(i.Matches, n)
}

// renewal is the new object of an old one, nil or itself when it keeps its place.
type renewal func(*object) *object

// byMap is the renewal renew lists.
func byMap(renew map[*object]*object) renewal {
	return func(o *object) *object { return renew[o] }
}

// renew points the names, selections and calls naming an old object at its new one; a
// Selection or Callee is copied, never changed, as the old program holds it.
func (i *Info) renew(to renewal) {
	for k, o := range i.Uses { //canon:unordered each entry is renewed in place
		if n := renewed(to, o); n != nil {
			i.Uses[k] = n
		}
	}
	for k, o := range i.NameUses { //canon:unordered each entry is renewed in place
		if n := renewed(to, o); n != nil {
			i.NameUses[k] = n
		}
	}
	for k, s := range i.Selections { //canon:unordered each entry is renewed in place
		if n := renewed(to, s.Obj); n != nil {
			moved := *s
			moved.Obj = n
			i.Selections[k] = &moved
		}
	}
	for k, cl := range i.Calls { //canon:unordered each entry is renewed in place
		if n := renewed(to, cl.Obj); n != nil {
			moved := *cl
			moved.Obj = n
			i.Calls[k] = &moved
		}
	}
}

// renewed is the new object of o, nil when o keeps its place.
func renewed(to renewal, o Object) *object {
	x, ok := o.(*object)
	if !ok || x == nil {
		return nil
	}
	if n := to(x); n != x {
		return n
	}
	return nil
}

// move gives node n the facts Info holds about old, a node of the same kind (a let's signature
// checked again, NFR-02).
func (i *Info) move(old, n syntax.Node) {
	if e, ok := old.(syntax.Expr); ok {
		moveFact(i.Types, e, n.(syntax.Expr))
		moveFact(i.Conv, e, n.(syntax.Expr))
		moveFact(i.Keys, e, n.(syntax.Expr))
	}
	if t, ok := old.(syntax.Type); ok {
		moveFact(i.TypeExprs, t, n.(syntax.Type))
	}
	switch x := old.(type) {
	case *syntax.Ident:
		moveFact(i.Defs, x, n.(*syntax.Ident))
		moveFact(i.NameUses, x, n.(*syntax.Ident))
	case *syntax.IdentExpr:
		moveFact(i.Uses, x, n.(*syntax.IdentExpr))
		moveFact(i.Symbols, x, n.(*syntax.IdentExpr))
	case *syntax.SelectorExpr:
		moveFact(i.Selections, x, n.(*syntax.SelectorExpr))
	case *syntax.CallExpr:
		moveFact(i.Calls, x, n.(*syntax.CallExpr))
	case *syntax.BraceLit:
		moveFact(i.Literals, x, n.(*syntax.BraceLit))
	}
	moveFact(i.Matches, old, n)
}

// moveFact copies m's fact about old to n.
func moveFact[K comparable, V any](m map[K]V, old, n K) {
	if v, ok := m[old]; ok {
		m[n] = v
	}
}

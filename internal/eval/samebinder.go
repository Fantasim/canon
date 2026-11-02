package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// sameUpToBinder reports a and b one dependent map type once one's binders are renamed to the other's (DECISIONS 446).
func sameUpToBinder(a *types.DepMapType, b types.Type) bool {
	d, ok := b.Base().(*types.DepMapType)
	if !ok || a.Coll != d.Coll {
		return false
	}
	w := binderWalk{a: []string{a.Binder}, b: []string{d.Binder}}
	return w.same(a.Value, d.Value)
}

// binderWalk is a parallel walk of two types: the binders in scope on each side, innermost first.
type binderWalk struct {
	a, b []string
}

// appShape is what the walk compares of a type as written: its kind, what it applies, is keyed
// by or names, its refinement, its component types, its type arguments and the binder it
// introduces; the rest is Identical's.
type appShape struct {
	kind   types.Kind
	head   any
	ref    *types.Refined
	parts  []types.Type
	args   []*types.Arg
	binder string
}

// aliasName is an alias as written: its qualified name.
type aliasName struct {
	pkg, name string
}

// shapeOf is t's appShape, its aliases and refinements as written.
func shapeOf(t types.Type) appShape {
	switch x := t.(type) {
	case *types.Alias:
		return appShape{kind: x.Kind(), head: aliasName{pkg: x.Pkg, name: x.Name}}
	case *types.Refined:
		return appShape{kind: x.Kind(), ref: x, parts: []types.Type{x.Of}}
	case *types.OptionalType:
		return appShape{kind: x.Kind(), parts: []types.Type{x.Elem}}
	case *types.ListType:
		return appShape{kind: x.Kind(), parts: []types.Type{x.Elem}}
	case *types.MapType:
		return appShape{kind: x.Kind(), parts: []types.Type{x.Key, x.Value}}
	case *types.DepMapType:
		return appShape{kind: x.Kind(), head: x.Coll, parts: []types.Type{x.Value}, binder: x.Binder}
	case *types.PairType:
		return appShape{kind: x.Kind(), parts: []types.Type{x.A, x.B}}
	case *types.LitUnionType:
		return appShape{kind: x.Kind(), parts: []types.Type{x.Of}}
	case *types.AppliedRecord:
		return appShape{kind: x.Kind(), head: x.Rec, args: x.Args}
	case *types.TypeAppType:
		return appShape{kind: x.Kind(), head: x.Fn, args: x.Args}
	}
	return appShape{kind: t.Kind()}
}

// same reports x and y the same type up to their binders' names: the same kind and head, each
// application with the same function and argument count, and each argument the same (sameArg).
func (w binderWalk) same(x, y types.Type) bool {
	if x == nil || y == nil {
		return x == nil && y == nil
	}
	px, py := shapeOf(x), shapeOf(y)
	if px.kind != py.kind || px.head != py.head || len(px.parts) != len(py.parts) || len(px.args) != len(py.args) {
		return false
	}
	if !both(px.ref, py.ref, sameRefinement) {
		return false
	}
	for i, a := range px.args {
		if !w.sameArg(a, py.args[i]) {
			return false
		}
	}
	inner := w
	if px.binder != "" {
		inner = binderWalk{a: append([]string{px.binder}, w.a...), b: append([]string{py.binder}, w.b...)}
	}
	for i, p := range px.parts {
		if !inner.same(p, py.parts[i]) {
			return false
		}
	}
	return true
}

// sameArg reports a and b one argument: a binder is the same when both name the binder at one
// depth, or both name one outside the walk by the same name.
func (w binderWalk) sameArg(a, b *types.Arg) bool {
	i, j := slices.Index(w.a, a.Binder), slices.Index(w.b, b.Binder)
	return a.Source == b.Source && a.Param == b.Param && slices.Equal(a.Path, b.Path) &&
		i == j && (i >= 0 || a.Binder == b.Binder)
}

// both reports a and b both nil, or both set and the same by eq.
func both[T any](a, b *T, eq func(x, y *T) bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return eq(a, b)
}

// sameRefinement reports two refinements written the same: past, range, pattern, asset and where text.
func sameRefinement(a, b *types.Refined) bool {
	return a.Past == b.Past && both(a.Range, b.Range, func(x, y *types.Bound) bool { return *x == *y }) &&
		both(a.Where, b.Where, func(x, y *types.Predicate) bool { return x.Text == y.Text }) &&
		both(a.Asset, b.Asset, func(x, y *types.AssetSpec) bool { return x.Root == y.Root && slices.Equal(x.Exts, y.Exts) }) &&
		(a.Pattern == nil) == (b.Pattern == nil) && (a.Pattern == nil || a.Pattern.String() == b.Pattern.String())
}

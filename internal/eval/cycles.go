package eval

import (
	"cmp"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// hop is one step of a cycle between consts: a const, and the use in its initializer that
// names the next one.
type hop struct {
	obj check.Object
	use syntax.Node
}

// constCycle is E4301 for a cycle of consts the checker broke silently (DECISIONS 172, 187).
func (e *Evaluator) constCycle(obj check.Object) {
	if obj == nil || obj.Kind() != check.ObjConst || e.info == nil {
		return
	}
	hops := e.constPath(obj, obj, map[check.Object]bool{obj: true})
	if len(hops) == 0 || !samePackage(hops) {
		return
	}
	first := 0
	for i, h := range hops {
		if declBefore(h.obj, hops[first].obj) {
			first = i
		}
	}
	hops = append(hops[first:], hops[:first]...)
	last := hops[len(hops)-1]
	pkg := last.obj.Pkg()
	names := make([]string, 0, len(hops)+1)
	for _, h := range append(hops, hops[0]) {
		names = append(names, qualify(pkg, h.obj.Pkg(), h.obj.Name()))
	}
	if last.obj.File() != nil {
		e.report(pkg, diag.E4301.At(last.obj.File().Span(last.use), names).Path(rootPath(last.obj.Name()).String()))
	}
}

// samePackage reports a cycle inside one package: one across packages is E2002's (TYPES.md §3.1).
func samePackage(hops []hop) bool {
	for _, h := range hops {
		if h.obj.Pkg() != hops[0].obj.Pkg() {
			return false
		}
	}
	return true
}

// constPath is a path of hops from cur back to target, depth first in source order.
func (e *Evaluator) constPath(cur, target check.Object, seen map[check.Object]bool) []hop {
	d, ok := cur.Decl().(*syntax.ConstDecl)
	if !ok {
		return nil
	}
	for _, u := range e.constUses(d.Value) {
		next := e.named(u)
		if next == target {
			return []hop{{obj: cur, use: u}}
		}
		if seen[next] {
			continue
		}
		seen[next] = true
		if rest := e.constPath(next, target, seen); rest != nil {
			return append([]hop{{obj: cur, use: u}}, rest...)
		}
	}
	return nil
}

// constUses are the names of consts in an initializer, in source order.
func (e *Evaluator) constUses(x syntax.Expr) []syntax.Node {
	var out []syntax.Node
	syntax.Inspect(x, func(n syntax.Node) bool {
		if obj := e.named(n); obj != nil && obj.Kind() == check.ObjConst {
			out = append(out, n)
		}
		return true
	})
	return out
}

// named is the object a name or a qualified form `pkg.C` names.
func (e *Evaluator) named(n syntax.Node) check.Object {
	switch x := n.(type) {
	case *syntax.IdentExpr:
		return e.info.Uses[x]
	case *syntax.SelectorExpr:
		if e.info.Selections[x] == nil {
			return e.info.NameUses[x.Name]
		}
	}
	return nil
}

// declBefore orders declarations by file path bytes, then position (EVALUATION.md §2.1).
func declBefore(a, b check.Object) bool {
	fa, fb := a.File(), b.File()
	if fa == nil || fb == nil {
		return false
	}
	if c := cmp.Compare(fa.Src.Path, fb.Src.Path); c != 0 {
		return c < 0
	}
	return fa.Span(a.Decl()).Start < fb.Span(b.Decl()).Start
}

// qualify names a declaration of pkg as package from sees it.
func qualify(from, pkg, name string) string {
	if pkg == "" || pkg == from {
		return name
	}
	return pkg + dot + name
}

// report files a finding of pkg in its own bag; a vector keeps only its first error (CONFORMANCE.md §6.5).
func (e *Evaluator) report(pkg string, b *diag.Builder) {
	if e.aside != nil {
		if bag := e.aside[pkg]; bag != nil {
			b.Report(bag)
		}
		return
	}
	if e.vec != nil {
		e.vec.note(e, pkg, b)
		return
	}
	if bag := e.bags[pkg]; bag != nil {
		b.Report(bag)
	}
}

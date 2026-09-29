package canon

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// cause explains why root has no value, from evaluation, else from the checker that broke it (rule R6).
func (s *snapshot) cause(root eval.Root) ([]Finding, error) {
	c, err := s.a.Cause(s.ctx, root)
	if err != nil || len(c) > 0 {
		return fromDiag(s.a.Files(), c), err
	}
	obj := s.object(root)
	if obj == nil {
		return nil, nil
	}
	return fromDiag(s.a.Files(), s.brokenBy(obj, map[check.Object]bool{})), nil
}

// object is the let or const root names.
func (s *snapshot) object(root eval.Root) check.Object {
	for _, pkg := range s.a.Program().Packages {
		if pkg.Path != root.Pkg {
			continue
		}
		for _, obj := range pkg.Decls {
			if obj.Name() == root.Name && (obj.Kind() == check.ObjLet || obj.Kind() == check.ObjConst) {
				return obj
			}
		}
	}
	return nil
}

// brokenBy is the errors inside obj's declaration and its active layers' amendments; with none
// there, those of the broken declarations it names, the dependency that broke it.
func (s *snapshot) brokenBy(obj check.Object, seen map[check.Object]bool) []diag.Finding {
	seen[obj] = true
	nodes := s.sources(obj)
	var own []diag.Finding
	for _, f := range s.a.Result().List {
		if f.Severity == diag.Error && slices.ContainsFunc(nodes, func(n located) bool { return n.holds(f.Span) }) {
			own = append(own, f)
		}
	}
	if len(own) > 0 {
		return own
	}
	var out []diag.Finding
	for _, d := range s.brokenNames(nodes) {
		if !seen[d] {
			out = append(out, s.brokenBy(d, seen)...)
		}
	}
	return out
}

// located is a node and the file it is in.
type located struct {
	f *syntax.File
	n syntax.Node
}

func (l located) holds(sp source.Span) bool {
	outer := l.f.Span(l.n)
	return sp.File == outer.File && outer.Start <= sp.Start && sp.End <= outer.End
}

// sources are obj's declaration and, for a let, the amend blocks of its active layers (EVALUATION.md §9.3).
func (s *snapshot) sources(obj check.Object) []located {
	var out []located
	if obj.File() != nil {
		out = append(out, located{obj.File(), obj.Decl()})
	}
	for _, pkg := range s.a.Program().Packages {
		if pkg.Path == obj.Pkg() {
			out = append(out, s.amends(pkg, obj)...)
		}
	}
	return out
}

// amends are the amend blocks of obj in pkg's files of an active layer.
func (s *snapshot) amends(pkg *check.Package, obj check.Object) []located {
	info := s.a.Program().Info
	var out []located
	for _, f := range pkg.Files {
		if f.Layer == nil || !slices.Contains(s.layers, f.Layer.Name) {
			continue
		}
		for _, blk := range f.Amends {
			if info.NameUses[blk.Target] == obj {
				out = append(out, located{f, blk})
			}
		}
	}
	return out
}

// brokenNames are the broken declarations the nodes name, in source order.
func (s *snapshot) brokenNames(nodes []located) []check.Object {
	info := s.a.Program().Info
	var out []check.Object
	for _, l := range nodes {
		syntax.Inspect(l.n, func(n syntax.Node) bool {
			if o := info.ObjectOf(n); o != nil && info.Broken[o] && !slices.Contains(out, o) {
				out = append(out, o)
			}
			return true
		})
	}
	return out
}

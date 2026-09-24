package ir

import (
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// emitSite is an emit with its declaration and the spans findings point at.
type emitSite struct {
	e       *Emit
	decl    *syntax.EmitDecl
	file    *syntax.File
	outSpan source.Span
	display string // the output as a display path (WIRE.md §2.3)
}

func (es *emitSite) span() source.Span { return es.file.Span(es.decl.Target) }

// emits reads every emit of u and resolves its output (CODEGEN.md §2.1, §2.8, decision 108), the first of each target only: a second one is check's E8002.
func (s *stage) emits(u *unit) {
	layout, _ := project.NewLayout(s.in.Project, curDir, nil, nil)
	bag := u.bag
	if !u.selected || bag == nil {
		bag = diag.NewBag(nil, u.cp.Path)
	}
	seen := map[Target]bool{}
	for _, f := range sourceFiles(u.cp) {
		for _, d := range f.Decls {
			ed, ok := d.(*syntax.EmitDecl)
			if !ok || ed.Target == nil || ed.Options == nil {
				continue
			}
			t, known := wordIndex[Target](targetWords[:], ed.Target.Name)
			if !known || seen[t] {
				continue
			}
			seen[t] = true
			es := &emitSite{e: &Emit{Target: t}, decl: ed, file: f}
			s.readOptions(u, es)
			s.resolveOut(u, es, layout, bag)
			u.emits = append(u.emits, es)
			u.p.Emits = append(u.p.Emits, es.e)
		}
	}
}

// readOptions types the options check validated (E8003, E8009): out, mode, values, package, namespace; an absent `values` is nil, `values: []` an empty list (decision 127).
func (s *stage) readOptions(u *unit, es *emitSite) {
	e := es.e
	if e.Target == TargetGo || e.Target == TargetCpp || e.Target == TargetTS {
		e.Mode = ModeBaked
	}
	for _, it := range es.decl.Options.Items {
		fi, ok := it.(*syntax.FieldItem)
		if !ok || fi.Name == nil {
			continue
		}
		switch fi.Name.Name {
		case optOut:
			e.Out = constString(fi.Value)
			es.outSpan = es.file.Span(fi.Value)
		case optMode:
			if id, isWord := fi.Value.(*syntax.IdentExpr); isWord {
				e.Mode, _ = wordIndex[Mode](modeWords[:], id.Name)
			}
		case optValues:
			e.Values = valueNames(fi.Value)
		case optPackage:
			e.GoPackage = constString(fi.Value)
		case optNamespace:
			e.Namespace = constString(fi.Value)
		}
	}
	if e.Target == TargetCpp && e.Namespace == "" {
		e.Namespace = strings.ReplaceAll(u.cp.Path, qnameSep, cppScope)
	}
}

// constString is a string literal's text, or "" for an interpolation (E1132, E1119) or another kind.
func constString(x syntax.Node) string {
	switch v := x.(type) {
	case *syntax.RawStringLit:
		return v.Value
	case *syntax.StringLit:
		var b strings.Builder
		for _, p := range v.Parts {
			if p.Interp != nil {
				return ""
			}
			b.WriteString(p.Text)
		}
		return b.String()
	}
	return ""
}

func valueNames(x syntax.Expr) []string {
	list, ok := x.(*syntax.ListLit)
	if !ok {
		return nil
	}
	out := []string{}
	for _, el := range list.Elems {
		if id, isName := el.(*syntax.IdentExpr); isName {
			out = append(out, id.Name)
		}
	}
	return out
}

// resolveOut places the output through the declared roots: a ts or view emit, and a json emit in file mode, name a file in Dir (decision 127); a go emit gets its import path (E8007).
func (s *stage) resolveOut(u *unit, es *emitSite, layout *project.Layout, bag *diag.Bag) {
	e := es.e
	if e.Out == "" || layout == nil {
		return
	}
	p, ok := layout.Resolve(e.Out, u.p.Dir, es.outSpan, bag)
	if !ok {
		return
	}
	es.display = p.Display
	file := e.Target == TargetTS || e.Target == TargetView || e.Target == TargetJSON && strings.HasSuffix(e.Out, JSONExt)
	if file {
		e.Dir, e.FileName = path.Dir(p.Abs), path.Base(p.Abs)
		return
	}
	e.Dir = p.Abs
	if e.Target != TargetGo {
		return
	}
	if e.GoPackage == "" {
		e.GoPackage = path.Base(p.Abs)
	}
	imp, mapped := s.goImport(e.Dir)
	if !mapped && u.selected {
		u.report(diag.E8007.At(es.outSpan, p.Display))
	}
	e.GoImport = imp
}

// goImport is the import path of Go directory dir: the module of the mapped root that is dir or its closest ancestor, joined with the rest of dir (CODEGEN.md §2.8).
func (s *stage) goImport(dir string) (string, bool) {
	best, bestLen, found := "", -1, false
	for _, m := range s.in.Project.GoModules {
		root, ok := s.in.Project.Root(m.Root)
		if !ok {
			continue
		}
		rootDir := path.Clean(root.Path)
		rel, under := below(dir, rootDir)
		if !under || len(rootDir) <= bestLen {
			continue
		}
		best, bestLen, found = m.Module, len(rootDir), true
		if rel != "" {
			best = m.Module + pathSep + rel
		}
	}
	return best, found
}

// below is dir relative to root when root is dir or one of its ancestors, lexically.
func below(dir, root string) (string, bool) {
	switch {
	case dir == root:
		return "", true
	case root == curDir:
		return dir, dir != parentDir && !strings.HasPrefix(dir, parentDir+pathSep)
	}
	rel, ok := strings.CutPrefix(dir, root+pathSep)
	return rel, ok
}

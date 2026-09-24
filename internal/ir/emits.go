package ir

import (
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// emitSite is an emit with its declaration and the spans findings point at.
type emitSite struct {
	e        *Emit
	decl     *syntax.EmitDecl
	file     *syntax.File
	outSpan  source.Span
	display  string // the output as a display path (WIRE.md §2.3)
	unmapped bool   // a go emit whose resolved directory is under no go_module root (E8007)
	written  bool   // a go emit whose package is written, so check validated it (E8009); a default from out is not yet
}

func (es *emitSite) span() source.Span { return es.file.Span(es.decl.Target) }

// emits reads every emit of u and resolves its output (CODEGEN.md §2.1, §2.8, decision 108), the first of each target only: a second one is check's E8002.
func (s *stage) emits(u *unit) {
	bag := u.bag
	if !u.selected || bag == nil {
		bag = diag.NewBag(nil, u.cp.Path)
	}
	seen := map[Target]bool{}
	for _, f := range sourceFiles(u.cp) {
		for _, d := range f.Decls {
			if ed, ok := d.(*syntax.EmitDecl); ok && ed.Target != nil && ed.Options != nil {
				s.emit(u, f, ed, seen, bag)
			}
		}
	}
}

// emit reads one emit of u, unless its target is unknown (E8003) or already seen (E8002).
func (s *stage) emit(u *unit, f *syntax.File, ed *syntax.EmitDecl, seen map[Target]bool, bag *diag.Bag) {
	t, known := wordIndex[Target](targetWords[:], ed.Target.Name)
	if !known || seen[t] {
		return
	}
	seen[t] = true
	es := &emitSite{e: &Emit{Target: t}, decl: ed, file: f}
	s.readOptions(u, es)
	s.resolveOut(u, es, s.layout, bag)
	u.emits = append(u.emits, es)
	u.p.Emits = append(u.p.Emits, es.e)
}

// modeRefused reports a code emit whose mode check refused (E8009): its out, directory and package still count in every rule that does not depend on the mode, and no mode-dependent rule judges it (decision 213).
func modeRefused(e *Emit) bool {
	return isCode(e.Target) && e.Mode == ModeNone
}

// readOptions types the options check validated (E8003, E8009): out, mode (ModeNone when check refused it), values, package, namespace; an absent `values` and an explicit `values: []` both read as nil, so selectedNames expands either to every public value (decision 127).
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
		case check.OptOut:
			e.Out = constString(fi.Value)
			es.outSpan = es.file.Span(fi.Value)
		case check.OptMode:
			e.Mode = ModeNone
			if id, isWord := fi.Value.(*syntax.IdentExpr); isWord {
				e.Mode, _ = wordIndex[Mode](modeWords[:], id.Name)
			}
		case check.OptValues:
			e.Values = valueNames(fi.Value)
		case check.OptPackage:
			e.GoPackage = constString(fi.Value)
			es.written = e.GoPackage != ""
		case check.OptNamespace:
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

// valueNames is an explicit `values` list's names, nil for an empty list too (decision 127).
func valueNames(x syntax.Expr) []string {
	list, ok := x.(*syntax.ListLit)
	if !ok {
		return nil
	}
	var out []string
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
	es.unmapped = !mapped
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
		return dir, notAbove(dir)
	}
	rel, ok := strings.CutPrefix(dir, root+pathSep)
	return rel, ok && notAbove(rel)
}

// notAbove reports a relative path that does not walk back out of root through "..".
func notAbove(rel string) bool {
	return rel != parentDir && !strings.HasPrefix(rel, parentDir+pathSep)
}

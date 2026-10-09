package ir

import (
	"path"
	"slices"
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
	outSpan  source.Span // the out entry this copy writes, or the out option
	outs     []outText   // the out entries as written, one per copy (CODEGEN.md §2.1)
	index    int         // which entry of outs this site writes: 0 for the first
	display  string      // the output as a display path (WIRE.md §2.3)
	unmapped bool        // a go emit whose resolved directory is under no go_module root (E8007)
	named    bool        // a go emit that writes its package, so none is defaulted from out
	refused  bool        // a go emit whose package check refused (E8009, or E1132 for an interpolation), written, defaulted, or none for want of an out (decisions 213, 215)
	modeSpan source.Span // the mode option's value, or none without one
}

// outText is one entry of an emit's out: its text as written ("" for one check refused) and its span.
type outText struct {
	text string
	span source.Span
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

// emit reads one emit of u, unless its target is unknown (E8003) or already seen (E8002). A go emit whose package check refused, written, defaulted or missing with its out (E8009), is marked refused: its importers do not refuse the import name again (decisions 213, 215).
func (s *stage) emit(u *unit, f *syntax.File, ed *syntax.EmitDecl, seen map[Target]bool, bag *diag.Bag) {
	t, known := wordIndex[Target](targetWords[:], ed.Target.Name)
	if !known || seen[t] {
		return
	}
	seen[t] = true
	base := &emitSite{e: &Emit{Target: t}, decl: ed, file: f}
	s.readOptions(u, base)
	sites := copies(base)
	for _, es := range sites {
		s.resolveOut(u, es, s.layout, bag)
	}
	sharedGoPackage(sites)
	for _, es := range sites {
		es.refused = t == TargetGo && !goPackageName(es.e.GoPackage)
		u.emits = append(u.emits, es)
		u.p.Emits = append(u.p.Emits, es.e)
	}
}

// copies is one site per entry of base's out, in list order, each with its own Emit; base itself for one out or none (CODEGEN.md §2.1).
func copies(base *emitSite) []*emitSite {
	if len(base.outs) == 1 {
		base.e.Out, base.outSpan = base.outs[0].text, base.outs[0].span
	}
	if len(base.outs) < severalCopies {
		return []*emitSite{base}
	}
	sites := make([]*emitSite, len(base.outs))
	for i, o := range base.outs {
		e := *base.e
		e.Out, e.Values, e.Open = o.text, slices.Clone(base.e.Values), slices.Clone(base.e.Open)
		site := *base
		site.e, site.outSpan, site.index = &e, o.span, i
		sites[i] = &site
	}
	return sites
}

// sharedGoPackage refuses the defaulted package of go copies ending in different names, check's E8009 `outPackage` (DECISIONS 213, 269).
func sharedGoPackage(sites []*emitSite) {
	first := sites[0]
	if first.e.Target != TargetGo || first.named {
		return
	}
	name := ""
	for _, es := range sites {
		switch {
		case es.e.Dir == "":
		case name == "":
			name = es.e.GoPackage
		case es.e.GoPackage != name:
			for _, c := range sites {
				c.e.GoPackage = ""
			}
			return
		}
	}
}

// modeRefused reports a code emit whose mode check refused (E8009) or stage E refuses as not built yet (unbuiltMode): its out, directory and package still count in every rule that does not depend on the mode, and no mode-dependent rule judges it (decision 213).
func modeRefused(e *Emit) bool {
	return isCode(e.Target) && (e.Mode == ModeNone || unbuiltMode(e))
}

// unbuiltMode reports a mode its target's generator does not write yet (DECISIONS 320): go and cpp embedded (CODEGEN.md §2.1, §2.2).
func unbuiltMode(e *Emit) bool {
	return int(e.Target) < len(unbuiltAlt) && unbuiltAlt[e.Target][e.Mode] != ModeNone
}

// readOptions types the options check validated (E8003, E8009): out, mode (ModeNone when check refused it), values, package, namespace, a go emit's open (only a go emit has it: E8003 elsewhere); an absent `values` and an explicit `values: []` both read as nil, so selectedNames expands either to every public value (decision 127).
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
			es.outSpan, es.outs = es.file.Span(fi.Value), outTexts(es.file, fi.Value, e.Target)
		case check.OptMode:
			e.Mode, es.modeSpan = ModeNone, es.file.Span(fi.Value)
			if id, isWord := fi.Value.(*syntax.IdentExpr); isWord {
				e.Mode, _ = wordIndex[Mode](modeWords[:], id.Name)
			}
		case check.OptValues:
			e.Values = valueNames(fi.Value)
		case check.OptOpen:
			e.Open = valueNames(fi.Value)
		case check.OptPackage:
			e.GoPackage = constString(fi.Value)
			es.named = true
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

// outTexts are the entries of out: its one string, or each element of a list but for view (CODEGEN.md §2.1).
func outTexts(f *syntax.File, x syntax.Expr, t Target) []outText {
	list, ok := x.(*syntax.ListLit)
	if !ok || t == TargetView {
		return []outText{{text: constString(x), span: f.Span(x)}}
	}
	out := make([]outText, len(list.Elems))
	for i, el := range list.Elems {
		out[i] = outText{text: constString(el), span: f.Span(el)}
	}
	return out
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

// goPackageName reports a name check accepts as a Go package: an identifier that is no Go keyword (CODEGEN.md §2.1, E8009).
func goPackageName(name string) bool {
	return identPattern.MatchString(name) && !check.IsGoKeyword(name)
}

// fileDir is the directory an unrooted path written in f starts at (WIRE.md §2.2).
func fileDir(f *syntax.File) string {
	if f.Src == nil {
		return ""
	}
	if d := path.Dir(f.Src.Path); d != curDir {
		return d
	}
	return ""
}

// resolveOut places the output through the declared roots, an unrooted out from the directory of the file holding the emit (WIRE.md §2.2): a ts or view emit, and a json emit in file mode, name a file in Dir (decision 127); a go emit gets its import path (E8007) and, unless it writes one, its package (goDefault).
func (s *stage) resolveOut(u *unit, es *emitSite, layout *project.Layout, bag *diag.Bag) {
	e := es.e
	if e.Out == "" || layout == nil {
		return
	}
	e.From = fileDir(es.file) // WIRE.md §2.2: an unrooted out starts at its own file's directory
	p, ok := layout.Resolve(e.Out, e.From, es.outSpan, bag)
	if !ok {
		return
	}
	es.display = p.Display
	file := e.Target == TargetTS || e.Target == TargetView || e.Target == TargetJSON && check.JSONFile(e.Out)
	if file {
		e.Dir, e.FileName = path.Dir(p.Abs), path.Base(p.Abs)
		return
	}
	e.Dir = p.Abs
	if e.Target != TargetGo {
		return
	}
	if !es.named {
		goDefault(es, p.Abs, layout.Dir)
	}
	imp, mapped := s.goImport(e.Dir)
	es.unmapped = !mapped
	if !mapped && u.selected {
		u.report(diag.E8007.At(es.outSpan, p.Display))
	}
	e.GoImport = imp
}

// goDefault is a go emit's package when it writes none: the last element of its directory as declared, none for the project directory itself; emit validates it as check does (CODEGEN.md §2.1, decisions 213, 215).
func goDefault(es *emitSite, dir, projectDir string) {
	if dir != projectDir {
		es.e.GoPackage = path.Base(dir)
	}
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
		rel, under := check.Within(dir, rootDir)
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

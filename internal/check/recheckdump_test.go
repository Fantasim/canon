package check_test

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// canonical prints all a program and its bags hold, nodes by file path and position, objects
// and types by name, never by pointer: two runs over the same sources print the same text.
func canonical(fs *source.FileSet, prog *check.Program, bags check.Bags) string {
	var b strings.Builder
	for _, p := range prog.Packages {
		fmt.Fprintf(&b, "package %s\n", p.Path)
		for _, imp := range p.Imports {
			fmt.Fprintf(&b, "  import %s\n", imp.Path)
		}
		for _, name := range slices.Sorted(maps.Keys(p.Layers)) {
			fmt.Fprintf(&b, "  layer %s %d\n", name, len(p.Layers[name]))
		}
		for _, f := range p.Files {
			fmt.Fprintf(&b, "  file %s\n", f.Src.Path)
		}
		for _, o := range p.Decls {
			fmt.Fprintf(&b, "  decl %s at %s\n", objText(o), where(o))
		}
	}
	for _, p := range prog.Packages {
		for _, f := range p.Files {
			facts(&b, f, prog.Info)
		}
	}
	broken(&b, prog.Info)
	sizes(&b, prog.Info)
	b.WriteString(findingsJSON(fs, bags))
	return b.String()
}

// where is the file path, line and column of an object's declaration.
func where(o check.Object) string {
	if o.File() == nil || o.Decl() == nil {
		return "-"
	}
	line, col := o.File().Src.Position(o.File().Span(o.Decl()).Start)
	return fmt.Sprintf("%s:%d:%d", o.File().Src.Path, line, col)
}

// facts prints each fact Info records about a node of f, in source order.
func facts(b *strings.Builder, f *syntax.File, info *check.Info) {
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n == nil {
			return true
		}
		sp := f.Span(n)
		line, col := f.Src.Position(sp.Start)
		at := fmt.Sprintf("%s:%d:%d+%d %s", f.Src.Path, line, col, sp.Len(), n.Kind())
		for _, fact := range nodeFacts(n, info) {
			fmt.Fprintf(b, "%s %s\n", at, fact)
		}
		return true
	})
}

// nodeFacts are the facts about n, one string each.
func nodeFacts(n syntax.Node, info *check.Info) []string {
	var out []string
	add := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }
	if e, ok := n.(syntax.Expr); ok {
		if t, ok := info.Types[e]; ok {
			add("type %s", t)
		}
		if c, ok := info.Conv[e]; ok {
			add("conv %s", convText(c))
		}
		if k, ok := info.Keys[e]; ok {
			add("key %s", k)
		}
	}
	if t, ok := n.(syntax.Type); ok {
		if tt, ok := info.TypeExprs[t]; ok {
			add("typeexpr %s", tt)
		}
	}
	if m, ok := info.Matches[n]; ok {
		add("match %s %v %v %v", m.Scrutinee, m.Covers, m.Exhaustive, m.Unreachable)
	}
	return append(out, kindFacts(n, info)...)
}

// kindFacts are the facts of the maps keyed by one node kind.
func kindFacts(n syntax.Node, info *check.Info) []string {
	var out []string
	add := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }
	switch x := n.(type) {
	case *syntax.Ident:
		if o, ok := info.Defs[x]; ok {
			add("def %s", objText(o))
		}
		if o, ok := info.NameUses[x]; ok {
			add("names %s at %s", objText(o), where(o))
		}
	case *syntax.IdentExpr:
		if o, ok := info.Uses[x]; ok {
			add("uses %s at %s", objText(o), where(o))
		}
		if s, ok := info.Symbols[x]; ok {
			add("symbol %v", s)
		}
	case *syntax.SelectorExpr:
		if s, ok := info.Selections[x]; ok {
			add("select %s %s at %s recv %s deref=%v", s.Kind, objText(s.Obj), whereOrNone(s.Obj), s.Recv, s.Deref)
		}
	case *syntax.CallExpr:
		if c, ok := info.Calls[x]; ok {
			add("call %s %s %s#%d %s", c.Kind, objText(c.Obj), c.Builtin, c.Overload, typeList(c.TypeArgs))
		}
	case *syntax.BraceLit:
		if k, ok := info.Literals[x]; ok {
			add("literal %s", k)
		}
	case *syntax.ViewDecl:
		add("view broken=%v", info.BrokenViews[x])
	case *syntax.TranslationEntry:
		add("translation broken=%v", info.BrokenTranslations[x])
	}
	return out
}

func whereOrNone(o check.Object) string {
	if o == nil {
		return "-"
	}
	return where(o)
}

// broken prints the broken objects, sorted.
func broken(b *strings.Builder, info *check.Info) {
	var lines []string
	for o, isBroken := range info.Broken {
		lines = append(lines, fmt.Sprintf("broken %s at %s %v\n", objText(o), whereOrNone(o), isBroken))
	}
	slices.Sort(lines)
	b.WriteString(strings.Join(lines, ""))
}

// sizes prints the size of every map of Info: a fact about a node of no checked file shows.
func sizes(b *strings.Builder, i *check.Info) {
	fmt.Fprintf(b, "sizes %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d\n", len(i.Types), len(i.TypeExprs), len(i.Defs),
		len(i.Uses), len(i.NameUses), len(i.Selections), len(i.Conv), len(i.Keys), len(i.Symbols), len(i.Calls),
		len(i.Literals), len(i.Matches), len(i.Broken), len(i.BrokenViews), len(i.BrokenTranslations))
}

// findingsJSON renders the findings of every bag as JSON, with their summary.
func findingsJSON(fs *source.FileSet, bags check.Bags) string {
	var all []diag.Finding
	var sum diag.Summary
	for _, name := range slices.Sorted(maps.Keys(bags)) {
		all = append(all, bags[name].Findings()...)
		sum = sum.Merge(bags[name].Summary())
	}
	var buf bytes.Buffer
	if err := diag.Render(&buf, fs, all, diag.RenderOptions{Format: diag.FormatJSON, Summary: sum}); err != nil {
		return err.Error()
	}
	return buf.String()
}

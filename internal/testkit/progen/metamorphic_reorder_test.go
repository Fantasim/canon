package progen_test

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// block is a declaration or project entry to move: its name and its whole lines, doc block and
// prefix annotations included.
type block struct {
	name       string
	start, end int
}

// reorderSites swap two whole-line blocks where order is no output: two `local` declarations of a
// source file that do not name each other, or two entries of one map of project.canon.
func reorderSites(tg target) []metaSite {
	switch {
	case isProject(tg) && tg.file != nil && tg.file.Project != nil:
		var out []metaSite
		for _, entry := range tg.file.Project.Items {
			out = append(out, swapSites(tg, projectBlocks(tg, text(tg, entry.Key)))...)
		}
		return out
	case isSource(tg):
		return swapSites(tg, localBlocks(tg))
	}
	return nil
}

// localBlocks are tg's `local` declarations as blocks.
func localBlocks(tg target) []block {
	var out []block
	for _, ld := range localDecls(tg) {
		s, e := span(tg, ld.decl)
		if ld.doc != nil {
			s = min(s, int(ld.doc.Start))
		}
		out = append(out, wholeLines(tg, ld.name.Name, s, e))
	}
	return out
}

// projectBlocks are the entries of the project map named key as blocks; none when key names no
// map.
func projectBlocks(tg target, key string) []block {
	if tg.file == nil || tg.file.Project == nil {
		return nil
	}
	i := slices.IndexFunc(tg.file.Project.Items, func(e *syntax.ProjectEntry) bool { return text(tg, e.Key) == key })
	if i < 0 {
		return nil
	}
	pm, ok := tg.file.Project.Items[i].Value.(*syntax.ProjectMap)
	if !ok {
		return nil
	}
	out := make([]block, 0, len(pm.Entries))
	for _, e := range pm.Entries {
		s, end := span(tg, e)
		if e.Doc != nil {
			s = min(s, int(e.Doc.Start))
		}
		out = append(out, wholeLines(tg, text(tg, e.Key), s, end))
	}
	return out
}

// wholeLines widens [s, e) to its whole lines, the final line break included.
func wholeLines(tg target, name string, s, e int) block {
	end := lineEnd(tg, e)
	if end < len(tg.src) {
		end++
	}
	return block{name: name, start: lineStart(tg, s), end: end}
}

// swapSites is a site per pair of bs that own their lines and name each other nowhere.
func swapSites(tg target, bs []block) []metaSite {
	var out []metaSite
	for i, a := range bs {
		for _, b := range bs[i+1:] {
			if a.end > b.start || b.end > len(tg.src) || crossReferences(tg, a, b) {
				continue
			}
			edits := []progen.Edit{replace(a.start, a.end, blockText(tg, b)), replace(b.start, b.end, blockText(tg, a))}
			out = append(out, metaSite{edits: edits, desc: "swap " + a.name + " and " + b.name})
		}
	}
	return out
}

// crossReferences tells a's text naming b, or b's naming a (a substring: over-cautious).
func crossReferences(tg target, a, b block) bool {
	return strings.Contains(blockText(tg, a), b.name) || strings.Contains(blockText(tg, b), a.name)
}

func blockText(tg target, b block) string { return string(tg.src[b.start:b.end]) }

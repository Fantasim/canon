package progen_test

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// Operators on project.canon (GRAMMAR.md §7.1): each edits one key of the examples' project.
func projectOperators() []operator {
	return []operator{
		op(diag.E1001.Def().Code, "GRAMMAR.md §7.1 (unsupported version)", onKey("canon", replaceValue(`"0.9"`))),
		op(diag.E1002.Def().Code, "GRAMMAR.md §7.1 (unknown key)", onKey("canon", insertEntryBefore("colour", `: "blue"`))),
		op(diag.E1004.Def().Code, "GRAMMAR.md §7.1 (canon missing)", onKey("canon", deleteEntryMarkName)),
		op(diag.E1005.Def().Code, "GRAMMAR.md §7.1 (duplicate key)", onKey("languages", duplicateEntry)),
		op(diag.E1006.Def().Code, "GRAMMAR.md §7.1 (budget below 1)", onKey("budget", replaceValue("0"))),
		op(diag.E1007.Def().Code, "GRAMMAR.md §7.1 (empty root path)", onMapEntries("roots", replaceValue(`""`))),
		op(diag.E1008.Def().Code, "GRAMMAR.md §7.1 (duplicate language)", onKey("languages", appendListItem("en"))),
		op(diag.E1009.Def().Code, "GRAMMAR.md §7.1 (go_module key not a root)", onMapEntries("go_module", insertEntryBefore("nowhere", `: "example.com/x"`))),
		op(diag.E1010.Def().Code, "GRAMMAR.md §7.1 (not MAJOR.MINOR)", onKey("canon", replaceValue(`"0.1.0"`))),
		op(diag.E1011.Def().Code, "GRAMMAR.md §5.2 (anything else in project.canon)", projectTrailer),
		op(diag.E1012.Def().Code, "GRAMMAR.md §7.1 (studio package missing)", onKey("studio", replaceValue("nowhere"))),
	}
}

type entryEdit func(tg target, e *syntax.ProjectEntry) progen.Site

// onKey applies f to the top-level project entry named key.
func onKey(key string, f entryEdit) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		if !isProject(tg) || tg.file.Project == nil {
			return nil
		}
		var out []progen.Site
		for _, e := range tg.file.Project.Items {
			if text(tg, e.Key) == key {
				out = append(out, f(tg, e))
			}
		}
		return out
	}
}

// onMapEntries applies f to each entry of the map-valued project entry named key.
func onMapEntries(key string, f entryEdit) func(target) []progen.Site {
	return onKeyMany(key, func(tg target, e *syntax.ProjectEntry) []progen.Site {
		m, ok := e.Value.(*syntax.ProjectMap)
		if !ok {
			return nil
		}
		var out []progen.Site
		for _, sub := range m.Entries {
			out = append(out, f(tg, sub))
		}
		return out
	})
}

func onKeyMany(key string, f func(target, *syntax.ProjectEntry) []progen.Site) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		if !isProject(tg) || tg.file.Project == nil {
			return nil
		}
		var out []progen.Site
		for _, e := range tg.file.Project.Items {
			if text(tg, e.Key) == key {
				out = append(out, f(tg, e)...)
			}
		}
		return out
	}
}

func replaceValue(v string) entryEdit {
	return func(tg target, e *syntax.ProjectEntry) progen.Site {
		s, end := span(tg, e.Value)
		return site(replace(s, end, v))
	}
}

// insertEntryBefore adds "key rest" on its own line before e; the finding is at the key.
func insertEntryBefore(key, rest string) entryEdit {
	return func(tg target, e *syntax.ProjectEntry) progen.Site {
		s, _ := span(tg, e.Key)
		return seq(0, insert(s, key), insert(s, rest+"\n"+indent(tg, s)))
	}
}

// deleteEntryMarkName removes e's line; the finding is at the project's name.
func deleteEntryMarkName(tg target, e *syntax.ProjectEntry) progen.Site {
	s, _ := span(tg, e)
	ns, ne := span(tg, tg.file.Project.Name)
	return site(mark(tg, ns, ne), replace(lineStart(tg, s), lineEnd(tg, s)+1, ""))
}

// duplicateEntry repeats e's line after it; the finding is at the copy's key.
func duplicateEntry(tg target, e *syntax.ProjectEntry) progen.Site {
	s, end := span(tg, e)
	ks, ke := span(tg, e.Key)
	at := lineEnd(tg, end) + 1
	return keeping(tg, seq(1, insert(at, indent(tg, s)), insert(at, string(tg.src[ks:ke])), insert(at, string(tg.src[ke:end])+"\n")), e.Key)
}

// appendListItem adds item to the list value of e, keeping the item it repeats; the finding is
// at the new item.
func appendListItem(item string) entryEdit {
	return func(tg target, e *syntax.ProjectEntry) progen.Site {
		_, end := span(tg, e.Value)
		var same []syntax.Node
		if l, ok := e.Value.(*syntax.ProjectList); ok {
			for _, it := range l.Items {
				if text(tg, it) == item {
					same = append(same, it)
				}
			}
		}
		return keeping(tg, seq(1, insert(end-1, ", "), insert(end-1, item)), same...)
	}
}

// projectTrailer declares something after the project declaration.
func projectTrailer(tg target) []progen.Site {
	if !isProject(tg) {
		return nil
	}
	return []progen.Site{appendDecl(tg, "", "let", " y = 1")}
}

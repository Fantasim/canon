package project

import (
	"cmp"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// roots is a map of root names to paths (GRAMMAR.md §7.1: E1005, E1006, E1007).
func (s *schema) roots(e *syntax.ProjectEntry) {
	m, ok := e.Value.(*syntax.ProjectMap)
	if !ok {
		s.fail(diag.E1006.AtRoots(s.span(e.Value)))
		return
	}
	for _, r := range m.Entries {
		if root, ok := s.root(r); ok {
			s.p.Roots = append(s.p.Roots, root)
		}
	}
	slices.SortFunc(s.p.Roots, func(a, b Root) int { return cmp.Compare(a.Name, b.Name) })
}

func (s *schema) root(e *syntax.ProjectEntry) (Root, bool) {
	name, ident := s.keyName(e.Key)
	span := s.span(e.Key)
	switch {
	case isBad(e.Key):
		return Root{}, false
	case !ident:
		s.fail(diag.E1007.AtName(span, name))
		return Root{}, false
	case s.named[name]:
		s.fail(diag.E1005.AtRoot(span, name))
		return Root{}, false
	}
	s.named[name] = true
	if isBad(e.Value) {
		return Root{}, false
	}
	path, ok := stringOf(e.Value)
	if !ok {
		s.fail(diag.E1006.AtRoots(s.span(e.Value)))
		return Root{}, false
	}
	if b := rootPathError(s.span(e.Value), name, path); b != nil {
		s.fail(b)
		return Root{}, false
	}
	return Root{Name: name, Path: path, Span: span}, true
}

// rootPathError is E1007 for a root path that is empty, absolute or holds a backslash.
func rootPathError(span source.Span, name, path string) *diag.Builder {
	switch {
	case path == "":
		return diag.E1007.AtEmpty(span, name)
	case isAbsolute(path):
		return diag.E1007.AtAbsolute(span, name)
	case strings.Contains(path, backslash):
		return diag.E1007.AtBackslash(span, name)
	}
	return nil
}

// isAbsolute reports a path absolute on some platform: "/x", "\x", "C:x" (WIRE.md §2.1).
func isAbsolute(path string) bool {
	return strings.HasPrefix(path, sep) || strings.HasPrefix(path, backslash) || drivePattern.MatchString(path)
}

// optionalRoots is a list of declared root names, each once (GRAMMAR.md §7.1: E1006, E1009).
func (s *schema) optionalRoots(e *syntax.ProjectEntry) {
	l, ok := e.Value.(*syntax.ProjectList)
	if !ok {
		s.fail(diag.E1006.AtOptionalRoots(s.span(e.Value)))
		return
	}
	listed := map[string]bool{}
	for _, item := range l.Items {
		q, ok := item.(*syntax.QualifiedName)
		span := s.span(item)
		switch {
		case isBad(item):
		case !ok || len(q.Parts) != 1:
			s.fail(diag.E1006.AtOptionalRoots(span))
		case !s.named[qualified(q)]:
			s.fail(diag.E1009.AtOptionalRoot(span, qualified(q)))
		case listed[qualified(q)]:
			s.fail(diag.E1009.AtOptionalDuplicate(span, qualified(q)))
		default:
			listed[qualified(q)] = true
		}
	}
	for i := range s.p.Roots {
		s.p.Roots[i].Optional = listed[s.p.Roots[i].Name]
	}
}

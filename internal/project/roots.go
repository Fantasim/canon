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

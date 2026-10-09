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

// rootList is how a list of root names is refused: a value that is not one (E1006), a name not
// declared or listed twice (E1009); optional_roots and consumer_roots alike.
type rootList struct {
	kind      func(source.Span) *diag.Builder
	undefined func(source.Span, string) *diag.Builder
	twice     func(source.Span, string) *diag.Builder
}

// optionalList is optional_roots (DECISIONS 332); consumerList is consumer_roots (DECISIONS 343).
var (
	optionalList = rootList{kind: diag.E1006.AtOptionalRoots, undefined: diag.E1009.AtOptionalRoot, twice: diag.E1009.AtOptionalDuplicate}
	consumerList = rootList{kind: diag.E1006.AtConsumerRoots, undefined: diag.E1009.AtConsumerRoot, twice: diag.E1009.AtConsumerDuplicate}
)

// optionalRoots is a list of declared root names, each once (GRAMMAR.md §7.1: E1006, E1009).
func (s *schema) optionalRoots(e *syntax.ProjectEntry) {
	listed := s.rootNames(e, optionalList)
	for i := range s.p.Roots {
		s.p.Roots[i].Optional = listed[s.p.Roots[i].Name]
	}
}

// consumerRoots is a list of declared root names, each once: the roots the project's consumers
// build, each optional too (DECISIONS 343: E1006, E1009).
func (s *schema) consumerRoots(e *syntax.ProjectEntry) {
	listed := s.rootNames(e, consumerList)
	for i := range s.p.Roots {
		s.p.Roots[i].Consumer = listed[s.p.Roots[i].Name]
	}
}

// rootNames is the set of declared root names the list e holds, refused as rl says.
func (s *schema) rootNames(e *syntax.ProjectEntry, rl rootList) map[string]bool {
	listed := map[string]bool{}
	l, ok := e.Value.(*syntax.ProjectList)
	if !ok {
		s.fail(rl.kind(s.span(e.Value)))
		return listed
	}
	for _, item := range l.Items {
		q, ok := item.(*syntax.QualifiedName)
		span := s.span(item)
		switch {
		case isBad(item):
		case !ok || len(q.Parts) != 1:
			s.fail(rl.kind(span))
		case !s.named[qualified(q)]:
			s.fail(rl.undefined(span, qualified(q)))
		case listed[qualified(q)]:
			s.fail(rl.twice(span, qualified(q)))
		default:
			listed[qualified(q)] = true
		}
	}
	return listed
}

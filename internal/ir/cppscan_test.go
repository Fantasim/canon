package ir_test

import (
	"regexp"
	"strings"
)

// cppFrame is one open brace of generated C++: a scope that declares names (a namespace, a class, an enum) or a block, whose contents declare none.
type cppFrame struct {
	scope string // "" for a block
	enum  bool
}

// cppScan collects the names gen/cpp's files declare, by scope: a namespace by its plan name (the emit's namespace, detail, conformance), a class, struct or enum by its name; an out-of-line `C::m` goes to C. Function bodies and initializers are blocks.
type cppScan struct {
	ns     string
	names  map[string]map[string]bool
	frames []cppFrame
}

var (
	cppNamespaceLine = regexp.MustCompile(`^namespace ([\w:]*) ?\{$`)
	cppTypeLine      = regexp.MustCompile(`^(?:class|struct|enum class) (\w+)`)
	cppIdent         = regexp.MustCompile(`[\w:]+$`)
	cppStringLit     = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
)

func newCppScan(ns string) *cppScan { return &cppScan{ns: ns, names: map[string]map[string]bool{}} }

func (s *cppScan) add(scope, name string) {
	if s.names[scope] == nil {
		s.names[scope] = map[string]bool{}
	}
	s.names[scope][name] = true
}

// file scans one generated file.
func (s *cppScan) file(text string) {
	s.frames = nil
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(cppStringLit.ReplaceAllString(raw, `""`))
		if i := strings.Index(line, "//"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		s.line(line)
	}
}

// top is the innermost frame, nil at file level.
func (s *cppScan) top() *cppFrame {
	if len(s.frames) == 0 {
		return nil
	}
	return &s.frames[len(s.frames)-1]
}

// line declares what a line declares in the current scope, then opens and closes its braces.
func (s *cppScan) line(line string) {
	opened := ""
	if m := cppNamespaceLine.FindStringSubmatch(line); m != nil {
		opened = s.namespace(m[1])
	} else if top := s.top(); top != nil && top.scope != "" {
		opened = s.decl(top, line)
	}
	first := true
	for _, c := range line {
		switch c {
		case '{':
			f := cppFrame{}
			if first {
				f.scope, f.enum = opened, strings.HasPrefix(line, "enum class")
			}
			first = false
			s.frames = append(s.frames, f)
		case '}':
			s.frames = s.frames[:len(s.frames)-1]
		}
	}
}

// namespace is the plan's name of a namespace: the emit's, its detail or conformance one; an anonymous one is its parent.
func (s *cppScan) namespace(name string) string {
	switch {
	case name == "":
		return s.top().scope
	case name == s.ns:
		return s.ns
	}
	return name[strings.LastIndex(name, ":")+1:]
}

// decl declares one line's name in scope top, returning the scope its brace opens: a type's own.
func (s *cppScan) decl(top *cppFrame, line string) string {
	switch {
	case strings.HasPrefix(line, "}"):
		return ""
	case top.enum:
		s.add(top.scope, strings.TrimSuffix(strings.Fields(line)[0], ","))
		return ""
	case strings.HasSuffix(line, ":") || strings.HasPrefix(line, "friend ") || strings.Contains(line, "operator="):
		return ""
	}
	if m := cppTypeLine.FindStringSubmatch(line); m != nil {
		s.add(top.scope, m[1])
		return m[1]
	}
	name := line
	if i := strings.IndexAny(line, "(=;{["); i >= 0 && line[i] == '(' {
		name = line[:i]
	} else if i >= 0 {
		name = strings.TrimSpace(line[:i])
	}
	name = cppIdent.FindString(name)
	scope := top.scope
	if i := strings.LastIndex(name, "::"); i >= 0 {
		scope, name = name[:i], name[i+2:]
	}
	if name != "" && name != scope {
		s.add(scope, name)
	}
	return ""
}

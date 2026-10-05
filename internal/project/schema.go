package project

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Load reads project.canon into a Project and its findings into bag. After an error finding the
// project is nil, and err is ErrUnsupportedVersion for an E1001, ErrInvalid otherwise.
func Load(src *source.File, bag *diag.Bag) (*Project, error) {
	before := bag.Summary().Errors
	f := syntax.Parse(src, syntax.FileProject, bag)
	if f.Project == nil {
		return nil, ErrInvalid
	}
	d := f.Project
	s := &schema{f: f, bag: bag, p: New(d.Name.Name, Version{}), seen: map[string]bool{}, named: map[string]bool{}}
	if d.Doc != nil {
		s.p.Doc = d.Doc.Text
	}
	for _, e := range d.Items { // GRAMMAR.md §7.1
		s.entry(e)
	}
	if s.goModule != nil {
		s.goModules(s.goModule)
	}
	if !s.seen[keyCanon] {
		s.fail(diag.E1004.At(f.Span(d.Name)))
	}
	switch {
	case s.unsupported:
		return nil, ErrUnsupportedVersion
	case s.failed || bag.Summary().Errors > before:
		return nil, ErrInvalid
	}
	return s.p, nil
}

// schema reads the entries of one project declaration into p.
type schema struct {
	f           *syntax.File
	bag         *diag.Bag
	p           *Project
	seen        map[string]bool
	named       map[string]bool // root names declared, their path refused or not
	failed      bool
	unsupported bool
	goModule    *syntax.ProjectEntry
}

func (s *schema) fail(b *diag.Builder) {
	b.Report(s.bag)
	s.failed = true
}

func (s *schema) span(n syntax.Node) source.Span { return s.f.Span(n) }

// entry dispatches one top-level key: unknown is E1002, twice is E1005 (GRAMMAR.md §7.1).
func (s *schema) entry(e *syntax.ProjectEntry) {
	name, _ := s.keyName(e.Key)
	rule, known := keyRules[name]
	if isBad(e.Key) {
		return
	}
	switch {
	case !known:
		s.fail(diag.E1002.At(s.span(e.Key), name))
	case s.seen[name]:
		s.fail(diag.E1005.AtKey(s.span(e.Key), name))
	default:
		s.seen[name] = true
		if !isBad(e.Value) {
			rule(s, e)
		}
	}
}

// canon is `canon: "MAJOR.MINOR"`: E1006, E1010, then E1001 for a version not read here.
func (s *schema) canon(e *syntax.ProjectEntry) {
	span := s.span(e.Value)
	text, ok := stringOf(e.Value)
	if !ok {
		s.fail(diag.E1006.AtCanon(span))
		return
	}
	m := versionPattern.FindStringSubmatch(text)
	if m == nil {
		s.fail(diag.E1010.At(span, text))
		return
	}
	major, errMajor := strconv.Atoi(m[majorGroup])
	minor, errMinor := strconv.Atoi(m[minorGroup])
	v := Version{Major: major, Minor: minor}
	if errMajor != nil || errMinor != nil || !v.Supported() {
		s.unsupported = true
		s.fail(diag.E1001.At(span, text, versionTexts()))
		return
	}
	s.p.Canon = v
}

// versionTexts are the supported versions as E1001 lists them.
func versionTexts() []string {
	var out []string
	for _, v := range SupportedVersions() {
		out = append(out, v.String())
	}
	return out
}

// languages is a non-empty list of language codes, each once (E1006, E1008).
func (s *schema) languages(e *syntax.ProjectEntry) {
	l, ok := e.Value.(*syntax.ProjectList)
	if !ok || len(l.Items) == 0 {
		s.fail(diag.E1006.AtLanguages(s.span(e.Value)))
		return
	}
	var codes []string
	for _, item := range l.Items {
		q, ok := item.(*syntax.QualifiedName)
		code := qualified(q)
		switch {
		case isBad(item):
		case !ok:
			s.fail(diag.E1006.AtLanguages(s.span(item)))
		case !languagePattern.MatchString(code):
			s.fail(diag.E1008.AtInvalid(s.span(item), code))
		case slices.Contains(codes, code):
			s.fail(diag.E1008.AtDuplicate(s.span(item), code))
		default:
			codes = append(codes, code)
		}
	}
	s.p.Languages = codes
}

// studio names a package; that it exists is checked once packages are known (CheckStudio).
func (s *schema) studio(e *syntax.ProjectEntry) {
	q, ok := e.Value.(*syntax.QualifiedName)
	if !ok {
		s.fail(diag.E1006.AtStudio(s.span(e.Value)))
		return
	}
	s.p.Studio = Package{Path: qualified(q), Span: s.span(q)}
}

// budget is an integer of at least 1 that fits the step counter (E1006).
func (s *schema) budget(e *syntax.ProjectEntry) {
	n, ok := e.Value.(*syntax.IntLit)
	if !ok || n.Value.Sign() <= 0 || !n.Value.IsInt64() {
		s.fail(diag.E1006.AtBudget(s.span(e.Value)))
		return
	}
	s.p.Budget = n.Value.Int64()
}

func (s *schema) deferGoModule(e *syntax.ProjectEntry) { s.goModule = e }

// goModules maps declared roots to Go module paths (GRAMMAR.md §7.1, E1005, E1006, E1009).
func (s *schema) goModules(e *syntax.ProjectEntry) {
	m, ok := e.Value.(*syntax.ProjectMap)
	if !ok {
		s.fail(diag.E1006.AtGoModule(s.span(e.Value)))
		return
	}
	for _, g := range m.Entries {
		if mod, ok := s.goModuleEntry(g); ok {
			s.p.GoModules = append(s.p.GoModules, mod)
		}
	}
	slices.SortFunc(s.p.GoModules, func(a, b GoModule) int { return cmp.Compare(a.Root, b.Root) })
}

func (s *schema) goModuleEntry(g *syntax.ProjectEntry) (GoModule, bool) {
	name, ident := s.keyName(g.Key)
	span := s.span(g.Key)
	declared := s.named[name]
	switch {
	case isBad(g.Key):
	case !ident || !declared:
		s.fail(diag.E1009.AtRoot(span, name))
	case slices.ContainsFunc(s.p.GoModules, func(m GoModule) bool { return m.Root == name }):
		s.fail(diag.E1005.AtKey(span, name))
	case isBad(g.Value):
	default:
		return s.modulePath(g, name)
	}
	return GoModule{}, false
}

func (s *schema) modulePath(g *syntax.ProjectEntry, root string) (GoModule, bool) {
	path, ok := stringOf(g.Value)
	switch {
	case !ok:
		s.fail(diag.E1006.AtGoModule(s.span(g.Value)))
	case path == "" || strings.ContainsFunc(path, unicode.IsSpace):
		s.fail(diag.E1009.AtPath(s.span(g.Value), root))
	default:
		return GoModule{Root: root, Module: path, Span: s.span(g.Key)}, true
	}
	return GoModule{}, false
}

// keyName is a key's text, and whether it is written as an identifier (not a reserved word,
// not a string).
func (s *schema) keyName(n syntax.NameLit) (string, bool) {
	if id, ok := n.(*syntax.Ident); ok {
		return id.Name, s.f.Tokens[id.From].Kind == syntax.TokIdent
	}
	text, _ := stringOf(n)
	return text, false
}

// stringOf is the text of a constant string literal.
func stringOf(n syntax.Node) (string, bool) {
	switch v := n.(type) {
	case *syntax.StringLit:
		var b strings.Builder
		for _, part := range v.Parts {
			if part.Interp != nil {
				return "", false
			}
			b.WriteString(part.Text)
		}
		return b.String(), true
	case *syntax.RawStringLit:
		return v.Value, true
	}
	return "", false
}

// qualified is a dotted name's text; "" for none.
func qualified(q *syntax.QualifiedName) string {
	if q == nil {
		return ""
	}
	parts := make([]string, len(q.Parts))
	for i, id := range q.Parts {
		parts[i] = id.Name
	}
	return strings.Join(parts, nameSep)
}

// isBad reports a value or key the parser already refused: unreadable, or a string with an
// interpolation (E1132), so it gets no second finding (DECISIONS 141).
func isBad(n syntax.Node) bool {
	switch v := n.(type) {
	case *syntax.BadExpr:
		return true
	case *syntax.StringLit:
		return slices.ContainsFunc(v.Parts, func(p syntax.StringPart) bool { return p.Interp != nil })
	}
	return n == nil
}

package project

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Template is the path template root Root gives the emits of target Target (GRAMMAR.md §7.1).
type Template struct {
	Target string
	Root   string
	Span   source.Span // the root key's
	parts  []templatePart
}

// partKind is what a templatePart holds: literal text, or a placeholder.
type partKind uint8

// templateKey is a template's place in Project.Templates.
type templateKey struct{ target, root string }

type templatePart struct {
	kind partKind
	text string
}

// Expand is @root/ and the template of root for target with pkg put in (CODEGEN.md §2.1); false for none.
func (p *Project) Expand(target, root, pkg string) (string, bool) {
	i, found := slices.BinarySearchFunc(p.Templates, templateKey{target, root}, func(t Template, k templateKey) int {
		return cmp.Or(cmp.Compare(t.Target, k.target), cmp.Compare(t.Root, k.root))
	})
	if !found {
		return "", false
	}
	var b strings.Builder
	b.WriteString(rootMark + root + sep)
	for _, part := range p.Templates[i].parts {
		b.WriteString(expanders[part.kind](part.text, pkg))
	}
	return b.String(), true
}

// expanders write each kind of part for a package path.
var expanders = [...]func(text, pkg string) string{
	partText:   func(text, _ string) string { return text },
	partDir:    func(_, pkg string) string { return strings.ReplaceAll(pkg, nameSep, sep) },
	partDotted: func(_, pkg string) string { return pkg },
}

func (s *schema) deferTemplates(e *syntax.ProjectEntry) { s.templates = e }

// outTemplates is a map of emit targets to maps of roots to templates (E1019, E1009, E1005, E1006), kept sorted.
func (s *schema) outTemplates(e *syntax.ProjectEntry) {
	m, ok := e.Value.(*syntax.ProjectMap)
	if !ok {
		s.fail(diag.E1006.AtOutTemplates(s.span(e.Value)))
		return
	}
	targets := map[string]bool{}
	for _, g := range m.Entries {
		s.templateTarget(g, targets)
	}
	slices.SortFunc(s.p.Templates, func(a, b Template) int {
		return cmp.Or(cmp.Compare(a.Target, b.Target), cmp.Compare(a.Root, b.Root))
	})
}

// templateTarget reads one target: an emit target word (E1019 target) given once (E1005), holding a map.
func (s *schema) templateTarget(g *syntax.ProjectEntry, seen map[string]bool) {
	name, _ := s.keyName(g.Key)
	_, word := g.Key.(*syntax.Ident)
	span := s.span(g.Key)
	switch {
	case isBad(g.Key):
		return
	case !word || !hasTarget(name):
		s.fail(diag.E1019.AtTarget(span, name))
		return
	case seen[name]:
		s.fail(diag.E1005.AtKey(span, name))
		return
	}
	seen[name] = true
	if isRefused(g.Value) {
		return
	}
	m, ok := g.Value.(*syntax.ProjectMap)
	if !ok {
		s.fail(diag.E1006.AtOutTemplates(s.span(g.Value)))
		return
	}
	roots := map[string]bool{}
	for _, r := range m.Entries {
		s.templateRoot(name, r, roots)
	}
}

// templateRoot reads one root of a target: declared and written as an identifier (E1009), once (E1005),
// holding a template string (E1006).
func (s *schema) templateRoot(target string, g *syntax.ProjectEntry, seen map[string]bool) {
	name, ident := s.keyName(g.Key)
	span := s.span(g.Key)
	switch {
	case isBad(g.Key):
		return
	case !ident || !s.named[name]:
		s.fail(diag.E1009.AtTemplateRoot(span, name))
		return
	case seen[name]:
		s.fail(diag.E1005.AtRoot(span, name))
		return
	}
	seen[name] = true
	if isRefused(g.Value) {
		return
	}
	reader := templateReader{schema: s, target: target, root: name}
	if parts, ok := reader.parts(g.Value); ok {
		s.p.Templates = append(s.p.Templates, Template{Target: target, Root: name, Span: span, parts: parts})
	}
}

// templateReader reads the text of one template of target and root.
type templateReader struct {
	*schema
	target, root string
}

// parts is a template's text and placeholders: each bad placeholder, else the first refusal of its text, is E1019.
func (r templateReader) parts(v syntax.ProjectValue) ([]templatePart, bool) {
	var parts []templatePart
	switch lit := v.(type) {
	case *syntax.RawStringLit:
		parts = []templatePart{{kind: partText, text: lit.Value}}
	case *syntax.StringLit:
		var ok bool
		if parts, ok = r.stringParts(lit); !ok {
			return nil, false
		}
	default:
		r.fail(diag.E1006.AtOutTemplates(r.span(v)))
		return nil, false
	}
	if b := r.refusal(v, parts); b != nil {
		r.fail(b)
		return nil, false
	}
	return parts, true
}

// stringParts splits a string literal at its interpolations; ok is false when one is no bare {dir} or {dotted}.
func (r templateReader) stringParts(lit *syntax.StringLit) ([]templatePart, bool) {
	var parts []templatePart
	ok := true
	for _, part := range lit.Parts {
		if part.Interp == nil {
			parts = append(parts, templatePart{kind: partText, text: part.Text})
			continue
		}
		kind, found := r.placeholder(part.Interp)
		ok = ok && found
		parts = append(parts, templatePart{kind: kind})
	}
	return parts, ok
}

// placeholder is the kind of a bare {dir} or {dotted}; any other interpolation is E1019 placeholder
// at its braces, but an empty one, which the lexer has refused (E1112).
func (r templateReader) placeholder(in *syntax.Interp) (partKind, bool) {
	from, to := r.f.Tokens[in.From-1], r.f.Tokens[in.To+1]
	span := source.Span{File: r.f.Src.ID, Start: from.End - 1, End: to.Start + 1}
	if kind, ok := placeholders[string(r.f.Src.Content[span.Start:span.End])]; ok {
		return kind, true
	}
	if in.From <= in.To {
		r.fail(diag.E1019.AtPlaceholder(span, span, r.target, r.root))
	}
	return partText, false
}

// refusal is the first finding that the text of a template, its placeholders replaced by a name,
// deserves: rooted, then path, then chars, then ts.
func (r templateReader) refusal(v syntax.ProjectValue, parts []templatePart) *diag.Builder {
	var b strings.Builder
	for _, p := range parts {
		if p.kind == partText {
			b.WriteString(p.text)
		} else {
			b.WriteString(placeholderStandIn)
		}
	}
	text, span := b.String(), r.span(v)
	bad := strings.IndexFunc(text, notPortable)
	switch {
	case strings.HasPrefix(text, rootMark):
		return diag.E1019.AtRooted(span, r.target, r.root)
	case slices.ContainsFunc(strings.Split(text, sep), isBadSegment):
		return diag.E1019.AtPath(span, r.target, r.root)
	case bad >= 0:
		c, _ := utf8.DecodeRuneInString(text[bad:])
		return diag.E1019.AtChars(span, r.target, r.root, c)
	case !strings.HasSuffix(text, emitTargets[r.target]):
		return diag.E1019.AtTs(span, r.root)
	}
	return nil
}

// isBadSegment is an empty, `.` or `..` path segment.
func isBadSegment(seg string) bool {
	return seg == "" || seg == currentSeg || seg == parentSeg
}

// notPortable is a character outside A-Z a-z 0-9 . _ - and / (DECISIONS 424).
func notPortable(r rune) bool {
	alnum := r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r))
	return !alnum && !strings.ContainsRune(portablePunct, r)
}

func hasTarget(name string) bool {
	_, ok := emitTargets[name]
	return ok
}

// isRefused reports a value the parser already refused; unlike isBad, an interpolated string is
// no refusal under out_templates.
func isRefused(n syntax.Node) bool {
	_, bad := n.(*syntax.BadExpr)
	return bad || n == nil
}

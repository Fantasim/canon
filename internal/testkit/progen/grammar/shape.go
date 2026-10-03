package grammar

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Parse parses src as the source file name and returns its tree and its findings.
func Parse(name string, src []byte) (*syntax.File, []diag.Finding) {
	return ParseKind(name, src, Source)
}

// ParseKind parses src as the file name of kind k and returns its tree and its findings.
func ParseKind(name string, src []byte, k Kind) (*syntax.File, []diag.Finding) {
	fs := &source.FileSet{}
	f, err := fs.Add(name, rootPath+name, src)
	if err != nil {
		return nil, nil
	}
	bag := diag.NewBag(fs, "")
	tree := syntax.Parse(f, k.Syntax(), bag)
	return tree, bag.Findings()
}

// Shape is what canon fmt keeps of a parsed file: its tokens but line breaks and commas, its
// comments, the imports sorted with theirs, durations by value, multiline strings by content.
func Shape(f *syntax.File) string {
	sh := &shaper{f: f, docs: attachedDocs(f)}
	var imports []string
	skip := map[int]bool{}
	for _, imp := range f.Imports {
		for i := imp.First(); i <= imp.Last(); i++ {
			skip[int(i)] = true
		}
		imports = append(imports, sh.importShape(imp))
	}
	slices.Sort(imports)
	sugared := sugaredColons(f)
	for i := range f.Tokens {
		if len(f.Imports) > 0 && i == int(f.Imports[0].First()) {
			sh.b.WriteString(strings.Join(imports, ""))
		}
		switch {
		case sugared[i]:
			sh.comments(&sh.b, f.Tokens[i].Leading, leadingTag)
			sh.comments(&sh.b, f.Tokens[i].Trailing, trailingTag)
		case !skip[i]:
			sh.token(&sh.b, i)
		}
	}
	sh.b.WriteString(values(f))
	return sh.b.String()
}

// sugaredColons are the colons of project.canon map items, written "key { … }" (FORMATTER.md §10).
func sugaredColons(f *syntax.File) map[int]bool {
	out := map[int]bool{}
	if f.Project == nil {
		return out
	}
	for _, e := range f.Project.Items {
		if _, ok := e.Value.(*syntax.ProjectMap); ok && e.Colon != syntax.NoTok {
			out[int(e.Colon)] = true
		}
	}
	return out
}

// shaper writes a file's shape; docs are the byte ranges of its attached doc blocks.
type shaper struct {
	f    *syntax.File
	docs []docRange
	b    strings.Builder
}

// docRange is the bytes of an attached doc block.
type docRange struct {
	start, end source.Pos
}

// attachedDocs are the doc blocks the parser attached to an item (GRAMMAR.md §9.1).
func attachedDocs(f *syntax.File) []docRange {
	var out []docRange
	syntax.Inspect(f, func(n syntax.Node) bool {
		if d, ok := n.(*syntax.DocComment); ok && d != nil {
			out = append(out, docRange{d.Start, d.End})
		}
		return true
	})
	return out
}

// importShape is an import's path, alias, names in byte order, then its comments, which move
// with it.
func (sh *shaper) importShape(imp *syntax.Import) string {
	names := make([]string, 0, len(imp.Names))
	for _, n := range imp.Names {
		names = append(names, n.Name)
	}
	slices.Sort(names)
	var b strings.Builder
	for i := imp.Path.First(); i <= imp.Path.Last(); i++ {
		b.WriteString(tokenShape(sh.f, sh.f.Tokens[i]))
	}
	if imp.Alias != nil {
		b.WriteString(imp.Alias.Name + newline)
	}
	b.WriteString(strings.Join(names, space) + newline)
	for i := imp.First(); i <= imp.Last(); i++ {
		sh.comments(&b, sh.f.Tokens[i].Leading, leadingTag)
		sh.comments(&b, sh.f.Tokens[i].Trailing, trailingTag)
	}
	return b.String()
}

// token writes a token's shape between its leading and trailing comments (FORMATTER.md §8.1).
func (sh *shaper) token(b *strings.Builder, i int) {
	t := sh.f.Tokens[i]
	sh.comments(b, t.Leading, leadingTag)
	b.WriteString(tokenShape(sh.f, t))
	sh.comments(b, t.Trailing, trailingTag)
}

// tokenShape is a token's kind, and its text when the kind does not fix it; line breaks and
// commas are nothing, durations and multiline string parts are left to values.
func tokenShape(f *syntax.File, t syntax.Token) string {
	switch t.Kind {
	case syntax.TokBOF, syntax.TokEOF, syntax.TokNL, syntax.TokComma:
		return ""
	case syntax.TokDuration, syntax.TokMLString, syntax.TokMLStringHead, syntax.TokMLStringMid, syntax.TokMLStringTail, syntax.TokRawML:
		return t.Kind.String() + newline
	default:
		if t.Kind < syntax.TokEllipsis {
			return t.Kind.String() + string(f.Src.Content[t.Start:t.End]) + newline
		}
		return t.Kind.String() + newline
	}
}

// comments writes each comment of trivia, tagged with its place and, for a doc line, whether
// it is attached, as canon fmt keeps it: trailing whitespace removed, a doc line spaced.
func (sh *shaper) comments(b *strings.Builder, trivia []syntax.Trivia, tag string) {
	for _, tv := range trivia {
		if tv.Kind != syntax.TriviaLineComment && tv.Kind != syntax.TriviaDocComment && tv.Kind != syntax.TriviaBlockComment {
			continue
		}
		lines := strings.Split(string(sh.f.Src.Content[tv.Start:tv.End]), newline)
		for k, l := range lines {
			lines[k] = strings.TrimRight(l, trailingSpace)
		}
		text := strings.Join(lines, newline)
		kind := tag
		if len(lines) > 1 {
			kind = leadingTag // a block comment over several lines is printed on its own lines
		}
		if tv.Kind == syntax.TriviaDocComment {
			kind += sh.docTag(tv.Start)
			if body, ok := strings.CutPrefix(text, docSlashes); ok && body != "" && !strings.HasPrefix(body, space) {
				text = docPrefix + body
			}
		}
		b.WriteString(commentMark + kind + space + strconv.Quote(text) + newline)
	}
}

// docTag tells a doc line attached to an item from a detached one.
func (sh *shaper) docTag(at source.Pos) string {
	for _, d := range sh.docs {
		if d.start <= at && at < d.end {
			return attachedTag
		}
	}
	return detachedTag
}

// values are the durations and multiline string contents of f, in source order.
func values(f *syntax.File) string {
	var b strings.Builder
	syntax.Inspect(f, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.DurationLit:
			b.WriteString(strconv.FormatInt(n.Millis, decimalBase) + newline)
		case *syntax.StringLit:
			for _, p := range n.Parts {
				if n.Multiline && p.Interp == nil {
					b.WriteString(strconv.Quote(p.Text) + newline)
				}
			}
		case *syntax.RawStringLit:
			if n.Multiline {
				b.WriteString(strconv.Quote(n.Value) + newline)
			}
		}
		return true
	})
	return b.String()
}

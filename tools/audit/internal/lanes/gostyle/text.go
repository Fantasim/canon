package gostyle

import (
	"bytes"
	"go/ast"
	"go/types"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
)

// splitLines splits src into physical lines, dropping the empty tail a trailing "\n" leaves.
func splitLines(src []byte) [][]byte {
	lines := bytes.Split(src, []byte(newline))
	if n := len(lines); n > 0 && len(lines[n-1]) == 0 {
		lines = lines[:n-1]
	}
	return lines
}

// countLines is every physical line in src.
func countLines(src []byte) int { return len(splitLines(src)) }

// countNonBlank is every physical line in src with visible content.
func countNonBlank(src []byte) int {
	n := 0
	for _, l := range splitLines(src) {
		if !gosrc.IsBlank(l) {
			n++
		}
	}
	return n
}

// isDirective reports whether a raw comment line is a tool directive or the generated-file
// marker, never counted toward a doc or block's length.
func isDirective(text string) bool {
	t := strings.TrimSpace(text)
	return strings.HasPrefix(t, directiveGo) || strings.HasPrefix(t, directiveNolint) ||
		strings.HasPrefix(t, directiveSovaudit) || reGeneratedHeader.MatchString(t)
}

// commentTextLines is one *ast.Comment's own physical line count.
func commentTextLines(c *ast.Comment) int { return strings.Count(c.Text, newline) + 1 }

// contentLines sums a comment group's lines, skipping directive lines.
func contentLines(g *ast.CommentGroup) int {
	n := 0
	for _, c := range g.List {
		if isDirective(c.Text) {
			continue
		}
		n += commentTextLines(c)
	}
	return n
}

// fieldListCount counts parameters or results: each name counts, an unnamed field counts 1.
func fieldListCount(fl *ast.FieldList) int {
	if fl == nil {
		return 0
	}
	n := 0
	for _, f := range fl.List {
		if len(f.Names) == 0 {
			n++
			continue
		}
		n += len(f.Names)
	}
	return n
}

// namedResults reports whether a function type has at least one named result.
func namedResults(ft *ast.FuncType) bool {
	if ft.Results == nil {
		return false
	}
	for _, f := range ft.Results.List {
		if len(f.Names) > 0 {
			return true
		}
	}
	return false
}

// fieldName names a struct field for a stable Detail: its own name, or its type's text
// for an embedded field.
func fieldName(f *ast.Field) string {
	if len(f.Names) > 0 {
		return f.Names[0].Name
	}
	return types.ExprString(f.Type)
}

// specName names a ValueSpec/TypeSpec inside a parenthesized const/var/type group.
func specName(s ast.Spec) string {
	switch s := s.(type) {
	case *ast.TypeSpec:
		return s.Name.Name
	case *ast.ValueSpec:
		if len(s.Names) > 0 {
			return s.Names[0].Name
		}
	}
	return ""
}

// nonNil filters out nil comment groups, e.g. a spec with a Doc but no trailing Comment.
func nonNil(gs ...*ast.CommentGroup) []*ast.CommentGroup {
	var out []*ast.CommentGroup
	for _, g := range gs {
		if g != nil {
			out = append(out, g)
		}
	}
	return out
}

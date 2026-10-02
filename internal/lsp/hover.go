package lsp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// hoverResult is LSP 3.17's Hover, its contents Markdown.
type hoverResult struct {
	Contents markupContent `json:"contents"`
	Range    textRange     `json:"range"`
}

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// hover is the canonical type text, doc comment and default of the name at the position, and
// for a let, a constant or an entry its value's canonical text, cut to hoverLines lines and
// hoverBytes bytes; null where no name is.
func (s *server) hover(ctx context.Context, params json.RawMessage) (any, error) {
	// IMPLEMENTATION-PLAN §8.4 Features, CLI.md §4
	sp, err := s.spotAt(ctx, params)
	if sp == nil {
		return nil, err
	}
	obj, at := sp.object()
	if obj == nil {
		return nil, nil
	}
	_, rng, ok := sp.conv.spanOf(sp.file.Span(at))
	if !ok {
		return nil, nil
	}
	parts := []string{fenced(signature(obj))}
	if doc := docOf(obj.Decl()); doc != nil && doc.Text != "" {
		parts = append(parts, doc.Text)
	}
	if text, ok := defaultOf(obj); ok {
		parts = append(parts, defaultLabel+fenced(text))
	}
	if text, ok := sp.valuePart(obj); ok {
		parts = append(parts, text)
	}
	return hoverResult{Contents: markupContent{Kind: markdown, Value: strings.Join(parts, paragraphSep)}, Range: rng}, nil
}

// hasValue are the kinds of name whose hover shows a value: an enum member is its own name.
var hasValue = map[check.ObjKind]bool{check.ObjLet: true, check.ObjConst: true, check.ObjEntry: true}

// signature is a type's canonical text, or a name and its type's (API.md §5.2 TypeInfo.Expr).
func signature(obj check.Object) string {
	t := obj.Type()
	switch {
	case t == nil:
		return obj.Name()
	case obj.Kind() == check.ObjTypeName:
		return t.String()
	}
	return obj.Name() + typeSep + t.String()
}

func fenced(text string) string {
	return fenceOpen + text + fenceClose
}

// valuePart is the value of a let, a constant or an entry, labelled; in a layer file, the base
// value, the analysis having no layer active.
func (sp *spot) valuePart(obj check.Object) (string, bool) {
	// IMPLEMENTATION-PLAN §8.4 Features
	if !hasValue[obj.Kind()] {
		return "", false
	}
	r, ok := sp.valueOf(obj)
	if !ok {
		return "", false
	}
	label := valueLabel
	if sp.file.FileKind == syntax.FileLayer {
		label = baseValueLabel
	}
	return label + fenced(hoverText(r.Target)), true
}

// hoverText is v's canonical text cut to hoverLines lines and hoverBytes bytes, never built
// further (DECISIONS 197), a last line marking a cut.
func hoverText(v value.Value) string {
	cut := value.TextLenUpTo(v, hoverBytes+1) > hoverBytes
	lines := strings.SplitN(value.TextUpTo(v, hoverBytes), lineSep, hoverLines+1)
	if len(lines) > hoverLines {
		lines, cut = lines[:hoverLines], true
	}
	text := strings.Join(lines, lineSep)
	if cut {
		text += lineSep + cutMark
	}
	return text
}

// defaultOf is the default expression of a field or a parameter in its canonical layout
// (FORMATTER.md), as written when it does not print; false without one.
func defaultOf(obj check.Object) (string, bool) {
	var def syntax.Expr
	switch d := obj.Decl().(type) {
	case *syntax.FieldDecl:
		def = d.Default
	case *syntax.Param:
		def = d.Default
	}
	f := obj.File()
	if def == nil || f == nil {
		return "", false
	}
	if text, err := format.Node(f, def, format.Place{}); err == nil {
		return string(text), true
	}
	at := f.Span(def)
	return string(f.Src.Content[at.Start:at.End]), true
}

// docOf is the doc comment of a declaration, nil for none.
func docOf(n syntax.Node) *syntax.DocComment {
	switch d := n.(type) {
	case *syntax.ConstDecl:
		return d.Doc
	case *syntax.LetDecl:
		return d.Doc
	case *syntax.TypeDecl:
		return d.Doc
	case *syntax.FnDecl:
		return d.Doc
	case *syntax.EntryDecl:
		return d.Doc
	case *syntax.CheckDecl:
		return d.Doc
	case *syntax.WidgetDecl:
		return d.Doc
	case *syntax.TestDecl:
		return d.Doc
	case *syntax.RecordDecl:
		return d.Doc
	case *syntax.FieldDecl:
		return d.Doc
	case *syntax.EnumDecl:
		return d.Doc
	case *syntax.EnumMember:
		return d.Doc
	case *syntax.VariantDecl:
		return d.Doc
	case *syntax.VariantCase:
		return d.Doc
	case *syntax.EntryItem:
		return d.Doc
	}
	return nil
}

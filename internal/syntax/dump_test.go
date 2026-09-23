package syntax_test

import (
	"fmt"
	"math/big"
	"reflect"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// dumper writes the AST golden format of testdata/ast: one node per line, two spaces per level,
// "<Field>: <Kind> [name] [attr=value …] @<line>:<col>-<line>:<col>", then the findings (the
// format is described in full in DECISIONS 136).
type dumper struct {
	f   *syntax.File
	out strings.Builder
}

var (
	nodeType  = reflect.TypeFor[syntax.Node]()
	boundsTyp = reflect.TypeFor[syntax.Bounds]()
	delimsTyp = reflect.TypeFor[syntax.Delims]()
	tokType   = reflect.TypeFor[syntax.Tok]()
	kindType  = reflect.TypeFor[syntax.TokenKind]()
	partsType = reflect.TypeFor[[]syntax.StringPart]()
	specType  = reflect.TypeFor[*syntax.FormatSpec]()
	bigType   = reflect.TypeFor[*big.Int]()
	skipTypes = []reflect.Type{
		boundsTyp, delimsTyp, reflect.TypeFor[*source.File](), reflect.TypeFor[[]syntax.Token](),
		reflect.TypeFor[source.Pos](),
	}
)

// dumpFile renders a parsed file in the AST golden format.
func dumpFile(f *syntax.File) string {
	d := &dumper{f: f}
	d.node("File", f, 0)
	return d.out.String()
}

func (d *dumper) node(label string, n syntax.Node, depth int) {
	v := reflect.ValueOf(n).Elem()
	var attrs, kids []string
	fmt.Fprintf(&d.out, "%s%s: %s", strings.Repeat("  ", depth), label, n.Kind())
	switch n := n.(type) {
	case *syntax.Ident:
		attrs = append(attrs, n.Name)
	case *syntax.IdentExpr:
		attrs = append(attrs, n.Name)
	case *syntax.QualifiedName:
		parts := make([]string, len(n.Parts))
		for i, p := range n.Parts {
			parts[i] = p.Name
		}
		attrs = append(attrs, strings.Join(parts, "."))
	case *syntax.DocComment:
		attrs = append(attrs, fmt.Sprintf("%q", n.Text))
	default:
		attrs, kids = d.fields(v)
	}
	s := d.f.Span(n)
	if _, isFile := n.(*syntax.File); isFile {
		attrs = append(attrs, "FileKind="+fileKinds[n.(*syntax.File).FileKind])
	}
	for _, a := range attrs {
		d.out.WriteString(" " + a)
	}
	l := d.f.Src.Position
	sl, sc := l(s.Start)
	el, ec := l(s.End)
	fmt.Fprintf(&d.out, " @%d:%d-%d:%d\n", sl, sc, el, ec)
	d.children(v, kids, depth+1)
}

var fileKinds = map[syntax.FileKind]string{
	syntax.FileInvalid: "invalid", syntax.FileSource: "source", syntax.FileLayer: "layer",
	syntax.FileTranslation: "translation", syntax.FileProject: "project",
}

// fields splits a node's fields into inline attributes and the names of child fields.
func (d *dumper) fields(v reflect.Value) (attrs, kids []string) {
	t := v.Type()
	for i := range t.NumField() {
		sf, fv := t.Field(i), v.Field(i)
		if slices.Contains(skipTypes, sf.Type) || sf.Type == reflect.TypeFor[syntax.FileKind]() {
			continue
		}
		if a, ok := d.attr(sf.Name, fv); ok {
			if a != "" {
				attrs = append(attrs, a)
			}
			continue
		}
		kids = append(kids, sf.Name)
	}
	return attrs, kids
}

// attr renders a non-node field as an attribute ("" when it says nothing); ok is false for a
// field holding nodes.
func (d *dumper) attr(name string, fv reflect.Value) (string, bool) {
	switch fv.Type() {
	case tokType:
		if t := syntax.Tok(fv.Int()); t.Valid() && name != opTokField {
			return fmt.Sprintf("%s=%q", name, d.text(t)), true
		}
		return "", true
	case kindType:
		if k := syntax.TokenKind(fv.Uint()); k != syntax.TokInvalid {
			return fmt.Sprintf("%s=%s", name, k), true
		}
		return "", true
	case bigType:
		return fmt.Sprintf("%s=%s", name, fv.Interface()), true
	case specType:
		if fv.IsNil() {
			return "", true
		}
		return fmt.Sprintf("%s=%q", name, d.text(fv.Interface().(*syntax.FormatSpec).Tok)), true
	case partsType:
		return "", false
	}
	switch fv.Kind() {
	case reflect.Bool:
		if fv.Bool() {
			return name, true
		}
		return "", true
	case reflect.String:
		return fmt.Sprintf("%s=%q", name, fv.String()), true
	case reflect.Int, reflect.Int64:
		return fmt.Sprintf("%s=%d", name, fv.Int()), true
	}
	return "", false
}

func (d *dumper) text(t syntax.Tok) string {
	tok := d.f.Tokens[t]
	return string(d.f.Src.Content[tok.Start:tok.End])
}

// children dumps the node fields named kids, a list element by element.
func (d *dumper) children(v reflect.Value, kids []string, depth int) {
	for _, name := range kids {
		fv := v.FieldByName(name)
		switch {
		case fv.Type() == partsType:
			d.parts(name, fv.Interface().([]syntax.StringPart), depth)
		case fv.Kind() == reflect.Slice:
			for i := range fv.Len() {
				d.child(fmt.Sprintf("%s[%d]", name, i), fv.Index(i), depth)
			}
		default:
			d.child(name, fv, depth)
		}
	}
}

func (d *dumper) child(label string, fv reflect.Value, depth int) {
	if fv.IsNil() || !fv.Type().Implements(nodeType) {
		return
	}
	n := fv.Interface().(syntax.Node)
	if rv := reflect.ValueOf(n); rv.Kind() == reflect.Pointer && rv.IsNil() {
		return
	}
	d.node(label, n, depth)
}

func (d *dumper) parts(name string, parts []syntax.StringPart, depth int) {
	for i, p := range parts {
		label := fmt.Sprintf("%s[%d]", name, i)
		if p.Interp != nil {
			d.node(label, p.Interp, depth)
			continue
		}
		fmt.Fprintf(&d.out, "%s%s: text %q\n", strings.Repeat("  ", depth), label, p.Text)
	}
}

package ir

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// TextFiles are the `@text` files of an accepted cp (p.TextFns: kept out of p.Fns, DECISIONS 300), in source order, each its fn's String value verbatim, or any other value as JSON (CODEGEN.md §2.9).
func TextFiles(cp *check.Package, p *Package) ([]File, error) {
	var out []File
	for _, f := range sourceFiles(cp) {
		for _, fd := range textFns(f) {
			content, err := textContent(p, fd.Name.Name)
			if err != nil {
				return nil, err
			}
			out = append(out, File{Path: textName(annotation(fd.Annotations, syntax.AnnText)), Content: content})
		}
	}
	return out, nil
}

// textFns are the fns of f carrying `@text`, in declaration order.
func textFns(f *syntax.File) []*syntax.FnDecl {
	var out []*syntax.FnDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*syntax.FnDecl); ok && fd.Name != nil && annotation(fd.Annotations, syntax.AnnText) != nil {
			out = append(out, fd)
		}
	}
	return out
}

// textName is a `@text` annotation's file name: its one positional string.
func textName(a *syntax.Annotation) string {
	for _, x := range a.Args {
		if x.Name == nil {
			return constString(x.Value)
		}
	}
	return ""
}

// textContent is the bytes of the precomputed value of p's export fn name: a String verbatim, any other result as JSON (WIRE.md §8.5, DECISIONS 308). A value stage E refused has no file: wire.Text's error is returned, never a panic.
func textContent(p *Package, name string) ([]byte, error) {
	for _, fn := range p.TextFns {
		if fn.Name != name {
			continue
		}
		if fn.Value == nil {
			break
		}
		if isText(fn.Result) {
			if s, ok := fn.Value.(*value.Str); ok {
				return []byte(s.V), nil
			}
			break
		}
		b, err := wire.Text(fn.Value)
		if err != nil {
			return nil, fmt.Errorf(fmtTextWire, ErrInternal, p.Name, name, err)
		}
		return b, nil
	}
	return nil, fmt.Errorf(fmtNoText, ErrInternal, p.Name, name)
}

// isText reports a `@text` result type written verbatim: String, an alias or refinement of it, or a literal union of strings (DECISIONS 308).
func isText(r TypeRef) bool {
	return r.Kind == types.String || r.Kind == types.LitUnion && r.Elem != nil && isText(*r.Elem)
}

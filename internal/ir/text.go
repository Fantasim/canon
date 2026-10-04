package ir

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// TextFiles are the `@text` files of an accepted cp, in source order, each its fn's String value verbatim (CODEGEN.md §2.9).
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

// textContent is the bytes of the precomputed String value of p's export fn name.
func textContent(p *Package, name string) ([]byte, error) {
	for _, fn := range p.Fns {
		if fn.Name != name {
			continue
		}
		if s, ok := fn.Value.(*value.Str); ok {
			return []byte(s.V), nil
		}
	}
	return nil, fmt.Errorf(fmtNoText, ErrInternal, p.Name, name)
}

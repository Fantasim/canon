package ir

import (
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// TextFiles are the `@text` files of an accepted cp (p.TextFns: kept out of p.Fns, DECISIONS 300), in source order, each its fn's String value verbatim, or any other value as JSON; an optional result that is none has no file (CODEGEN.md §2.9, DECISIONS 336).
func TextFiles(cp *check.Package, p *Package) ([]File, error) {
	var out []File
	for _, f := range sourceFiles(cp) {
		for _, fd := range textFns(f) {
			content, written, err := textContent(p, fd.Name.Name)
			if err != nil {
				return nil, err
			}
			if !written {
				continue
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

// textContent is the bytes of the precomputed value of p's export fn name: a String verbatim, any other result as JSON (WIRE.md §8.5, DECISIONS 308); written is false for an optional result that is none, which has no file (DECISIONS 336). A value stage E refused has no file: wire.Text's error is returned, never a panic.
func textContent(p *Package, name string) (content []byte, written bool, err error) {
	i := slices.IndexFunc(p.TextFns, func(fn *ExportFn) bool { return fn.Name == name })
	if i < 0 || p.TextFns[i].Value == nil {
		return nil, false, fmt.Errorf(fmtNoText, ErrInternal, p.Name, name)
	}
	fn := p.TextFns[i]
	result := fn.Result
	if result.Kind == types.Optional && result.Elem != nil {
		if _, none := fn.Value.(*value.None); none {
			return nil, false, nil
		}
		result = *result.Elem
	}
	if isText(result) {
		s, ok := fn.Value.(*value.Str)
		if !ok {
			return nil, false, fmt.Errorf(fmtNoText, ErrInternal, p.Name, name)
		}
		return []byte(s.V), true, nil
	}
	b, err := wire.Text(fn.Value)
	if err != nil {
		return nil, false, fmt.Errorf(fmtTextWire, ErrInternal, p.Name, name, err)
	}
	return b, true, nil
}

// isText reports a `@text` result type written verbatim: String, an alias or refinement of it, or a literal union of strings (DECISIONS 308).
func isText(r TypeRef) bool {
	return r.Kind == types.String || r.Kind == types.LitUnion && r.Elem != nil && isText(*r.Elem)
}

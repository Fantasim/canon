package canon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/typedef"
)

// typeEncoder writes TypeInfo.VM with the model's own type writer, locked (API.md §5.2, VIEWMODEL.md §12.3).
type typeEncoder struct {
	mu   sync.Mutex
	defs *typedef.Types
}

// newTypeEncoder reads a's program without views, asset roots by a's layout, refs counted as a settled (R4).
func newTypeEncoder(ctx context.Context, a *build.Analysis) *typeEncoder {
	prog := a.Program()
	force := func(pkg, name string) (value.Value, bool) { return a.Force(eval.Root{Pkg: pkg, Name: name}) }
	in := typedef.Input{Program: prog, Colls: encode.NewColls(force), Assets: encode.NewAssets(prog, a.Layout())}
	return &typeEncoder{defs: typedef.New(ctx, in, "")}
}

// site is where a type is written: the record or case declaring it (J12), and the field it is
// the type of, nil for an element, an entry or a root.
type site struct {
	decl  types.Type
	field *types.Field
}

// expr is t's type expression as compact JSON: a field's as its definition writes it (C32).
func (e *typeEncoder) expr(at site, t types.Type) (json.RawMessage, error) {
	var x vm.TypeExpr
	e.mu.Lock()
	if at.field != nil {
		x = e.defs.FieldExpr(at.decl, at.field)
	} else {
		x = e.defs.Expr(at.decl, t)
	}
	e.mu.Unlock()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(x); err != nil {
		return nil, fmt.Errorf(fmtTypeVM, t, err)
	}
	return bytes.TrimSuffix(buf.Bytes(), lineEnd), nil
}

// siteOf is where r's target is written: the nearest record on its path, and the field when
// that record holds the target directly.
func (s *snapshot) siteOf(r edit.Resolved) site {
	for i, st := range slices.Backward(r.Steps) {
		rec, ok := s.parentOf(r, i).(*value.Record)
		if !ok {
			continue
		}
		at := site{decl: declType(rec.T)}
		if i == len(r.Steps)-1 {
			at.field = fieldNamed(rec.T, st.Seg.Name)
		}
		return at
	}
	return site{}
}

// parentOf is the value step i of r is read in: the previous step's, or the root's.
func (s *snapshot) parentOf(r edit.Resolved, i int) value.Value {
	if i > 0 {
		return r.Steps[i-1].Value
	}
	p, err := edit.Parse(r.Canonical)
	if err != nil {
		return nil
	}
	root, _ := s.a.Force(eval.Root{Pkg: p.Package, Name: p.Root})
	return root
}

// fieldNamed is the field name of the record or case t; nil for a pseudo-field.
func fieldNamed(t types.Type, name string) *types.Field {
	for _, f := range encode.FieldsOf(t) {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// declType is the record or case type whose fields the model writes; an applied record's record.
func declType(t types.Type) types.Type {
	switch b := t.Base().(type) {
	case *types.AppliedRecord:
		return b.Rec
	case *types.RecordType, *types.CaseType:
		return b
	}
	return nil
}

package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/fantasim/canonlang/internal/types"
)

// Schema is the `$schema` of value name of package pkg, fns its `$fns` (FINGERPRINT.md §2).
func Schema(pkg, name string, t *TypeRef, fns []*ExportFn) (string, error) {
	text, err := Fingerprint(t, fns)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(text)
	return schemaName(pkg, name, t) + fpAt + hex.EncodeToString(sum[:])[:fpHashChars], nil
}

// schemaName is the name part of FINGERPRINT.md §2.2, t already validated by Fingerprint.
func schemaName(pkg, name string, t *TypeRef) string {
	if t.Kind == types.Optional {
		t = t.Elem
	}
	if t.Kind == types.List || t.Kind == types.Table {
		t = t.Elem
		if t.Kind == types.Optional {
			t = t.Elem
		}
	}
	switch t.Kind {
	case types.Record, types.Variant, types.Enum:
		return t.Named.QName()
	default:
		return pkg + qnameSep + name
	}
}

// Fingerprint is the canon-fp v1 text of a value of type t with `$fns` fns (FINGERPRINT.md §4).
func Fingerprint(t *TypeRef, fns []*ExportFn) ([]byte, error) {
	fp := &fingerprint{num: map[Type]int{}}
	stored := precomputed(fns)
	if err := fp.walk(t); err != nil {
		return nil, err
	}
	for _, fn := range stored {
		if err := fp.walkFn(fn); err != nil {
			return nil, err
		}
	}
	return fp.text(t, stored)
}

// text prints the header, the root and `fn` lines, then one block per number (§4.2).
func (fp *fingerprint) text(t *TypeRef, fns []*ExportFn) ([]byte, error) {
	b, err := fp.typ(append([]byte(fpHeader), fpRoot...), t)
	if err != nil {
		return nil, err
	}
	b = append(b, fpNewline...)
	for _, fn := range fns {
		if b, err = fp.fnLine(b, "", fn.Name, fn); err != nil {
			return nil, err
		}
	}
	for i, n := range fp.order {
		if b, err = fp.block(b, i, n); err != nil {
			return nil, err
		}
	}
	return b, nil
}

// fingerprint numbers the named types in walk order (FINGERPRINT.md §4.3).
type fingerprint struct {
	num   map[Type]int
	order []Type
}

// walk numbers named types depth first, pre-order, left to right (FINGERPRINT.md §4.3).
func (fp *fingerprint) walk(t *TypeRef) error {
	if t == nil {
		return fmt.Errorf("%w: missing type", ErrFingerprint)
	}
	switch t.Kind {
	case types.Enum, types.Record, types.Variant, types.Case:
		return fp.named(t.Named)
	case types.TypeApp:
		return fp.walkArms(t)
	default:
	}
	for _, sub := range [...]*TypeRef{t.Key, t.Elem} {
		if sub == nil {
			continue
		}
		if err := fp.walk(sub); err != nil {
			return err
		}
	}
	return nil
}

// walkArms walks a dependent type's arms in discriminant member order.
func (fp *fingerprint) walkArms(t *TypeRef) error {
	d, ok := t.Named.(*Dependent)
	if !ok {
		return fmt.Errorf("%w: type application of %T", ErrFingerprint, t.Named)
	}
	for _, arm := range d.ByMember {
		if arm == NoBranch || arm >= len(d.Branches) {
			continue
		}
		if err := fp.walk(&d.Branches[arm].Type); err != nil {
			return err
		}
	}
	return nil
}

func (fp *fingerprint) named(n Type) error {
	if n == nil {
		return fmt.Errorf("%w: named type missing", ErrFingerprint)
	}
	if _, seen := fp.num[n]; seen {
		return nil
	}
	fp.num[n] = len(fp.order)
	fp.order = append(fp.order, n)
	switch x := n.(type) {
	case *Record:
		return fp.walkBody(x.Fields, x.Methods)
	case *Variant:
		for _, c := range x.Cases {
			if err := fp.walkBody(c.Fields, c.Methods); err != nil {
				return err
			}
		}
		return nil
	case *Enum:
		return nil
	}
	return fmt.Errorf("%w: named type %T", ErrFingerprint, n)
}

// walkBody walks the non-input fields, then the `$` functions, in declaration order.
func (fp *fingerprint) walkBody(fields []*Field, methods []*ExportFn) error {
	for _, f := range fields {
		if f.Input == nil {
			if err := fp.walk(&f.Type); err != nil {
				return err
			}
		}
	}
	for _, m := range precomputed(methods) {
		if err := fp.walkFn(m); err != nil {
			return err
		}
	}
	return nil
}

// walkFn walks a `$` function's wire type, map(A1, map(A2, … R)) (§4.5).
func (fp *fingerprint) walkFn(fn *ExportFn) error {
	for _, p := range fn.Params {
		if err := fp.walk(&p.Type); err != nil {
			return err
		}
	}
	return fp.walk(&fn.Result)
}

// precomputed is the export fns data stores: every one but the translated (WIRE.md §5.11).
func precomputed(fns []*ExportFn) []*ExportFn {
	var out []*ExportFn
	for _, fn := range fns {
		if fn.Kind != FnTranslated {
			out = append(out, fn)
		}
	}
	return out
}

package canon

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// TypeInfo describes the type of a value: its canonical text and its view model encoding.
type TypeInfo struct {
	Expr string
	VM   json.RawMessage
}

// Origin records where a value comes from; Replaced is the origin an amendment replaced (API.md §5.2).
type Origin struct {
	Kind OriginKind
	Span
	Pointer    string
	Layer      string
	Via        *Origin
	Stack      []Frame
	Text       string // canonical text (STD-06) of the value this origin produced; "" when not known
	MoreFrames int    // frames cut from Stack (F13)
	Replaced   *Origin
}

// Editability says whether and where a value can be edited (API.md §5.2).
type Editability struct {
	Mode   EditMode
	Reason Reason
	File   string
	Origin string
	Layer  string
}

// Value is a value of a snapshot, with its type and origin; it holds its snapshot (rule R4).
type Value struct {
	Path     string
	Kind     ValueKind
	Type     TypeInfo
	Text     string
	Origin   Origin
	Editable Editability
	snap     *snapshot
	res      edit.Resolved
}

// String returns the canonical Canon text of the value.
func (v *Value) String() string {
	return v.Text
}

// Bool returns the value and true if the value is a Bool.
func (v *Value) Bool() (bool, bool) {
	b, ok := v.res.Target.(*value.Bool)
	if !ok {
		return false, false
	}
	return b.V, true
}

// Int returns the value and true if the value is of an integer type.
func (v *Value) Int() (int64, bool) {
	n, ok := v.res.Target.(*value.Int)
	if !ok {
		return 0, false
	}
	return n.V, true
}

// Float returns the value and true if the value is a Float or Float32.
func (v *Value) Float() (float64, bool) {
	f, ok := v.res.Target.(*value.Float)
	if !ok {
		return 0, false
	}
	return f.V, true
}

// Str returns the value and true if the value is a String or an asset.
func (v *Value) Str() (string, bool) {
	s, ok := v.res.Target.(*value.Str)
	if !ok {
		return "", false
	}
	return s.V, true
}

// Dur returns the value and true if the value is a Duration.
func (v *Value) Dur() (time.Duration, bool) {
	d, ok := v.res.Target.(*value.Dur)
	if !ok {
		return 0, false
	}
	return time.Duration(d.Ms) * time.Millisecond, true
}

// Member returns the enum's qualified name and the member's Canon name; a variant's `.kind` is a
// member of its Kind enum (API.md P3).
func (v *Value) Member() (enum, name string, ok bool) {
	switch m := v.res.Target.(type) {
	case *value.Member:
		return m.Enum.String(), m.CanonText(), true
	case *value.CaseKind:
		return m.T.String(), m.CanonText(), true
	}
	return "", "", false
}

// Case returns the current case of a variant.
func (v *Value) Case() (string, bool) {
	r, ok := v.res.Target.(*value.Record)
	if !ok {
		return "", false
	}
	c, ok := r.T.Base().(*types.CaseType)
	if !ok {
		return "", false
	}
	return c.Name, true
}

// Key returns the key of a ref, table entry or keyed-list element, in canonical text.
func (v *Value) Key() (string, bool) {
	switch x := v.res.Target.(type) {
	case *value.Ref:
		return x.Key.Text(), true
	case *value.Record:
		if x.Ident != nil {
			return x.Ident.Key.Text(), true
		}
	}
	return "", false
}

// IsNone reports whether the value is none.
func (v *Value) IsNone() bool {
	return v.Kind == KindNone
}

// Len returns the number of elements, of fields of a record or a variant's current case, or 0.
func (v *Value) Len() int {
	return len(childSegs(v.res.Target))
}

// Children returns fields in declaration order, elements, or map entries in insertion order,
// read in the value's own snapshot (rule R4). A child it cannot read is a compiler bug: Children
// panics with its *InternalError.
func (v *Value) Children() []*Value {
	segs := childSegs(v.res.Target)
	if v.snap == nil || len(segs) == 0 {
		return nil
	}
	base, err := edit.Parse(v.res.Canonical)
	if err != nil {
		panic(internalError(err))
	}
	out := make([]*Value, 0, len(segs))
	for _, seg := range segs {
		c, err := v.snap.resolve(v.res.Canonical, withSeg(base, seg))
		if err != nil {
			panic(internalError(err))
		}
		out = append(out, c)
	}
	return out
}

// Child returns one child by a path segment: ".f", "[k]" or "[#n]" (API.md §6.2).
func (v *Value) Child(seg string) (*Value, error) {
	path := v.res.Canonical + seg
	one, err := edit.Parse(segmentRoot + seg)
	switch {
	case err != nil:
		return nil, syntaxError(path, err, len(v.res.Canonical)-len(segmentRoot))
	case len(one.Segs) != 1 || one.Package != "":
		return nil, &PathError{Op: -1, Path: path, Err: ErrBadPath, Detail: fmt.Sprintf(fmtSegment, seg)}
	case v.snap == nil:
		return nil, &PathError{Op: -1, Path: path, Err: ErrNoPath}
	}
	base, err := edit.Parse(v.res.Canonical)
	if err != nil {
		return nil, internalError(err)
	}
	return v.snap.resolve(path, withSeg(base, one.Segs[0]))
}

// JSON returns the wire form of the value (WIRE.md), compact; nil for a value without one (a
// range, a pair). A value it fails to encode otherwise is a compiler bug: JSON panics with its *InternalError.
func (v *Value) JSON() []byte {
	if v.res.Target == nil {
		return nil
	}
	return wireJSON(v.res.Target)
}

// Ref is one place that references an entry or member (API.md §5.3).
type Ref struct {
	Kind    RefKind
	Package string
	Path    string
	Span
}

// RefsResult lists every reference to Target, in the order of rule F2.
type RefsResult struct {
	Target string
	Refs   []Ref
}

// Refs lists every place that references the entry, element or member at path (rules R7, R8).
func (p *Project) Refs(ctx context.Context, path string) (*RefsResult, error) {
	return nil, errUnimplemented()
}

// ViewModel is the view model of one package (VIEWMODEL.md).
type ViewModel struct {
	Package  string
	Revision Revision
	data     []byte
}

// JSON returns exactly the bytes emit view would write for the package (rule R9).
func (vm *ViewModel) JSON() []byte {
	return vm.data
}

// Decode unmarshals the view model into v, typically a *vm.ViewModel (rule R10), strictly: a
// member fills only the field of exactly its name, and null only an unconstrained member
// (json.RawMessage or an interface); a member no field reads is skipped (VIEWMODEL.md J6).
func (vm *ViewModel) Decode(v any) error {
	return strictDecode(vm.Package, vm.data, v)
}

// ViewModel returns pkg's view model, errors or not (R9); a non-name or no package: ErrUnknownPackage.
func (p *Project) ViewModel(ctx context.Context, pkg string) (m *ViewModel, err error) {
	defer recoverInternal(&err)
	return p.viewModel(ctx, pkg)
}

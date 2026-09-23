package canon

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// TypeInfo describes the type of a value: its canonical text and its view model encoding.
type TypeInfo struct {
	Expr string
	VM   json.RawMessage
}

// Origin records where a value comes from (API.md §5.2).
type Origin struct {
	Kind OriginKind
	Span
	Pointer string
	Layer   string
	Via     *Origin
	Stack   []Frame
}

// Editability says whether and where a value can be edited (API.md §5.2).
type Editability struct {
	Mode   EditMode
	Reason Reason
	File   string
	Origin string
	Layer  string
}

// Value is a value of a snapshot, with its type and origin (API.md §5.2).
type Value struct {
	Path     string
	Kind     ValueKind
	Type     TypeInfo
	Text     string
	Origin   Origin
	Editable Editability
}

// Value returns the final value at path (rules R4-R6).
func (p *Project) Value(ctx context.Context, path string) (*Value, error) {
	return nil, errUnimplemented()
}

// String returns the canonical Canon text of the value.
func (v *Value) String() string {
	return v.Text
}

// Bool returns the value and true if the value is a Bool.
func (v *Value) Bool() (bool, bool) {
	panic(msgUnimplemented)
}

// Int returns the value and true if the value is of an integer type.
func (v *Value) Int() (int64, bool) {
	panic(msgUnimplemented)
}

// Float returns the value and true if the value is a Float or Float32.
func (v *Value) Float() (float64, bool) {
	panic(msgUnimplemented)
}

// Str returns the value and true if the value is a String or an asset.
func (v *Value) Str() (string, bool) {
	panic(msgUnimplemented)
}

// Dur returns the value and true if the value is a Duration.
func (v *Value) Dur() (time.Duration, bool) {
	panic(msgUnimplemented)
}

// Member returns the enum's qualified name and the member's Canon name.
func (v *Value) Member() (enum, name string, ok bool) {
	panic(msgUnimplemented)
}

// Case returns the current case of a variant.
func (v *Value) Case() (string, bool) {
	panic(msgUnimplemented)
}

// Key returns the key of a ref, table entry or keyed-list element, in canonical text.
func (v *Value) Key() (string, bool) {
	panic(msgUnimplemented)
}

// IsNone reports whether the value is none.
func (v *Value) IsNone() bool {
	return v.Kind == KindNone
}

// Len returns the number of elements, of fields of a record or a variant's current case, or 0.
func (v *Value) Len() int {
	panic(msgUnimplemented)
}

// Children returns fields in declaration order, elements, or map entries in insertion order.
func (v *Value) Children() []*Value {
	panic(msgUnimplemented)
}

// Child returns one child by a path segment: ".f", "[k]" or "[#n]".
func (v *Value) Child(seg string) (*Value, error) {
	return nil, errUnimplemented()
}

// JSON returns the wire form of the value (WIRE.md).
func (v *Value) JSON() []byte {
	panic(msgUnimplemented)
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

// Decode unmarshals the view model into v, typically a *vm.ViewModel (rule R10).
func (vm *ViewModel) Decode(v any) error {
	if err := json.Unmarshal(vm.data, v); err != nil {
		return fmt.Errorf(fmtDecode, vm.Package, err)
	}
	return nil
}

// ViewModel returns the view model of pkg, even when the package has errors (rule R9).
func (p *Project) ViewModel(ctx context.Context, pkg string) (*ViewModel, error) {
	return nil, errUnimplemented()
}

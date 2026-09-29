package canon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/fantasim/canonlang/api/vm"
)

// strictDecode is ViewModel.Decode: data checked against v's type, then read by encoding/json (ADR-0006).
func strictDecode(pkg string, data []byte, v any) error {
	if t := reflect.TypeOf(v); t != nil && t.Kind() == reflect.Pointer {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		var doc any
		if err := dec.Decode(&doc); err != nil {
			return fmt.Errorf(fmtDecode, pkg, err)
		}
		if err := strictValue(doc, t, ""); err != nil {
			return fmt.Errorf(fmtDecode, pkg, err)
		}
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf(fmtDecode, pkg, err)
	}
	return nil
}

var (
	rawType         = reflect.TypeFor[json.RawMessage]()
	unmarshalerType = reflect.TypeFor[json.Unmarshaler]()
	// ownMembers are the object members a vm type that reads its own JSON accepts.
	ownMembers = map[reflect.Type][]string{reflect.TypeFor[vm.TextRef](): {textRefMember}}
)

// strictValue checks the JSON value x, at the RFC 6901 pointer ptr, against the Go type t.
func strictValue(x any, t reflect.Type, ptr string) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch {
	case t == rawType || t.Kind() == reflect.Interface:
		return nil
	case x == nil:
		return fmt.Errorf(fmtAtPointer, errDecodeNull, ptr)
	}
	if names, own := ownMembers[t]; own {
		return strictOwn(x, names, ptr)
	}
	if reflect.PointerTo(t).Implements(unmarshalerType) {
		return nil // vm.Number, vm.Scalar: their UnmarshalJSON refuses what they do not read
	}
	switch y := x.(type) {
	case map[string]any:
		return strictObject(y, t, ptr)
	case []any:
		if t.Kind() != reflect.Slice && t.Kind() != reflect.Array {
			return nil
		}
		for i, e := range y {
			if err := strictValue(e, t.Elem(), fmt.Sprintf(fmtIndexPointer, ptr, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// strictObject checks an object's members, in byte order, against a struct's fields or a map's values.
func strictObject(obj map[string]any, t reflect.Type, ptr string) error {
	if t.Kind() != reflect.Struct && t.Kind() != reflect.Map {
		return nil
	}
	for _, name := range slices.Sorted(maps.Keys(obj)) {
		at := ptr + pointerSep + pointerEscaper.Replace(name)
		elem, err := memberType(t, name, at)
		if err != nil {
			return err
		}
		if elem == nil {
			continue
		}
		if err := strictValue(obj[name], elem, at); err != nil {
			return err
		}
	}
	return nil
}

// memberType is the type member name of t reads into: a map's values, or the field that has
// exactly that name; nil for a member no field reads, which encoding/json skips and a reader
// ignores (VIEWMODEL.md J6). A name that only a case fold matches is refused.
func memberType(t reflect.Type, name, at string) (reflect.Type, error) {
	if t.Kind() == reflect.Map {
		return t.Elem(), nil
	}
	fields := jsonFields(t)
	if ft, ok := fields[name]; ok {
		return ft, nil
	}
	return nil, foldMatch(slices.Sorted(maps.Keys(fields)), name, at)
}

// foldMatch refuses name when one of names, in their order, matches it only by a case fold.
func foldMatch(names []string, name, at string) error {
	for _, n := range names {
		if n != name && strings.EqualFold(n, name) {
			return fmt.Errorf(fmtMemberCase, errDecodeCase, at, n)
		}
	}
	return nil
}

// strictOwn checks the member names of an object a vm type reads itself.
func strictOwn(x any, names []string, ptr string) error {
	obj, ok := x.(map[string]any)
	if !ok {
		return nil
	}
	for _, name := range slices.Sorted(maps.Keys(obj)) {
		if err := foldMatch(names, name, ptr+pointerSep+pointerEscaper.Replace(name)); err != nil {
			return err
		}
	}
	return nil
}

// fieldCache holds each struct type's JSON member names and field types, read once per type.
var fieldCache sync.Map // reflect.Type -> map[string]reflect.Type

// jsonFields is the members encoding/json reads into t, as it reads them: each field by its tag's
// name, else its Go name; an embedded struct without a tag name flattened; the shallower of two
// fields of one name kept.
func jsonFields(t reflect.Type) map[string]reflect.Type {
	if v, ok := fieldCache.Load(t); ok {
		return v.(map[string]reflect.Type)
	}
	out, depth := map[string]reflect.Type{}, map[string]int{}
	for _, f := range reflect.VisibleFields(t) {
		name, ok := jsonName(f)
		if !ok || hidden(t, f.Index) {
			continue
		}
		if d, seen := depth[name]; !seen || len(f.Index) < d {
			out[name], depth[name] = f.Type, len(f.Index)
		}
	}
	v, _ := fieldCache.LoadOrStore(t, out)
	return v.(map[string]reflect.Type)
}

// jsonName is the member f reads by itself; false for a field tagged `-`, an unexported field
// other than an embedded struct, and an embedded struct flattened into its parent.
func jsonName(f reflect.StructField) (string, bool) {
	tag := f.Tag.Get(structTagJSON)
	name, _, _ := strings.Cut(tag, tagSep)
	if tag == tagSkip {
		return "", false
	}
	if f.Anonymous && embeddedStruct(f) {
		return name, name != ""
	}
	if !f.IsExported() {
		return "", false
	}
	if name == "" {
		name = f.Name
	}
	return name, true
}

// hidden reports a field promoted through an embedded field encoding/json does not flatten: one
// tagged `-`, or with a tag name.
func hidden(t reflect.Type, index []int) bool {
	for k := 1; k < len(index); k++ {
		tag := t.FieldByIndex(index[:k]).Tag.Get(structTagJSON)
		if name, _, _ := strings.Cut(tag, tagSep); tag == tagSkip || name != "" {
			return true
		}
	}
	return false
}

// embeddedStruct reports an embedded struct or pointer to one.
func embeddedStruct(f reflect.StructField) bool {
	t := f.Type
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct
}

package viewgen

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/jsonsrc"
)

// buildValue builds the tree of rv, a field, map value or slice element of the api/vm structs
// (VIEWMODEL.md J1-J3, J10). A present pointer is built as the value it points to.
func buildValue(rv reflect.Value) (*jsonsrc.Node, error) {
	if !rv.IsValid() {
		return nil, fmt.Errorf("%w: invalid value", errType)
	}
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if n, ok, err := buildScalar(rv); ok || err != nil {
		return n, err
	}
	switch rv.Kind() {
	case reflect.Struct:
		return buildStruct(rv)
	case reflect.Map:
		return buildMap(rv)
	case reflect.Slice:
		return buildSlice(rv)
	default:
		return nil, fmt.Errorf("%w: %s", errType, rv.Type())
	}
}

// buildScalar builds a leaf: the view model's own types (vm.Number, vm.Scalar, vm.TextRef,
// json.RawMessage, api/vm/doc.go), or a bool, string or integer Go kind.
func buildScalar(rv reflect.Value) (*jsonsrc.Node, bool, error) {
	switch v := rv.Interface().(type) {
	case vm.Number:
		n, err := buildNumber(v)
		return n, true, err
	case vm.Scalar:
		n, err := buildScalarValue(v)
		return n, true, err
	case vm.TextRef:
		n, err := buildTextRef(v)
		return n, true, err
	case json.RawMessage:
		n, err := buildRaw(v)
		return n, true, err
	case bool:
		return &jsonsrc.Node{Kind: jsonsrc.Bool, Text: boolText(v)}, true, nil
	}
	switch rv.Kind() {
	case reflect.String:
		return &jsonsrc.Node{Kind: jsonsrc.String, Text: rv.String()}, true, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &jsonsrc.Node{Kind: jsonsrc.Number, Text: strconv.FormatInt(rv.Int(), decimalBase)}, true, nil
	default:
		return nil, false, nil
	}
}

func boolText(v bool) string {
	if v {
		return litTrue
	}
	return litFalse
}

// fieldInfo is one vm struct field's `json` tag, cached per type (structFields).
type fieldInfo struct {
	name  string
	omit  bool
	index int
}

// fieldCache holds each vm struct type's fields in declaration order (J2), read once: a large
// document repeats a type (a search index's rows) many times over.
var fieldCache sync.Map // reflect.Type -> []fieldInfo

// structFields is t's exported, named json fields, computed once per type.
func structFields(t reflect.Type) []fieldInfo {
	if v, ok := fieldCache.Load(t); ok {
		return v.([]fieldInfo)
	}
	fields := make([]fieldInfo, 0, t.NumField())
	for i := range t.NumField() {
		name, omit, ok := jsonTag(t.Field(i))
		if ok {
			fields = append(fields, fieldInfo{name: name, omit: omit, index: i})
		}
	}
	v, _ := fieldCache.LoadOrStore(t, fields)
	return v.([]fieldInfo)
}

// buildStruct builds a vm struct's members in its declared field order (J2), a `,omitzero`
// member left out when its value is the Go zero value (J3, api/vm/doc.go).
func buildStruct(rv reflect.Value) (*jsonsrc.Node, error) {
	fields := structFields(rv.Type())
	members := make([]jsonsrc.Member, 0, len(fields))
	for _, f := range fields {
		fv := rv.Field(f.index)
		if f.omit && fv.IsZero() {
			continue
		}
		val, err := buildValue(fv)
		if err != nil {
			return nil, err
		}
		members = append(members, jsonsrc.Member{Key: f.name, Value: val})
	}
	return &jsonsrc.Node{Kind: jsonsrc.Object, Members: members}, nil
}

// mapEntry is one map entry read from rv.MapRange, kept together for the byte-order sort.
type mapEntry struct {
	key string
	val reflect.Value
}

// buildMap builds a map's entries sorted by the byte order of its string keys (J2): every vm
// map is keyed by a name the program chose (types, values, languages, field keys, …).
func buildMap(rv reflect.Value) (*jsonsrc.Node, error) {
	if rv.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("%w: %s", errMapKey, rv.Type())
	}
	entries := make([]mapEntry, 0, rv.Len())
	for iter := rv.MapRange(); iter.Next(); {
		entries = append(entries, mapEntry{key: iter.Key().String(), val: iter.Value()})
	}
	slices.SortFunc(entries, func(a, b mapEntry) int { return strings.Compare(a.key, b.key) })
	members := make([]jsonsrc.Member, len(entries))
	for i, e := range entries {
		val, err := buildValue(e.val)
		if err != nil {
			return nil, err
		}
		members[i] = jsonsrc.Member{Key: e.key, Value: val}
	}
	return &jsonsrc.Node{Kind: jsonsrc.Object, Members: members}, nil
}

// buildSlice builds a slice's elements in order (never sorted: a slice's order is already the
// program's, J2's "every other object" case for the arrays this schema uses).
func buildSlice(rv reflect.Value) (*jsonsrc.Node, error) {
	elems := make([]*jsonsrc.Node, rv.Len())
	for i := range elems {
		v, err := buildValue(rv.Index(i))
		if err != nil {
			return nil, err
		}
		elems[i] = v
	}
	return &jsonsrc.Node{Kind: jsonsrc.Array, Elems: elems}, nil
}

// jsonTag reads a vm struct field's `json` tag: its member name, whether it is `,omitzero`,
// and whether it names a member at all (a bare `-` skips the field, as encoding/json does).
func jsonTag(f reflect.StructField) (name string, omitzero, ok bool) {
	tag, present := f.Tag.Lookup(tagJSON)
	if !present || tag == "" || tag == tagSkip {
		return "", false, false
	}
	parts := strings.Split(tag, tagSep)
	for _, p := range parts[1:] {
		if p == tagOmitzero {
			omitzero = true
		}
	}
	return parts[0], omitzero, parts[0] != ""
}

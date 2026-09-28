package vm_test

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/api/vm"
)

const specSchema = "../../spec/viewmodel.schema.json"

// jnode is a JSON value read in document order, independently of the generator's reader.
type jnode struct {
	object bool
	keys   []string
	vals   []*jnode // object member values, or array elements
	value  any      // a scalar: string, json.Number, bool or nil
}

func readJSON(t *testing.T, dec *json.Decoder) *jnode {
	t.Helper()
	tok, err := dec.Token()
	if err != nil {
		t.Fatal(err)
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return &jnode{value: tok}
	}
	n := &jnode{object: delim == '{'}
	for dec.More() {
		if n.object {
			key, err := dec.Token()
			if err != nil {
				t.Fatal(err)
			}
			n.keys = append(n.keys, key.(string))
		}
		n.vals = append(n.vals, readJSON(t, dec))
	}
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	return n
}

// at is member key of an object node, nil when absent.
func (n *jnode) at(key string) *jnode {
	if n == nil || !n.object {
		return nil
	}
	if i := slices.Index(n.keys, key); i >= 0 {
		return n.vals[i]
	}
	return nil
}

// str is the string value of member key, "" when absent.
func (n *jnode) str(key string) string {
	s, _ := n.at(key).scalar().(string)
	return s
}

func (n *jnode) scalar() any {
	if n == nil {
		return nil
	}
	return n.value
}

// strs is the string elements of array member key.
func (n *jnode) strs(key string) []string {
	var out []string
	if a := n.at(key); a != nil {
		for _, v := range a.vals {
			s, _ := v.value.(string)
			out = append(out, s)
		}
	}
	return out
}

var (
	typeNumber  = reflect.TypeFor[vm.Number]()
	typeScalar  = reflect.TypeFor[vm.Scalar]()
	typeTextRef = reflect.TypeFor[vm.TextRef]()
	typeRaw     = reflect.TypeFor[json.RawMessage]()
)

type visit struct {
	n   *jnode
	typ reflect.Type
}

// walker walks the schema and the api/vm types side by side.
type walker struct {
	t       *testing.T
	defs    *jnode
	reached map[string]bool
	seen    map[visit]bool
	structs []reflect.Type            // in first-met order
	objects map[reflect.Type][]*jnode // the object schemas each struct stands for
}

// API.md R10, VIEWMODEL.md V2, J2, J3, J9, J10: members and fields match one to one, in order;
// omitzero and pointers follow J3; arrays are slices, maps maps; Number, Scalar, TextRef and
// json.RawMessage stand for J10/J9 nodes; const and enum members have their values' Go kind.
func TestStructsMatchSchema(t *testing.T) {
	data, err := os.ReadFile(specSchema)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	root := readJSON(t, dec)
	w := &walker{t: t, defs: root.at("$defs"), reached: map[string]bool{}, seen: map[visit]bool{}, objects: map[reflect.Type][]*jnode{}}
	w.walk(root, reflect.TypeFor[vm.ViewModel](), "ViewModel")
	for _, name := range w.defs.keys {
		if !w.reached[name] {
			t.Errorf("$defs/%s is not reached from the root", name)
		}
	}
	for _, typ := range w.structs {
		w.checkStruct(typ, w.objects[typ])
	}
}

func (w *walker) walk(n *jnode, typ reflect.Type, at string) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if n == nil {
		w.t.Errorf("%s: %s has no schema", at, typ)
		return
	}
	if ref := n.str("$ref"); ref != "" {
		name := strings.TrimPrefix(ref, "#/$defs/")
		w.reached[name] = true
		if typ == typeTextRef && name != "textRef" {
			w.t.Errorf("%s: TextRef stands for $defs/%s", at, name)
		}
		if typ != typeTextRef {
			w.walk(w.defs.at(name), typ, at)
		}
		return
	}
	if w.special(n, typ, at) || w.seen[visit{n, typ}] {
		return
	}
	w.seen[visit{n, typ}] = true
	w.shape(n, typ, at)
}

// special checks a hand-written type against its node (J9, J10); false for any other type.
func (w *walker) special(n *jnode, typ reflect.Type, at string) bool {
	checks := []struct {
		typ reflect.Type
		ok  func(*jnode) bool
	}{
		{typeTextRef, func(*jnode) bool { return false }}, // only through $ref textRef
		{typeNumber, numberish},
		{typeScalar, stringOrInteger},
		{typeRaw, unconstrained},
	}
	for _, c := range checks {
		if typ == c.typ {
			if !c.ok(n) {
				w.t.Errorf("%s: %s does not stand for this schema node", at, typ)
			}
			return true
		}
	}
	return false
}

// shape checks a node that is not a hand-written type: an object, a union, a const or enum,
// an array, a map or a scalar.
func (w *walker) shape(n *jnode, typ reflect.Type, at string) {
	switch {
	case n.at("properties") != nil:
		w.object(n, typ, at)
	case n.at("oneOf") != nil:
		for _, b := range n.at("oneOf").vals {
			w.walk(b, typ, at)
		}
	case n.at("const") != nil:
		w.kind(n.at("const").value, typ, at)
	case n.at("enum") != nil:
		for _, v := range n.at("enum").vals {
			w.kind(v.value, typ, at)
		}
	case n.str("type") == "array":
		w.container(n.at("items"), typ, reflect.Slice, at)
	case n.str("type") == "object":
		w.container(n.at("additionalProperties"), typ, reflect.Map, at)
	default:
		v, ok := jsonTypes[n.str("type")]
		if !ok {
			w.t.Errorf("%s: %s for a schema of type %q (a number is vm.Number, no type json.RawMessage)", at, typ, n.str("type"))
			return
		}
		w.kind(v, typ, at)
	}
}

// jsonTypes stands a scalar "type" for a Go value of its kind (number has none: it is Number).
var jsonTypes = map[string]any{"string": "", "integer": json.Number("0"), "boolean": false}

func (w *walker) container(elem *jnode, typ reflect.Type, kind reflect.Kind, at string) {
	if typ.Kind() != kind || elem == nil || !elem.object {
		w.t.Errorf("%s: %s for a schema of %s without an element schema", at, typ, kind)
		return
	}
	w.walk(elem, typ.Elem(), at)
}

// kind checks that typ holds a JSON scalar like v: a string, an integer or a boolean.
func (w *walker) kind(v any, typ reflect.Type, at string) {
	want := reflect.Invalid
	switch x := v.(type) {
	case string:
		want = reflect.String
	case bool:
		want = reflect.Bool
	case json.Number:
		if _, err := x.Int64(); err == nil {
			want = reflect.Int
		}
	}
	if want == reflect.Invalid || typ.Kind() != want {
		w.t.Errorf("%s: %s holds a JSON %v", at, typ, v)
	}
}

func (w *walker) object(n *jnode, typ reflect.Type, at string) {
	if typ.Kind() != reflect.Struct {
		w.t.Errorf("%s: an object schema maps to %s", at, typ)
		return
	}
	if _, ok := w.objects[typ]; !ok {
		w.structs = append(w.structs, typ)
	}
	w.objects[typ] = append(w.objects[typ], n)
	props := n.at("properties")
	for i, name := range props.keys {
		f, ok := fieldByTag(typ, name)
		if !ok {
			w.t.Errorf("%s has no field for member %q", typ, name)
			continue
		}
		w.walk(props.vals[i], f.Type, typ.Name()+"."+name)
	}
}

func fieldByTag(typ reflect.Type, name string) (reflect.StructField, bool) {
	for i := range typ.NumField() {
		if f := typ.Field(i); tagName(f) == name {
			return f, true
		}
	}
	return reflect.StructField{}, false
}

func tagName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	return name
}

// checkStruct holds a struct to the objects it stands for (J2, J3).
func (w *walker) checkStruct(typ reflect.Type, objects []*jnode) {
	var order []string
	for i := range typ.NumField() {
		order = append(order, tagName(typ.Field(i)))
	}
	for _, name := range order {
		f, _ := fieldByTag(typ, name)
		always, declared, zero := true, false, false
		for _, o := range objects {
			p := o.at("properties").at(name)
			declared = declared || p != nil
			always = always && p != nil && slices.Contains(o.strs("required"), name)
			zero = zero || p != nil && w.zeroAdmitted(p)
		}
		if !declared {
			w.t.Errorf("%s.%s is no member of the schema", typ, f.Name)
		}
		if omit := strings.Contains(f.Tag.Get("json"), ",omitzero"); omit == always {
			w.t.Errorf("%s.%s: omitzero %v, but always present %v (J3)", typ, f.Name, omit, always)
		}
		if got, want := f.Type.Kind() == reflect.Pointer, wantPointer(f.Type, always, zero); got != want {
			w.t.Errorf("%s.%s: pointer %v, want %v (J3)", typ, f.Name, got, want)
		}
	}
	for _, o := range objects {
		keys := o.at("properties").keys
		got := slices.DeleteFunc(slices.Clone(order), func(s string) bool { return !slices.Contains(keys, s) })
		if !slices.Equal(got, keys) {
			w.t.Errorf("%s orders %v, the schema %v (J2)", typ, got, keys)
		}
	}
}

// wantPointer: an optional struct, or an optional scalar whose zero value is also a value.
func wantPointer(t reflect.Type, always, zero bool) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch {
	case always || t == typeNumber || t == typeScalar || t == typeTextRef:
		return false
	case t.Kind() == reflect.Struct:
		return true
	}
	return zero && slices.Contains([]reflect.Kind{reflect.String, reflect.Int, reflect.Bool}, t.Kind())
}

// zeroAdmitted reports whether "", 0 or false satisfies the node.
func (w *walker) zeroAdmitted(n *jnode) bool {
	if ref := n.str("$ref"); ref != "" {
		return w.zeroAdmitted(w.defs.at(strings.TrimPrefix(ref, "#/$defs/")))
	}
	isZero := func(v any) bool { return v == "" || v == false || v == json.Number("0") }
	switch {
	case n.at("const") != nil:
		return isZero(n.at("const").value)
	case n.at("enum") != nil:
		return slices.ContainsFunc(n.at("enum").vals, func(v *jnode) bool { return isZero(v.value) })
	case n.at("oneOf") != nil:
		return slices.ContainsFunc(n.at("oneOf").vals, w.zeroAdmitted)
	case n.str("type") == "string":
		return bound(n, "minLength") <= 0 && (n.at("pattern") == nil || regexp.MustCompile(n.str("pattern")).MatchString(""))
	case n.str("type") == "integer":
		return bound(n, "minimum") <= 0 && (n.at("maximum") == nil || bound(n, "maximum") >= 0)
	}
	return n.str("type") == "boolean"
}

func bound(n *jnode, key string) float64 {
	num, _ := n.at(key).scalar().(json.Number)
	v, _ := strconv.ParseFloat(num.String(), 64)
	return v
}

// numberish is a JSON number, an integer, or a oneOf of those and J10's decimal string.
func numberish(n *jnode) bool {
	if t := n.str("type"); t == "number" || t == "integer" {
		return true
	}
	branches := n.at("oneOf")
	if branches == nil {
		return false
	}
	numeric := false
	for _, b := range branches.vals {
		decimal := b.str("type") == "string" && b.str("pattern") == "^-?[0-9]+$"
		if !decimal && !numberish(b) {
			return false
		}
		numeric = numeric || !decimal
	}
	return numeric
}

// stringOrInteger is a oneOf of exactly a string and an integer.
func stringOrInteger(n *jnode) bool {
	b := n.at("oneOf")
	if b == nil || len(b.vals) != 2 {
		return false
	}
	types := []string{b.vals[0].str("type"), b.vals[1].str("type")}
	slices.Sort(types)
	return slices.Equal(types, []string{"integer", "string"})
}

// unconstrained is a node that admits any JSON value (J10's value encoding).
func unconstrained(n *jnode) bool {
	for _, k := range []string{"$ref", "type", "const", "enum", "oneOf", "properties", "items", "additionalProperties"} {
		if n.at(k) != nil {
			return false
		}
	}
	return true
}

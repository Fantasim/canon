package views

import (
	"reflect"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/api/vm"
)

// refs are what a model refers to outside itself: the packages of its qualified names, value
// ids and text keys, and the studio's units and widgets its controls name.
type refs struct {
	pkgs    map[string]bool
	studio  bool
	units   map[string]bool
	widgets map[string]bool
	roots   map[string]bool // asset roots (12.9 `assets`)
}

// collect walks m for what it refers to (VIEWMODEL.md 12.1 `requires`).
func collect(m *vm.ViewModel) *refs {
	r := &refs{pkgs: map[string]bool{}, units: map[string]bool{}, widgets: map[string]bool{}, roots: map[string]bool{}}
	r.walk(reflect.ValueOf(*m), "")
	return r
}

// walkers visit a value of the model by its kind; the other kinds name nothing.
var walkers map[reflect.Kind]func(*refs, reflect.Value, string)

func init() {
	walkers = map[reflect.Kind]func(*refs, reflect.Value, string){
		reflect.Pointer: (*refs).elem, reflect.Struct: func(r *refs, v reflect.Value, _ string) { r.object(v) },
		reflect.Slice: (*refs).items, reflect.Map: (*refs).members,
		reflect.String: func(r *refs, v reflect.Value, name string) { r.name(v.String(), name) },
	}
}

// walk visits v, a member named name.
func (r *refs) walk(v reflect.Value, name string) {
	if fn, ok := walkers[v.Kind()]; ok {
		fn(r, v, name)
	}
}

func (r *refs) elem(v reflect.Value, name string) {
	if !v.IsNil() {
		r.walk(v.Elem(), name)
	}
}

// items visits a slice's elements; a J10 value (raw JSON) names nothing.
func (r *refs) items(v reflect.Value, name string) {
	if v.Type().Elem().Kind() == reflect.Uint8 {
		return
	}
	for i := range v.Len() {
		r.walk(v.Index(i), name)
	}
}

// object visits a struct's members, a text reference by its key's package; a wire mapping names
// nothing (its `unit` is a Duration's wire unit).
func (r *refs) object(v reflect.Value) {
	switch x := v.Interface().(type) {
	case vm.TextRef:
		if pkg, _, found := strings.Cut(x.Key, colon); found {
			r.pkgs[pkg] = true
		}
		return
	case vm.Wire:
		return
	case vm.TypeExpr:
		if x.Kind == assetKind {
			r.roots[x.Root] = true
		}
	}
	for i := range v.NumField() {
		tag, _, _ := strings.Cut(v.Type().Field(i).Tag.Get(jsonTag), jsonSep)
		if !slices.Contains(skippedMembers, tag) {
			r.walk(v.Field(i), tag)
		}
	}
}

// members visits a map's values; the keys of `drivers` are value ids (J14).
func (r *refs) members(v reflect.Value, name string) {
	iter := v.MapRange()
	for iter.Next() {
		if name == driversMember {
			r.name(iter.Key().String(), valueIDMembers[0])
		}
		r.walk(iter.Value(), "")
	}
}

// name records the package a string names, by the member holding it.
func (r *refs) name(s, member string) {
	switch {
	case s == "":
	case slices.Contains(qualifiedMembers, member):
		if i := strings.LastIndex(s, dot); i > 0 {
			r.pkgs[s[:i]] = true
		}
	case slices.Contains(valueIDMembers, member):
		if pkg, _, found := strings.Cut(s, colon); found {
			r.pkgs[pkg] = true
		}
	case slices.Contains(studioMembers, member):
		r.studio = true
	case member == unitMember:
		r.studio, r.units[s] = true, true
	case member == widgetMember:
		r.studio, r.widgets[s] = true, true
	}
}

// requires is the `requires` section (12.1): the packages the model refers to, but itself,
// byte order; the studio's when it names a unit, widget, menu, icon or tone.
func (b *builder) requires() {
	r := collect(b.m)
	if r.studio && b.in.Studio != "" {
		r.pkgs[b.in.Studio] = true
	}
	delete(r.pkgs, b.pkg.Path)
	//canon:unordered the names are sorted below
	for pkg := range r.pkgs {
		b.m.Requires = append(b.m.Requires, pkg)
	}
	slices.Sort(b.m.Requires)
}

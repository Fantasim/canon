package encode

import (
	"strings"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/types"
)

// Name is the qualified name of a record, variant or enum (VIEWMODEL.md J7); an applied record
// is its record's, a case its variant's.
func Name(t types.Type) string {
	switch x := t.Base().(type) {
	case *types.AppliedRecord:
		return x.Rec.String()
	case *types.CaseType:
		return x.Variant.String()
	}
	return t.Base().String()
}

// FuncName is the qualified name of a type function (J7).
func FuncName(fn *types.TypeFunc) string {
	if fn.Pkg == "" {
		return fn.Name
	}
	return fn.Pkg + dot + fn.Name
}

// CollectionID is the value id of a collection a ref can name (J8, J12): `"<package>:<let>"`,
// followed by the field path through its records; "" for a field of an enclosing record.
func CollectionID(c *types.Collection) string {
	if c.Kind == types.CollField {
		return ""
	}
	return strings.Join(append([]string{c.Pkg + colon + c.Name}, c.FieldPath...), dot)
}

// Element is a collection's element as a ref names it: its qualified name, or `$define` (J12).
func Element(c *types.Collection) string {
	if c.Kind == types.CollDefines {
		return elemDefine
	}
	return Name(c.Elem)
}

// KeyType is `int` for a keyed list keyed by an integer field, else `string` (J12).
func KeyType(c *types.Collection) string {
	if c.KeyedBy != nil && c.KeyedBy.Type.Base().Kind() == types.Int {
		return keyInt
	}
	return keyString
}

// Driver is where a type argument is read (J13): an earlier field, a parameter of the enclosing
// record or the key of the enclosing dependent map, with the fields read down it; nil for none.
func Driver(a *types.Arg) *vm.Driver {
	if a == nil {
		return nil
	}
	d, path := &vm.Driver{Key: true}, a.Path
	switch {
	case a.Source == types.ArgParam && a.Param != nil:
		d = &vm.Driver{Param: a.Param.Name}
	case a.Source == types.ArgField && len(a.Path) > 0:
		d, path = &vm.Driver{Field: a.Path[0].Name}, a.Path[1:]
	}
	for _, f := range path {
		d.Path = append(d.Path, f.Name)
	}
	return d
}

// Sibling is the per-instance target (J12, TYPES.md 10.2 level 1) of a ref declared in d: the
// records from d up to the owner, and its field; nil when no field path leads from the owner to
// d. perInstance is false for another collection.
func Sibling(d types.Type, c *types.Collection) (s *vm.Sibling, perInstance bool) {
	if c.Kind != types.CollField || c.Owner == nil || len(c.FieldPath) == 0 {
		return nil, false
	}
	up, found := depth(c.Owner, d)
	if !found {
		return nil, true
	}
	return &vm.Sibling{Up: up, Field: c.FieldPath[0]}, true
}

// depth is how many record or case levels below owner d first sits, 0 when d is owner; false
// when no field path leads to it.
func depth(owner *types.RecordType, d types.Type) (int, bool) {
	level := map[types.Type]int{owner: 0}
	queue := []types.Type{owner}
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		if t == d {
			return level[t], true
		}
		for _, h := range held(t) {
			if _, seen := level[h]; !seen {
				level[h] = level[t] + 1
				queue = append(queue, h)
			}
		}
	}
	return 0, false
}

// held are the records and cases the fields of the record or case t hold, directly or in
// collections, maps and optionals.
func held(t types.Type) []types.Type {
	var out []types.Type
	for _, f := range FieldsOf(t) {
		Walk(f.Type, func(x types.Type) bool {
			switch y := x.Base().(type) {
			case *types.RecordType:
				out = append(out, y)
				return false
			case *types.AppliedRecord:
				out = append(out, y.Rec)
				return false
			case *types.VariantType:
				for _, c := range y.Cases {
					out = append(out, c)
				}
				return false
			}
			return true
		})
	}
	return out
}

// FieldsOf are the fields of a record, applied record or case; nil for any other type.
func FieldsOf(t types.Type) []*types.Field {
	if t == nil {
		return nil
	}
	switch x := t.Base().(type) {
	case *types.RecordType:
		return x.Fields
	case *types.AppliedRecord:
		return x.Rec.Fields
	case *types.CaseType:
		return x.Fields
	}
	return nil
}

// Walk calls fn on t and, while fn returns true, on the types t holds: elements, keys, values,
// optional and union members, alias and refinement layers kept by Base.
func Walk(t types.Type, fn func(types.Type) bool) {
	if t == nil || !fn(t) {
		return
	}
	for _, c := range parts(t.Base()) {
		Walk(c, fn)
	}
}

// parts are the types a composite holds.
func parts(t types.Type) []types.Type {
	switch x := t.(type) {
	case *types.OptionalType:
		return []types.Type{x.Elem}
	case *types.ListType:
		return []types.Type{x.Elem}
	case *types.TableType:
		return []types.Type{x.Elem}
	case *types.MapType:
		return []types.Type{x.Key, x.Value}
	case *types.DepMapType:
		return []types.Type{x.Value}
	case *types.LitUnionType:
		return []types.Type{x.Of}
	}
	return nil
}

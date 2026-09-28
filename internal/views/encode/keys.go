package encode

import "github.com/fantasim/canonlang/internal/types"

// Keyed is a field of a layout and its field key (VIEWMODEL.md L17): its own, or an inline case's.
type Keyed struct {
	Key   string // "f", or "v.c.f" for a case field
	Field *types.Field
	Decl  types.Type // the record or case declaring it
	Case  string     // its case path "v=c/w=c2" (L18)
	Path  string     // its value path "v.w.f"
}

// Keys are t's fields in declaration order, each @json(inline) variant field followed by the
// fields of its cases in case order, nested inline variants depth first (L17, L18).
func Keys(t types.Type) []Keyed {
	var out []Keyed
	inline(&out, t, Keyed{}, map[*types.VariantType]bool{})
	return out
}

// inline adds the fields of decl below at, then its inline cases'; seen stops a self-inline.
func inline(out *[]Keyed, decl types.Type, at Keyed, seen map[*types.VariantType]bool) {
	for _, f := range FieldsOf(decl) {
		k := Keyed{Key: join(at.Key, f.Name), Field: f, Decl: decl, Case: at.Case}
		if at.Case != "" {
			k.Path = join(at.Path, f.Name)
		}
		*out = append(*out, k)
		v, ok := f.Type.Base().(*types.VariantType)
		if !ok || !f.Inline || seen[v] {
			continue
		}
		seen[v] = true
		for _, c := range v.Cases {
			inline(out, c, Keyed{Key: join(k.Key, c.Name), Case: joinWith(caseSep, at.Case, f.Name+caseIs+c.Name), Path: join(at.Path, f.Name)}, seen)
		}
		delete(seen, v)
	}
}

// Named are the keyed fields a view item's name places (L18): the own field of that name, else
// every case field of it, in case declaration order.
func Named(keys []Keyed, name string) []Keyed {
	var cases []Keyed
	for _, k := range keys {
		switch {
		case k.Field.Name != name:
		case k.Case == "":
			return []Keyed{k}
		default:
			cases = append(cases, k)
		}
	}
	return cases
}

func join(a, b string) string { return joinWith(dot, a, b) }

func joinWith(sep, a, b string) string {
	if a == "" {
		return b
	}
	return a + sep + b
}

package gogen

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// goType is what a getter of t returns, a ref as its key (CODEGEN.md §4.1, §4.2, §5.8).
func (g *gen) goType(t ir.TypeRef) string {
	switch t.Kind {
	case types.Bool:
		return goBool
	case types.Int:
		return g.intType(t)
	case types.Float:
		return g.sized(floatTypes, t.Bits)
	case types.String, types.LitUnion:
		return goString
	case types.Duration:
		return g.use(timePkg, timePkg) + durationType
	case types.Enum:
		return g.typeName(t.Named)
	case types.VariantKind:
		return g.qualify(typePkg(t.Named), g.kindType(t.Named))
	case types.Record, types.Variant, types.TypeApp:
		return pointer + g.typeName(t.Named)
	case types.Case:
		return pointer + g.caseTypeName(t)
	case types.List:
		return g.listType(t)
	case types.Map, types.DepMap:
		return g.rt() + mapType + lbracket + g.goType(g.sub(t.Key)) + listSep + g.goType(g.sub(t.Elem)) + rbracket
	case types.Ref:
		return g.keyType(t)
	default:
		g.refuseKind(t.Kind, typeRefused)
		return ""
	}
}

// kindText names a kind in a message: an IR defect never prints as a bare number.
func kindText(k types.Kind) string {
	if int(k) < len(kindNames) && kindNames[k] != "" {
		return kindNames[k]
	}
	return unknownKind
}

// failKind refuses an untranslatable kind of the item being written, the Subject (go.md §3).
func (g *gen) failKind(k types.Kind) {
	g.fail(newDetail(ErrUnsupported, g.at, kindFormat, g.at, kindText(k)))
}

// refuseKind is failKind, but ErrMalformed for a kind stage E already refuses at this position (E8019).
func (g *gen) refuseKind(k types.Kind, refused map[types.Kind]bool) {
	if refused[k] {
		g.fail(newDetail(ErrMalformed, g.at, kindFormat, g.at, kindText(k)))
		return
	}
	g.failKind(k)
}

// modeText names an emit mode in a message: an out-of-range mode never indexes modeNames (CODEGEN.md §2.1).
func modeText(m ir.Mode) string {
	if int(m) < len(modeNames) && modeNames[m] != "" {
		return modeNames[m]
	}
	return unknownMode
}

func (g *gen) intType(t ir.TypeRef) string {
	if t.Signed {
		return g.sized(signedTypes, t.Bits)
	}
	return g.sized(unsignedTypes, t.Bits)
}

// sized is the Go type of a width: the IR never holds a width outside the table (TYP-03).
func (g *gen) sized(names map[int]string, bits int) string {
	name, ok := names[bits]
	if !ok {
		g.failf(ErrMalformed, "a number of %d bits", bits)
	}
	return name
}

func (g *gen) listType(t ir.TypeRef) string {
	if t.KeyedBy == nil {
		return g.rt() + listType + lbracket + g.goType(g.sub(t.Elem)) + rbracket
	}
	key := g.keyField(t)
	return g.rt() + keyedListType + lbracket + g.goType(key.Type) + listSep + g.typeName(g.sub(t.Elem).Named) + rbracket
}

// keyField is the key field of a keyed list's element record.
func (g *gen) keyField(t ir.TypeRef) *ir.Field {
	rec, ok := g.sub(t.Elem).Named.(*ir.Record)
	if ok {
		for _, f := range rec.Fields {
			if f.Name == t.KeyedBy.Name {
				return f
			}
		}
	}
	g.failf(ErrMalformed, "keyed list without its key field %s", t.KeyedBy.Name)
	return &ir.Field{Type: ir.TypeRef{Kind: types.String}}
}

// sub is a composite type's element, key or value type, which the IR sets for its kind.
func (g *gen) sub(t *ir.TypeRef) ir.TypeRef {
	if t == nil {
		g.failf(ErrMalformed, "a composite type without its element")
		return ir.TypeRef{Kind: types.String}
	}
	return *t
}

// keyType is a ref's key: a table's id type, else the IR's key type, a define's name (§5.8).
func (g *gen) keyType(t ir.TypeRef) string {
	if isTableRef(t.Ref) {
		return g.qualify(t.Ref.Pkg, g.idType(t.Ref.Elem))
	}
	if t.Key == nil {
		g.failf(ErrMalformed, noKeyType)
		return ""
	}
	return g.goType(*t.Key)
}

// isTableRef reports a ref into a public table value: its key is that table's id enum (§5.3).
func isTableRef(r *ir.RefTarget) bool {
	return r != nil && r.Coll == types.CollLet && !r.Local && !r.Keyed
}

// typeName is the Go name of a named type, qualified when another package declares it.
func (g *gen) typeName(t ir.Type) string {
	return g.qualify(typePkg(t), g.goName(t))
}

// goName is the Go name of a named type in its own package (CODEGEN.md §3.3); anything else is an IR defect.
func (g *gen) goName(t ir.Type) string {
	name := g.names.TypeName(t)
	if name == "" {
		g.failf(ErrMalformed, "a named type %T", t)
	}
	return name
}

// idType is the id enum of a table of elem (CODEGEN.md §5.3), elem a named type.
func (g *gen) idType(elem ir.Type) string {
	if g.goName(elem) == "" {
		return ""
	}
	return g.names.IDTypeName(elem)
}

// kindType is the kind enum of variant v (CODEGEN.md §5.5).
func (g *gen) kindType(v ir.Type) string {
	if g.goName(v) == "" {
		return ""
	}
	return g.names.KindName(v)
}

// idMember is the id constant of key in a table of elem.
func (g *gen) idMember(elem ir.Type, key string) string {
	if g.goName(elem) == "" {
		return ""
	}
	return g.names.IDMemberName(elem, key)
}

func (g *gen) caseTypeName(t ir.TypeRef) string {
	v, ok := t.Named.(*ir.Variant)
	if !ok || t.Case == nil {
		g.failf(ErrMalformed, "a case type without its variant")
		return ""
	}
	if len(t.Case.Fields) == 0 {
		g.fail(newDetail(ErrMalformed, v.QName()+dot+t.Case.Name, // E8019 CaseField
			"the type %s.%s, a case without fields, which has no Go type", v.QName(), t.Case.Name))
	}
	return g.qualify(v.Pkg, g.names.CaseName(v, t.Case))
}

// qualify prefixes name with the import name of pkg, a Canon package other than this one.
func (g *gen) qualify(pkg, name string) string {
	if pkg == g.p.Name {
		return name
	}
	return g.importPkg(pkg) + dot + name
}

func typePkg(t ir.Type) string {
	switch t := t.(type) {
	case *ir.Record:
		return t.Pkg
	case *ir.Enum:
		return t.Pkg
	case *ir.Variant:
		return t.Pkg
	case *ir.Dependent:
		return t.Pkg
	}
	return ""
}

// importPkg imports another Canon package's go emit (CODEGEN.md §2.8).
func (g *gen) importPkg(pkg string) string {
	for _, ref := range g.p.Imports {
		if ref.Name != pkg {
			continue
		}
		for _, e := range ref.Emits {
			if e.Target == ir.TargetGo {
				return g.use(e.GoImport, e.GoPackage)
			}
		}
	}
	g.failf(ErrMalformed, "package %s is used but has no go emit", pkg)
	return pkg
}

func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

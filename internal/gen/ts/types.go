package tsgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// scalarTypes are the TypeScript types of the scalar kinds (CODEGEN.md §4.1).
var scalarTypes = map[types.Kind]string{
	types.Bool: tsBoolean, types.Float: tsNumber, types.String: tsString, types.Duration: tsNumber,
}

// tsType is the type a property of type t has (CODEGEN.md §4.1–§4.3); big is the field's @ts(bigint), which bigAt applies.
func (g *gen) tsType(t ir.TypeRef, big bool) string {
	if s, ok := scalarTypes[t.Kind]; ok {
		return s
	}
	big = bigAt(t, big)
	switch t.Kind {
	case types.Int:
		if big {
			return tsBigint
		}
		return tsNumber
	case types.Enum, types.Record, types.Variant, types.TypeApp:
		return g.named(t.Named)
	case types.Case:
		return g.caseType(t)
	case types.VariantKind:
		return g.kindType(t.Named)
	case types.LitUnion:
		return g.unionType(t)
	case types.Optional:
		return g.optionalType(t, big)
	case types.List, types.Table:
		return g.listType(t, big)
	case types.Map, types.DepMap:
		return g.mapType(t, big)
	case types.Ref:
		return g.keyType(t, big)
	default:
		g.failf(errUnsupported, unsupportedKind, kindText(t.Kind), g.at)
		return tsUnknown
	}
}

func (g *gen) optionalType(t ir.TypeRef, big bool) string {
	return g.tsType(g.elem(t), big) + unionSep + tsNull
}

// listType is ReadonlyArray of the element: a keyed list holds its records, a table field its rows, CanonRow with this package's id whoever owns the record (CODEGEN.md §3.3, §4.2; DECISIONS 323; log-2026-10-06 "U4 (gen/ts) done" (e)).
func (g *gen) listType(t ir.TypeRef, big bool) string {
	if rec, ok := rowRecord(t); ok {
		return fmt.Sprintf(readonlyArrayFormat, g.rowType(rec, g.idTypeOf(rec)))
	}
	return fmt.Sprintf(readonlyArrayFormat, g.tsType(g.elem(t), big))
}

func (g *gen) mapType(t ir.TypeRef, big bool) string {
	if t.Key == nil {
		g.failf(ErrMalformed, malformedNoKey, g.at)
		return tsUnknown
	}
	return fmt.Sprintf(readonlyMapFormat, g.keyType(*t.Key, big), g.tsType(g.elem(t), big))
}

// elem is a composite type's element, which the IR sets for its kind.
func (g *gen) elem(t ir.TypeRef) ir.TypeRef {
	if t.Elem == nil {
		g.failf(ErrMalformed, malformedNoElem, g.at)
		return ir.TypeRef{Kind: types.String}
	}
	return *t.Elem
}

// unionType is `A | "lit"` when A is an enum or String, else string (CODEGEN.md §4.1).
func (g *gen) unionType(t ir.TypeRef) string {
	arm := g.elem(t)
	if arm.Kind != types.Enum && arm.Kind != types.String {
		return tsString
	}
	parts := []string{g.tsType(arm, false)}
	for _, lit := range t.Literals {
		parts = append(parts, quote(lit))
	}
	return strings.Join(parts, unionSep)
}

// keyType is a ref's key: its table's id type, else the key type the IR gives (CODEGEN.md §5.8); a map key is written the same way.
func (g *gen) keyType(t ir.TypeRef, big bool) string {
	if t.Kind != types.Ref {
		return g.tsType(t, big)
	}
	if t.Key == nil {
		g.failf(ErrMalformed, malformedNoKey, g.at)
		return tsUnknown
	}
	if id := g.tableID(t.Ref); id != "" {
		return id
	}
	return g.tsType(*t.Key, bigAt(t, big))
}

// tableID is the id type of the table a ref targets, imported when another package's; "" when the table has none in its emit (CODEGEN.md §5.3).
func (g *gen) tableID(r *ir.RefTarget) string {
	if !isTableRef(r) || r.Elem == nil {
		return ""
	}
	if r.Pkg == g.p.Name {
		if g.tableOf(r.Elem) == nil {
			return ""
		}
		return idName(r.Elem)
	}
	if !g.foreignHasIDs(r.Pkg) {
		return ""
	}
	return g.importType(r.Pkg, idName(r.Elem))
}

// isTableRef reports a ref into a public table value: its key is that table's id type.
func isTableRef(r *ir.RefTarget) bool {
	return r != nil && r.Coll == types.CollLet && !r.Local && !r.Keyed
}

// named is a named type's name, imported when another package declares it.
func (g *gen) named(t ir.Type) string {
	name := typeName(t)
	if name == "" {
		g.failf(ErrMalformed, malformedDecl, g.at)
		return tsUnknown
	}
	if pkg := pkgOf(t); pkg != g.p.Name {
		return g.importType(pkg, name)
	}
	return name
}

// caseType is the type of a value narrowed to one case (CODEGEN.md §5.5); a case without fields has no type, so it is the literal object type.
func (g *gen) caseType(t ir.TypeRef) string {
	v, ok := t.Named.(*ir.Variant)
	if !ok || t.Case == nil {
		g.failf(ErrMalformed, malformedDecl, g.at)
		return tsUnknown
	}
	if !hasInterface(t.Case) {
		return fmt.Sprintf(bareCaseFormat, quote(t.Case.Wire))
	}
	name := caseName(v, t.Case)
	if v.Pkg != g.p.Name {
		return g.importType(v.Pkg, name)
	}
	return name
}

// kindType is a variant's kind union, TKind.
func (g *gen) kindType(t ir.Type) string {
	v, ok := t.(*ir.Variant)
	if !ok {
		g.failf(ErrMalformed, malformedDecl, g.at)
		return tsUnknown
	}
	if v.Pkg != g.p.Name {
		return g.importType(v.Pkg, kindName(v))
	}
	return kindName(v)
}

// kindText names a kind in a message.
func kindText(k types.Kind) string {
	if int(k) < len(kindNames) && kindNames[k] != "" {
		return kindNames[k]
	}
	return fmt.Sprintf(unknownKindFormat, k)
}

package tsgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// constants writes every public constant (CODEGEN.md §5.1): a scalar bare, an enum typed, a list or map typed and frozen.
func (g *gen) constants() {
	for _, c := range g.p.Consts {
		g.constant(c)
	}
}

func (g *gen) constant(c *ir.Const) {
	defer g.enter(c.Name)()
	name := g.declare(escape(effective(c.TS, c.Name)), c.Name)
	if c.V == nil {
		g.failf(ErrMalformed, malformedNoValue, c.Name, g.at)
		return
	}
	text := g.lit(c.Type, c.V, false)
	switch c.Type.Kind {
	case types.Bool, types.Int, types.Float, types.String, types.Duration:
		g.add(docComment("", c.Doc) + fmt.Sprintf(constFormat, name, text))
	case types.Enum, types.LitUnion, types.Ref:
		g.add(docComment("", c.Doc) + fmt.Sprintf(typedConstFormat, name, g.tsType(c.Type, false), text))
	default:
		g.add(docComment("", c.Doc) + g.frozenConst(name, c.Type, text))
	}
}

// frozenConst is a composite value frozen deeply; a map is already read-only.
func (g *gen) frozenConst(name string, t ir.TypeRef, text string) string {
	typ := g.tsType(t, false)
	if t.Kind == types.Map || t.Kind == types.DepMap {
		return fmt.Sprintf(typedConstFormat, name, typ, text)
	}
	return fmt.Sprintf(typedConstFormat, name, typ, fmt.Sprintf(freezeFormat, g.helper(canonFreezeName), typ, text))
}

// values writes the accessor of every emitted value, baked into the module (CODEGEN.md §5.9, §8.1).
func (g *gen) values() {
	for _, v := range g.emitted {
		g.value(v)
	}
}

func (g *gen) value(v *ir.Value) {
	defer g.enter(v.Name)()
	name := g.declare(escape(effective(v.TS, v.Name)), v.Name)
	if v.V == nil {
		g.failf(ErrMalformed, malformedNoValue, v.Name, g.at)
		return
	}
	doc := docComment("", v.Doc)
	switch {
	case v.Type.Kind == types.Table:
		g.add(doc + g.tableValue(name, v))
	case v.Type.Kind == types.List && v.Type.KeyedBy != nil:
		g.add(doc + g.keyedValue(name, v))
	default:
		g.add(doc + g.plainValue(name, v))
	}
}

// tableValue is a table: its rows in entry order, found by id (CODEGEN.md §8.3).
func (g *gen) tableValue(name string, v *ir.Value) string {
	elem := g.tableElem(v)
	rec, ok := elem.(*ir.Record)
	if !ok {
		g.failf(ErrMalformed, malformedTable, v.Name)
		return ""
	}
	tab := as[value.Table](g, v.V)
	if len(tab.Entries) != len(v.IDs) {
		g.failf(ErrMalformed, malformedTableIDs, v.Name, len(v.IDs), len(tab.Entries))
	}
	rows := make([]string, len(tab.Entries))
	for i, r := range tab.Entries {
		rows[i] = g.rowLit(rec, r)
	}
	return g.tableDecl(name, typeName(rec), idName(rec), rows, idProp)
}

// keyedValue is a keyed list: its rows in order, found by the key field.
func (g *gen) keyedValue(name string, v *ir.Value) string {
	rec := g.recordOf(g.elem(v.Type).Named)
	key := g.keyField(v.Type, rec)
	list := as[value.List](g, v.V)
	rows := make([]string, len(list.Elems))
	for i, x := range list.Elems {
		rows[i] = g.recordLit(rec, as[value.Record](g, x))
	}
	return g.tableDecl(name, g.named(rec), g.tsType(key.Type, key.BigInt), rows, fieldProp(key))
}

// tableDecl is `export const name: CanonTable<K, T> = canonTable(canonFreeze<ReadonlyArray<T>>([rows]), (e) => e.key);`.
func (g *gen) tableDecl(name, elem, keyType string, rows []string, keyProp string) string {
	g.helper(canonTableName)
	body := ""
	if len(rows) > 0 {
		body = newline + indent + strings.Join(rows, listEnd+indent) + listEnd
	}
	return fmt.Sprintf(tableFormat, name, keyType, elem, g.helper(canonFreezeName), body, keyProp)
}

// keyField is the key field of a keyed list's element record.
func (g *gen) keyField(t ir.TypeRef, rec *ir.Record) *ir.Field {
	if t.KeyedBy != nil {
		for _, f := range rec.Fields {
			if f.Name == t.KeyedBy.Name {
				return f
			}
		}
	}
	g.failf(ErrMalformed, malformedKeyField, g.at)
	return &ir.Field{Type: ir.TypeRef{Kind: types.String}}
}

// plainValue is a value that is no container: a record, a scalar, an enum, a ref, a list or a map.
func (g *gen) plainValue(name string, v *ir.Value) string {
	text := g.lit(v.Type, v.V, false)
	typ := g.tsType(v.Type, false)
	switch v.Type.Kind {
	case types.Map, types.DepMap:
		return fmt.Sprintf(typedConstFormat, name, typ, text)
	case types.Record, types.Variant, types.Case, types.List:
		return fmt.Sprintf(typedConstFormat, name, typ, fmt.Sprintf(freezeFormat, g.helper(canonFreezeName), typ, text))
	default:
		return fmt.Sprintf(typedConstFormat, name, typ, text)
	}
}

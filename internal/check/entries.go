package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// entryDecl is `[retired] entry t.k { … }` (TYPES.md §9.3).
func (c *checker) entryDecl(o *object) {
	d := o.decl.(*syntax.EntryDecl)
	env := c.declEnv(o)
	table, ok := env.pkg.names[d.Table.Name]
	if !ok || table.kind != ObjLet {
		c.report(env, diag.E3103.AtTarget(env.span(d.Table), d.Table.Name, o.name))
		return
	}
	c.info.NameUses[d.Table] = table
	c.dependsOn(env, table)
	elem, keyed, isColl := collectionElem(c.letType(table))
	if !isColl {
		c.report(env, diag.E3103.AtTarget(env.span(d.Table), d.Table.Name, o.name))
		return
	}
	if !literalInit(table) {
		c.report(env, diag.E3103.AtLiteral(env.span(d.Table), d.Table.Name, o.name))
	}
	o.typ = elem
	if keyed == nil {
		if k, isInt := d.Key.(*syntax.IntLit); isInt {
			c.expr(env, k, types.StringType)
		}
		c.expr(env, d.Value, elem)
		return
	}
	if d.Mods != nil && d.Mods.Retired.Valid() {
		c.report(env, diag.E3103.AtRetired(env.tokSpan(d.Mods.Retired)))
	}
	c.keyedEntryKey(env, d, table, keyed)
	c.keyedEntry(env, d, elem, keyed)
}

// literalInit reports a let initialized by a table literal or a list literal.
func literalInit(let *object) bool {
	switch v := let.decl.(*syntax.LetDecl).Value.(type) {
	case *syntax.BraceLit:
		return isTableLiteral(v)
	case *syntax.ListLit:
		return true
	}
	return false
}

// keyedEntry checks the body of an entry of a keyed list: every field but the key, which the
// entry's key gives.
func (c *checker) keyedEntry(env *env, d *syntax.EntryDecl, elem types.Type, key *types.Field) {
	for _, it := range d.Value.Items {
		if f, ok := it.(*syntax.FieldItem); ok && f.Name.Name == key.Name {
			c.report(env, diag.E3321.At(env.span(f.Name), key.Name))
		}
	}
	given := map[string]bool{key.Name: true}
	rt, ok := elem.Base().(*types.RecordType)
	if !ok {
		return
	}
	c.info.Literals[d.Value] = LitRecord
	fields := recordFields(c, rt)
	for _, it := range d.Value.Items {
		if f, isField := it.(*syntax.FieldItem); isField && f.Name.Name != key.Name {
			c.fieldItem(env, f, rt, fields, given)
		} else if !isField {
			c.report(env, diag.E3320.At(env.span(it), itemKind(it), diag.KindRecord))
		}
	}
	c.requiredFields(env, d.Value, rt, fields, given)
	c.info.Types[d.Value] = rt
}

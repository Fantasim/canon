package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// braceLit classifies `{ … }` from its items and the expected type (TYPES.md §5.2).
func (c *checker) braceLit(env *env, e *syntax.BraceLit, want types.Type) types.Type {
	if len(e.Clauses) > 0 {
		return c.mapComp(env, e, want)
	}
	switch w := unwrap(want).(type) {
	case *types.RecordType, *types.AppliedRecord, *types.CaseType:
		return c.recordLit(env, e, w)
	case *types.TableType:
		return c.tableLit(env, e, w)
	case *types.MapType:
		return c.mapLit(env, e, w.Key, w.Value, w)
	case *types.DepMapType:
		return c.mapLit(env, e, &types.RefType{Target: w.Coll}, staticView(w.Value), w)
	case *types.DepUnionType, *types.TypeAppType:
		if b := braceBranch(depFunc(w)); b != nil {
			return c.braceLit(env, e, b)
		}
	case nil:
		return c.untypedBrace(env, e)
	}
	c.info.Literals[e] = LitError
	c.itemsAlone(env, e.Items)
	if want.Kind() != types.Error {
		c.report(env, diag.E3002.At(env.span(e), want, env.literalText(e)))
	}
	return types.ErrorType
}

// braceBranch is the first branch of a type function a brace literal classifies against (DECISIONS 171).
func braceBranch(fn *types.TypeFunc) types.Type {
	return firstBranch(fn, func(r types.Type) bool {
		switch unwrap(r).(type) {
		case *types.RecordType, *types.AppliedRecord, *types.CaseType, *types.TableType, *types.MapType, *types.DepMapType:
			return true
		}
		return false
	})
}

// untypedBrace is a brace literal without an expected type: a record typed by its leading
// spread, a map of strings when every key is a string literal, else E3305 (a spread of a
// non-record included).
func (c *checker) untypedBrace(env *env, e *syntax.BraceLit) types.Type {
	if len(e.Items) > 0 {
		if sp, ok := e.Items[0].(*syntax.SpreadItem); ok {
			t := c.synth(env, sp.X)
			switch t.Base().(type) {
			case *types.RecordType, *types.AppliedRecord, *types.CaseType:
				return c.recordLit(env, e, t.Base())
			}
			c.info.Literals[e] = LitError
			if t.Kind() != types.Error {
				c.report(env, diag.E3305.At(env.span(e)))
			}
			c.itemsAlone(env, e.Items[1:])
			return types.ErrorType
		}
		if allStringKeys(e.Items) {
			return c.stringMap(env, e)
		}
	}
	if len(e.Items) == 0 && env.joining() {
		return emptyMap
	}
	c.info.Literals[e] = LitError
	c.itemsAlone(env, e.Items)
	c.report(env, diag.E3305.At(env.span(e)))
	return types.ErrorType
}

func allStringKeys(items []syntax.BraceItem) bool {
	for _, it := range items {
		m, ok := it.(*syntax.MapItem)
		if !ok {
			return false
		}
		if _, isStr := m.Key.(syntax.StrLit); !isStr || !interpolationFree(m.Key) {
			return false
		}
	}
	return true
}

// stringMap is `{"k": v, …}` without an expected type: {String: V}, V the join of the values.
func (c *checker) stringMap(env *env, e *syntax.BraceLit) types.Type {
	c.info.Literals[e] = LitMap
	var values []syntax.Expr
	for _, it := range e.Items {
		m := it.(*syntax.MapItem)
		c.expr(env, m.Key, types.StringType)
		values = append(values, m.Value)
	}
	c.duplicateKeys(env, e.Items)
	v, ok := c.joinAll(env, e, values)
	if !ok {
		return types.ErrorType
	}
	return &types.MapType{Key: types.StringType, Value: v}
}

// itemsAlone types the values of items that fit no classification.
func (c *checker) itemsAlone(env *env, items []syntax.BraceItem) {
	for _, it := range items {
		switch it := it.(type) {
		case *syntax.FieldItem:
			c.synth(env, it.Value)
		case *syntax.MapItem:
			c.synth(env, it.Key)
			c.synth(env, it.Value)
		case *syntax.SpreadItem:
			c.synth(env, it.X)
		}
	}
}

// itemKind is the Kind an item prints as in E3320.
func itemKind(it syntax.BraceItem) diag.Kind {
	switch it.(type) {
	case *syntax.MapItem:
		return diag.KindMap
	case *syntax.EntryItem:
		return diag.KindTableEntry
	case *syntax.SpreadItem:
		return diag.KindSpread
	}
	return diag.KindField
}

// tableLit is a table literal (TYPES.md §5.2, §9.3).
func (c *checker) tableLit(env *env, e *syntax.BraceLit, t *types.TableType) types.Type {
	c.info.Literals[e] = LitTable
	seen := map[string]*syntax.EntryItem{}
	for _, it := range e.Items {
		ent, ok := it.(*syntax.EntryItem)
		if !ok {
			c.report(env, diag.E3320.At(env.span(it), itemKind(it), diag.KindTable))
			c.itemsAlone(env, []syntax.BraceItem{it})
			continue
		}
		c.entryItem(env, ent, t)
		if first, dup := seen[ent.Key.Name]; dup {
			c.report(env, diag.E3101.At(env.span(ent.Key), ent.Key.Name, c.tableName(env), env.span(first.Key)))
			continue
		}
		seen[ent.Key.Name] = ent
	}
	return t
}

// entryItem checks one entry of a table literal; its key object exists when the table's keys
// are known statically, else it is made here.
func (c *checker) entryItem(env *env, ent *syntax.EntryItem, t *types.TableType) {
	o, ok := c.info.Defs[ent.Key].(*object)
	if !ok {
		o = c.newObject(ObjEntry, ent.Key.Name, env.pkg, ent, env.file)
		c.info.Defs[ent.Key] = o
	}
	if o.typ == nil {
		o.typ = t.Elem
	}
	c.expr(env, ent.Value, t.Elem)
}

// tableName is the name of the let whose initializer is checked, for E3101.
func (c *checker) tableName(env *env) string {
	if env.owner != nil {
		return env.owner.name
	}
	return ""
}

// typedLit is `Name { … }` (TYPES.md §5.2).
func (c *checker) typedLit(env *env, e *syntax.TypedLit, want types.Type) types.Type {
	t := c.literalType(env, e.Type, want)
	if t == nil {
		c.info.Literals[e.Lit] = LitError
		c.itemsAlone(env, e.Lit.Items)
		c.info.Types[e.Lit] = types.ErrorType
		return types.ErrorType
	}
	switch t.Base().(type) {
	case *types.RecordType, *types.AppliedRecord, *types.CaseType:
		c.info.Types[e.Lit] = c.recordLit(env, e.Lit, t.Base())
		return t
	}
	c.report(env, diag.E3002.At(env.span(e.Type), t, env.literalText(e)))
	c.info.Literals[e.Lit] = LitError
	c.itemsAlone(env, e.Lit.Items)
	c.info.Types[e.Lit] = types.ErrorType
	return types.ErrorType
}

// literalType resolves the name of a typed literal.
func (c *checker) literalType(env *env, q *syntax.QualifiedName, want types.Type) types.Type {
	if v := variantOf(unwrap(want)); v != nil && len(q.Parts) == 1 {
		if cs := c.caseObject(v, q.Parts[0].Name); cs != nil {
			c.info.NameUses[q.Parts[0]] = cs
			c.deprecatedUse(env, q.Parts[0], cs)
			return cs.typ
		}
	}
	o := c.typeName(env, q)
	if o == nil {
		return nil
	}
	c.dependsOn(env, o)
	if o.kind == ObjCase {
		c.deprecatedUse(env, q.Parts[len(q.Parts)-1], o)
	}
	return c.typeOfName(env, o, q)
}

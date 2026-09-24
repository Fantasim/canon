package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// collectKeys gathers the keys known statically (TYPES.md §4.1).
func (c *checker) collectKeys(p *pkgState) {
	for _, o := range p.all {
		if o.kind != ObjLet {
			continue
		}
		if lit, ok := o.decl.(*syntax.LetDecl).Value.(*syntax.BraceLit); ok && isTableLiteral(lit) {
			o.keys = &entryKeys{byName: map[string]*object{}}
			for _, it := range lit.Items {
				c.addKey(p, o, it.(*syntax.EntryItem), o.file)
			}
		}
	}
	for _, e := range p.entries {
		c.collectEntryDecl(p, e)
	}
}

// isTableLiteral reports a brace literal whose items are all table entries, `{}` included.
func isTableLiteral(lit *syntax.BraceLit) bool {
	if len(lit.Clauses) > 0 {
		return false
	}
	for _, it := range lit.Items {
		if _, ok := it.(*syntax.EntryItem); !ok {
			return false
		}
	}
	return true
}

// addKey declares the entry key of a table literal item; a key given twice is reported when
// the literal is checked (E3101).
func (c *checker) addKey(p *pkgState, table *object, it *syntax.EntryItem, f *syntax.File) {
	o := c.newObject(ObjEntry, it.Key.Name, p, it, f)
	o.parent = table
	c.info.Defs[it.Key] = o
	table.keys.add(o)
}

// collectEntryDecl makes the object of `entry t.k` and, when t has static keys, adds k.
func (c *checker) collectEntryDecl(p *pkgState, at entryAt) {
	e, f, o := at.decl, at.file, at.obj
	key, isIdent := e.Key.(*syntax.Ident)
	name := o.name
	if isIdent {
		c.info.Defs[key] = o
	}
	table, ok := p.names[e.Table.Name]
	if !ok || table.kind != ObjLet {
		return
	}
	o.parent = table
	c.info.NameUses[e.Table] = table
	if table.keys == nil || !isIdent {
		return
	}
	if first, fresh := table.keys.add(o); !fresh {
		diag.E3101.At(f.Span(key), name, table.name, declSpan(first)).Report(p.bag)
		c.breakObj(o)
	}
}

// entryKeyText is an entry key as written: a word, or an integer in decimal.
func entryKeyText(k syntax.EntryKey) string {
	switch k := k.(type) {
	case *syntax.Ident:
		return k.Name
	case *syntax.IntLit:
		return k.Value.String()
	}
	return ""
}

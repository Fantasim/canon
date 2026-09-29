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

// collectEntryDecl makes the object of `entry t.k` and, when t has static keys, adds k: a key
// given twice is E3101, kept in entryDups for Recheck to report again.
func (c *checker) collectEntryDecl(p *pkgState, at entryAt) {
	table := c.registerEntry(p, at)
	if table == nil {
		return
	}
	if first, fresh := table.keys.add(at.obj); !fresh {
		dup := entryDup{entry: at.obj, table: table, first: first}
		c.entryDups[p] = append(c.entryDups[p], dup)
		c.duplicateEntry(p, dup)
	}
}

// registerEntry records what `entry t.k` names; the result is t when it has static keys and k
// is a word, else nil.
func (c *checker) registerEntry(p *pkgState, at entryAt) *object {
	e, o := at.decl, at.obj
	key, isIdent := e.Key.(*syntax.Ident)
	if isIdent {
		c.info.Defs[key] = o
	}
	table, ok := p.names[e.Table.Name]
	if !ok || table.kind != ObjLet {
		return nil
	}
	o.parent = table
	c.info.NameUses[e.Table] = table
	if table.keys == nil || !isIdent {
		return nil
	}
	return table
}

// entryDup is an `entry` declaration whose key its table already has (E3101).
type entryDup struct {
	entry, table, first *object
}

// duplicateEntry is E3101 at the second key; it breaks the entry.
func (c *checker) duplicateEntry(p *pkgState, d entryDup) {
	key := d.entry.file.Span(d.entry.decl.(*syntax.EntryDecl).Key)
	c.deliver(p, origin{keys: p}, diag.E3101.At(key, d.entry.name, d.table.name, declSpan(d.first)).Report)
	c.breakObj(d.entry)
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

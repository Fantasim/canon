package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// ForeignRow is a table of another package's record that this package holds (CODEGEN.md §5.9, DECISIONS 323): its rows are of this package's own row type `<Element>Row`, keyed by this package's id type. Value or Field is the first holder, Origin how messages name it (the value, or `<class>.<field>`).
type ForeignRow struct {
	Record *Record
	Value  *Value
	Field  *Field
	Origin string
}

// ForeignRows are the records of other packages that a table of p holds, a value emit e selects or a field of p's own classes, each once in first-use order: the values first.
func ForeignRows(p *Package, e *Emit) []ForeignRow {
	rows := &foreignRows{own: p.Name}
	for _, v := range p.Values {
		if rec := tableRecord(v); rec != nil && emitSelects(e, v.Name) {
			rows.add(ForeignRow{Record: rec, Value: v, Origin: v.Name})
		}
	}
	for _, t := range p.Types {
		switch x := t.(type) {
		case *Record:
			rows.fields(x.QName(), x.Fields)
		case *Variant:
			for _, c := range x.Cases {
				rows.fields(x.QName()+qnameSep+c.Name, c.Fields)
			}
		}
	}
	return rows.out
}

// foreignRows collects ForeignRows.
type foreignRows struct {
	own string
	out []ForeignRow
}

// add keeps a row of another package's record, its first holder only.
func (r *foreignRows) add(row ForeignRow) {
	if row.Record.Pkg != r.own && !slices.ContainsFunc(r.out, func(o ForeignRow) bool { return o.Record == row.Record }) {
		r.out = append(r.out, row)
	}
}

// fields adds the records the table fields of class owner hold, at any depth of their types.
func (r *foreignRows) fields(owner string, fs []*Field) {
	for _, f := range fs {
		walkTypeRef(f.Type, func(t TypeRef) {
			if rec, ok := tableElem(t); ok {
				r.add(ForeignRow{Record: rec, Field: f, Origin: owner + qnameSep + f.Name})
			}
		})
	}
}

// EmitDefines are the define tables emit e of p carries, in p.Defines' order (CODEGEN.md §5.8, DECISIONS 323): those its own classes' fields and dependent branches ref (OwnDefines), and those of the fields and branches of other packages' classes it reads or writes.
func EmitDefines(p *Package, e *Emit) []*DefineTable {
	refs := emitDefineRefs(p, e)
	var out []*DefineTable
	for _, d := range p.Defines {
		if slices.ContainsFunc(refs, func(r *DefineTable) bool { return r.Pkg == d.Pkg && r.Value == d.Value }) {
			out = append(out, d)
		}
	}
	return out
}

// emitDefineRefs are ownDefineRefs, then the define tables of the other packages' classes e builds, by package and let.
func emitDefineRefs(p *Package, e *Emit) []*DefineTable {
	out := ownDefineRefs(p)
	add := func(r *RefTarget) {
		if r != nil && !slices.ContainsFunc(out, func(d *DefineTable) bool { return d.Pkg == r.Pkg && d.Value == r.Value }) {
			out = append(out, &DefineTable{Pkg: r.Pkg, Value: r.Value})
		}
	}
	for _, c := range ForeignUses(p, e).Built() {
		if d, ok := c.(*Dependent); ok {
			for _, b := range d.Branches {
				add(DefineTarget(b.Type))
			}
			continue
		}
		fields, _ := classBody(c)
		for _, f := range fields {
			if f.Input == nil {
				add(DefineTarget(f.Type))
			}
		}
	}
	return out
}

// rtKind reports a type whose Go getter returns an `rt` type (CODEGEN.md §4.2): a list, keyed list, table field or map.
func rtKind(t TypeRef) bool {
	if t.Kind == types.Optional && t.Elem != nil {
		t = *t.Elem
	}
	return goStdOfKind[t.Kind] == goRT || t.Kind == types.Table
}

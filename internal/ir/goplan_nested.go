package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// nestedRows are the records of this package that a table field of its own classes holds, in first-use order: their entries have an id and a retired flag (CODEGEN.md §4.2, §5.3); a record another package declares is held in this package's row type instead (ForeignRows, CODEGEN.md §5.9, DECISIONS 323).
func nestedRows(p *Package) []*Record {
	var out []*Record
	seen := map[*Record]bool{}
	add := func(t TypeRef) {
		if rec, ok := tableElem(t); ok && rec.Pkg == p.Name && !seen[rec] {
			seen[rec] = true
			out = append(out, rec)
		}
	}
	each := func(fields []*Field) {
		for _, f := range fields {
			walkTypeRef(f.Type, add)
		}
	}
	for _, t := range p.Types {
		switch x := t.(type) {
		case *Record:
			each(x.Fields)
		case *Variant:
			for _, c := range x.Cases {
				each(c.Fields)
			}
		}
	}
	return out
}

// NestedRow reports a record of this package that a table field holds: Go gives its entries an id even when no public table value is of it (CODEGEN.md §4.2, §5.3).
func (pl *GoNamePlan) NestedRow(rec *Record) bool {
	return slices.Contains(pl.nested, rec)
}

// HasNestedTables reports a package with a table field, of its own records or another package's, or another package's class its readers read holding one: a data-mode decoder then reads a nested table (WIRE.md §5.7; log-2026-10-06 "U2 (gen/go) done" 5).
func (pl *GoNamePlan) HasNestedTables() bool {
	if len(pl.nested) > 0 || slices.ContainsFunc(pl.rows, func(r ForeignRow) bool { return r.Field != nil }) {
		return true
	}
	return pl.data != nil && slices.ContainsFunc(pl.foreign.Read, func(class any) bool {
		return classHolds(class, func(t TypeRef) bool { return t.Kind == types.Table })
	})
}

// declareNestedIDs declares the id type of each record only a table field holds, a string in both modes: a nested table has no id constants (CODEGEN.md §5.3).
func (pl *GoNamePlan) declareNestedIDs(top *nameScope) {
	for _, rec := range pl.nested {
		if pl.isTableValueRecord(rec) {
			continue
		}
		pl.declareFrom(top, rec.QName(), rec, derivation{rec, func() string { return pl.IDTypeName(rec) }})
	}
}

// isTableValueRecord reports a record some public table value holds, emitted or not (decision 124).
func (pl *GoNamePlan) isTableValueRecord(rec *Record) bool {
	for _, v := range pl.p.Values {
		if tableRecord(v) == rec {
			return true
		}
	}
	return false
}

// tableValueRecords are the records the public table values of u hold, which baked Go gives an id enum (CODEGEN.md §5.3).
func tableValueRecords(p *Package) map[*Record]bool {
	out := map[*Record]bool{}
	for _, v := range p.Values {
		if rec := tableRecord(v); rec != nil {
			out[rec] = true
		}
	}
	return out
}

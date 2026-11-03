package ir

import (
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// GoLookupTables are the two package-level tables of the lookups of a go emit that take an enum whose index is searched: Keys holds the enum's members sorted by code (an opened enum: by wire), At the declaration index of each (CODEGEN.md §5.10, DECISIONS 435).
type GoLookupTables struct {
	Keys, At string
}

// LookupTables are the tables the lookups of this emit search an enum's index in, and whether the enum has them: a @codes enum whose codes are not dense in declaration order, or that this emit opens. A dense closed enum indexes by its code minus the first (CODEGEN.md §5.10).
func (pl *GoNamePlan) LookupTables(e *Enum) (GoLookupTables, bool) {
	t, ok := pl.lookups[e.QName()]
	return t, ok
}

// LookupEnums are the enums LookupTables answers for, in the order the lookups meet them.
func (pl *GoNamePlan) LookupEnums() []*Enum { return pl.lookupEnums }

// searchedIndex reports an enum whose lookup index is found in tables: not dense, or opened.
func (pl *GoNamePlan) searchedIndex(e *Enum) bool {
	_, dense := DenseCodes(e)
	return e.Codes != nil && (!dense || OpenEnum(pl.p, pl.e, e))
}

// lookupKey names an enum in the tables' names: its Go type, behind its package for another package's enum, so two enums named alike do not share one.
func (pl *GoNamePlan) lookupKey(e *Enum) string {
	if e.Pkg == pl.p.Name {
		return pl.TypeName(e)
	}
	return strings.ReplaceAll(e.Pkg, qnameSep, goLookupPkgSep) + goLookupPkgSep + pl.TypeName(e)
}

// lookupFn is a lookup gen/go writes in this emit, with the Canon item it is written for.
type lookupFn struct {
	fn     *ExportFn
	origin string
}

// lookupFns are the lookups gen/go writes: a baked emit's package fns, and the methods of every record and case; a types-mode emit stores none.
func (pl *GoNamePlan) lookupFns() []lookupFn {
	if pl.e.Mode == ModeTypes {
		return nil
	}
	var out []lookupFn
	add := func(origin string, fns []*ExportFn) {
		for _, fn := range fns {
			if fn.Kind == FnLookup {
				out = append(out, lookupFn{fn, origin + qnameSep + fn.Name})
			}
		}
	}
	if pl.data == nil {
		add(pl.self, pl.p.Fns)
	}
	for _, t := range pl.p.Types {
		switch x := t.(type) {
		case *Record:
			add(x.QName(), x.Methods)
		case *Variant:
			for _, c := range x.Cases {
				add(x.QName()+qnameSep+c.Name, c.Methods)
			}
		}
	}
	return out
}

// declareLookupTables declares, once per enum, the two tables the lookups taking it search (CODEGEN.md §5.10): a name already declared is E8005 at the first lookup that needs it.
func (pl *GoNamePlan) declareLookupTables(top *nameScope) {
	pl.lookups = map[string]GoLookupTables{}
	for _, l := range pl.lookupFns() {
		for _, prm := range l.fn.Params {
			e, ok := prm.Type.Named.(*Enum)
			if !ok || prm.Type.Kind != types.Enum || !pl.searchedIndex(e) {
				continue
			}
			if _, done := pl.lookups[e.QName()]; done {
				continue
			}
			key := pl.lookupKey(e)
			t := GoLookupTables{Keys: pl.TableName(goLookupKeys, key), At: pl.TableName(goLookupAt, key)}
			pl.lookups[e.QName()] = t
			pl.lookupEnums = append(pl.lookupEnums, e)
			pl.declare(top, t.Keys, l.origin, l.fn)
			pl.declare(top, t.At, l.origin, l.fn)
		}
	}
}

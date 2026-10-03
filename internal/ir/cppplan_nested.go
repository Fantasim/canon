package ir

import "slices"

// NestedRow reports a record of this package that a table field holds: its entries have an id and a retired flag, as a table value's do (CODEGEN.md §4.2, §5.3).
func (pl *CppNamePlan) NestedRow(rec *Record) bool {
	return slices.Contains(pl.nested, rec)
}

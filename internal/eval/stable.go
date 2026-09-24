package eval

import (
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// stableChange finds what LOCK.md §6.1 forbids when an amendment replaces old (of type t) by nv.
func stableChange(old, nv value.Value, t types.Type) (field string, table bool, found bool) {
	if old == nil || nv == nil {
		return "", false, false
	}
	switch b := unwrapOptional(t).Base().(type) {
	case *types.RecordType, *types.CaseType, *types.AppliedRecord:
		return stableRecord(old, nv, b)
	case *types.TableType:
		return stableEntries(old, nv, b.Elem, b.Stable)
	case *types.ListType:
		if b.KeyedBy != nil {
			return stableEntries(old, nv, b.Elem, false)
		}
	}
	return "", false, false
}

// stableRecord compares the fields of two records of type t, recursing below them.
func stableRecord(old, nv value.Value, t types.Type) (string, bool, bool) {
	o, okO := old.(*value.Record)
	n, okN := nv.(*value.Record)
	if !okO || !okN || o.T.Base() != n.T.Base() {
		return "", false, false
	}
	for i, f := range fieldsOf(t) {
		if i >= len(o.Fields) || i >= len(n.Fields) || o.Fields[i] == nil || n.Fields[i] == nil {
			continue
		}
		if f.Stable && !value.Equal(o.Fields[i], n.Fields[i]) {
			return f.Name, false, true
		}
		if name, table, found := stableChange(o.Fields[i], n.Fields[i], f.Type); found {
			return name, table, found
		}
	}
	return "", false, false
}

// stableEntries matches entries by key: a new one in a stable table, or a stable change in one.
func stableEntries(old, nv value.Value, elem types.Type, stable bool) (string, bool, bool) {
	olds := map[value.Key]value.Value{}
	for _, e := range std.Elems(old) {
		if k, ok := std.KeyOf(e); ok {
			olds[k] = e
		}
	}
	for _, e := range std.Elems(nv) {
		k, _ := std.KeyOf(e)
		prev, had := olds[k]
		if !had && stable {
			return "", true, true
		}
		if name, table, found := stableChange(prev, e, elem); found {
			return name, table, found
		}
	}
	return "", false, false
}

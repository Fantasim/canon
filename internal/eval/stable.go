package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// stableTask is one comparison left while looking for a stable change: a pair of values of
// type t to descend into, a @stable field to compare (field set), or an entry a stable table
// gains (added).
type stableTask struct {
	old, nv value.Value
	t       types.Type
	field   string
	added   bool
}

// stableChange finds what LOCK.md §6.1 forbids when an amendment replaces old (of type t) by nv.
func stableChange(old, nv value.Value, t types.Type) (field string, table bool, found bool) {
	stack := []stableTask{{old: old, nv: nv, t: t}}
	for len(stack) > 0 {
		k := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch {
		case k.added:
			return "", true, true
		case k.field != "":
			if !value.Equal(k.old, k.nv) {
				return k.field, false, true
			}
		default:
			stack = append(stack, stableTasks(k)...)
		}
	}
	return "", false, false
}

// stableTasks are the comparisons below one pair, last first so they run in order.
func stableTasks(k stableTask) []stableTask {
	if k.old == nil || k.nv == nil {
		return nil
	}
	var tasks []stableTask
	switch b := unwrapOptional(k.t).Base().(type) {
	case *types.RecordType, *types.CaseType, *types.AppliedRecord:
		tasks = stableFields(k.old, k.nv, b)
	case *types.TableType:
		tasks = stableEntries(k.old, k.nv, b.Elem, b.Stable)
	case *types.ListType:
		if b.KeyedBy != nil {
			tasks = stableEntries(k.old, k.nv, b.Elem, false)
		}
	}
	slices.Reverse(tasks)
	return tasks
}

// stableFields compares the fields of two records of type t: each @stable field, then below it.
func stableFields(old, nv value.Value, t types.Type) []stableTask {
	o, okO := old.(*value.Record)
	n, okN := nv.(*value.Record)
	if !okO || !okN || o.T.Base() != n.T.Base() {
		return nil
	}
	var tasks []stableTask
	for i, f := range fieldsOf(t) {
		if i >= len(o.Fields) || i >= len(n.Fields) || o.Fields[i] == nil || n.Fields[i] == nil {
			continue
		}
		if f.Stable {
			tasks = append(tasks, stableTask{old: o.Fields[i], nv: n.Fields[i], field: f.Name})
		}
		tasks = append(tasks, stableTask{old: o.Fields[i], nv: n.Fields[i], t: f.Type})
	}
	return tasks
}

// stableEntries matches entries by key: a new one in a stable table, or a stable change in one.
func stableEntries(old, nv value.Value, elem types.Type, stable bool) []stableTask {
	olds := map[value.Key]value.Value{}
	for _, e := range std.Elems(old) {
		if k, ok := std.KeyOf(e); ok {
			olds[k] = e
		}
	}
	var tasks []stableTask
	for _, e := range std.Elems(nv) {
		k, _ := std.KeyOf(e)
		prev, had := olds[k]
		if !had && stable {
			tasks = append(tasks, stableTask{added: true})
		}
		tasks = append(tasks, stableTask{old: prev, nv: e, t: elem})
	}
	return tasks
}

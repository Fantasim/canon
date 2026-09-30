package build

import (
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
)

// staleSpans names each span of a's findings and selected values' provenance in an older version
// of one of its program's files: rendered alike, as the shared prefix is, but unequal as a value
// (checkRuns, an edit's region), which the dump, resolving spans, cannot see.
func staleSpans(a *Analysis) []string {
	current := map[string]source.FileID{}
	for _, cp := range a.Program().Packages {
		for _, f := range cp.Files {
			current[f.Src.Path] = f.Src.ID
		}
	}
	files := a.Files()
	var out []string
	note := func(what string, id source.FileID) {
		if want, ok := current[files.Path(id)]; ok && want != id {
			out = append(out, fmt.Sprintf("%s in %s#%d, not #%d", what, files.Path(id), id, want))
		}
	}
	for _, f := range a.Result().List {
		note(string(f.Code), f.Span.File)
		for _, r := range f.Related {
			note(string(f.Code)+" related", r.Span.File)
		}
	}
	for _, v := range selectedValues(a) {
		for p := v.Prov(); p != nil; p = p.Via {
			note("provenance of "+v.CanonText(), p.Span.File)
			for _, fr := range p.Stack {
				note("a frame of "+v.CanonText(), fr.Span.File)
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// selectedValues is every value the selected packages' consts and lets hold, in depth.
func selectedValues(a *Analysis) []value.Value {
	seen := map[value.Value]bool{}
	var queue []value.Value
	add := func(v value.Value) {
		if v != nil && !seen[v] {
			seen[v] = true
			queue = append(queue, v)
		}
	}
	for _, cp := range a.Program().Packages {
		if !a.r.selects(cp.Path) {
			continue
		}
		for _, obj := range cp.Decls {
			if obj.Kind() == check.ObjConst || obj.Kind() == check.ObjLet {
				v, _ := a.Force(eval.Root{Pkg: cp.Path, Name: obj.Name()})
				add(v)
			}
		}
	}
	for i := 0; i < len(queue); i++ {
		for _, p := range valueParts(queue[i]) {
			add(p)
		}
	}
	return queue
}

// valueParts are the values v holds.
func valueParts(v value.Value) []value.Value {
	switch x := v.(type) {
	case *value.Record:
		return x.Fields
	case *value.List:
		return x.Elems
	case *value.Map:
		return append(slices.Clone(x.Keys), x.Vals...)
	case *value.Table:
		out := make([]value.Value, 0, len(x.Entries))
		for _, en := range x.Entries {
			out = append(out, en)
		}
		return out
	case *value.Pair:
		return []value.Value{x.A, x.B}
	}
	return nil
}

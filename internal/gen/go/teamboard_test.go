package gogen_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	teamboardIR     = "testdata/ir/teamboard.json"
	teamboardGolden = "testdata/teamboard"
)

// teamboardCells evaluates taxonomy.canon's export fns in domain order, as stage E would (§5.10).
func teamboardCells(w *world, fn *ir.ExportFn) []value.Value {
	statuses := w.entries("teamboard.statuses")
	var cells []value.Value
	switch fn.Name {
	case "canTransition":
		for _, from := range statuses {
			for _, to := range statuses {
				cells = append(cells, &value.Bool{V: slices.Contains(refKeys(field(from, "next")), to.Ident.Key.S)})
			}
		}
	case "columnOf":
		for _, s := range statuses {
			cells = append(cells, columnOf(w, s.Ident.Key.S))
		}
	case "areasVisibleTo":
		for role := range len(w.types["Role"].enum.Members) {
			cells = append(cells, &value.List{Elems: visibleAreas(w, role, "")})
		}
	case "groupedAreasVisibleTo":
		for role := range len(w.types["Role"].enum.Members) {
			cells = append(cells, groupedAreas(w, role))
		}
	}
	return cells
}

// columnOf is `columns.active().first(c => c.statuses.contains(s))`.
func columnOf(w *world, status string) value.Value {
	for _, c := range w.entries("teamboard.columns") {
		if slices.Contains(refKeys(field(c, "statuses")), status) {
			return refOf(c.Ident.Key.S)
		}
	}
	return &value.None{}
}

// visibleAreas is `[a for a in areas.active() if role >= a.minRole]`, of one group if given.
func visibleAreas(w *world, role int, group string) []value.Value {
	var out []value.Value
	for _, a := range w.entries("teamboard.areas") {
		inGroup := group == "" || field(a, "group").(*value.Ref).Key.S == group
		if !a.Ident.Retired && inGroup && role >= field(a, "minRole").(*value.Member).Index {
			out = append(out, refOf(a.Ident.Key.S))
		}
	}
	return out
}

// groupedAreas is groupedAreasVisibleTo's comprehension: a section per group with an area.
func groupedAreas(w *world, role int) value.Value {
	section := w.types["AreaSection"].rec
	list := &value.List{}
	for _, g := range w.entries("teamboard.areaGroups") {
		areas := visibleAreas(w, role, g.Ident.Key.S)
		if len(areas) == 0 {
			continue
		}
		list.Elems = append(list.Elems, &value.Record{T: section, Fields: []value.Value{
			refOf(g.Ident.Key.S), &value.List{Elems: areas},
		}})
	}
	return list
}

// CODEGEN.md §6.2, §10: the teamboard, roles and ui packages in baked mode equal their goldens.
func TestTeamboardGolden(t *testing.T) {
	w := loadWorld(t, teamboardIR, teamboardCells)
	files := generate(t, w)
	checkGoldens(t, files, sortedPaths(files), teamboardGolden)
}

// CODEGEN.md §9, decision 205: the installed Go builds it; getters read every value.
func TestTeamboardCompiles(t *testing.T) {
	w := loadWorld(t, teamboardIR, teamboardCells)
	files := generate(t, w)
	compile(t, w, files, sortedPaths(files), [][2]string{{"teamboard/smoke_test.go", "testdata/smoke/teamboard_test.go"}})
}

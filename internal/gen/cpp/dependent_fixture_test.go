package cppgen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// dependentPackage is an Event of dependent fields: Payload over a Bool, an optional Multi (a and
// b both String, c and e one arm, d Never), Deep (renamed Depth) over a held record's field, and
// a list of Payload.
func dependentPackage() *ir.Package {
	kind := enumOf("Kind", "spawn", "despawn")
	kind3 := enumOf("Kind3", "a", "b", "c", "d", "e")
	tKind3 := ir.TypeRef{Kind: types.Enum, Named: kind3}
	eventType := &ir.Record{Pkg: "demo", Name: "EventType", Fields: []*ir.Field{field("param", "param", "", tKind3)}}
	payload := &ir.Dependent{
		Pkg: "demo", Name: "Payload", Doc: "What an event carries.", Params: 1, Disc: &tBool, ByMember: []int{0, 1},
		Branches: []*ir.Branch{{Name: "false", Members: []int{0}, Type: tString}, {Name: "true", Members: []int{1}, Type: ir.TypeRef{Kind: types.Enum, Named: kind}}},
	}
	multi := &ir.Dependent{
		Pkg: "demo", Name: "Multi", Params: 1, Disc: &tKind3, ByMember: []int{0, 1, 2, ir.NoBranch, 2},
		Branches: []*ir.Branch{{Name: "a", Members: []int{0}, Type: tString}, {Name: "b", Members: []int{1}, Type: tString}, {Name: "c", Members: []int{2, 4}, Type: tInt32}},
	}
	deep := &ir.Dependent{
		Pkg: "demo", Name: "Deep", Cpp: ir.NameOptions{Name: "Depth"}, Params: 1, DiscPath: []string{"param"}, Disc: &tKind3, ByMember: []int{0, 1, 1, 1, 1},
		Branches: []*ir.Branch{{Name: "a", Members: []int{0}, Type: tBool}, {Name: "b", Members: []int{1, 2, 3, 4}, Type: tFloat}},
	}
	onActive := appOf(payload, "active")
	multiField := field("multi", "multi", "", appOf(multi, "kind3"))
	multiField.Optional = true
	event := &ir.Record{Pkg: "demo", Name: "Event", Fields: []*ir.Field{
		field("active", "active", "", tBool), field("payload", "payload", "", onActive),
		field("kind3", "kind3", "", tKind3), multiField,
		field("et", "et", "", ir.TypeRef{Kind: types.Record, Named: eventType}), field("deep", "deep", "", appOf(deep, "et")),
		field("many", "many", "", listOf(onActive)),
	}}
	v := &ir.Value{Name: "event", Schema: "demo.Event@00000001", Type: ir.TypeRef{Kind: types.Record, Named: event}}
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: "demo/out", Mode: ir.ModeData, Namespace: "demo"}
	return &ir.Package{
		Name: "demo", Dir: "demo", Types: []ir.Type{kind, kind3, payload, multi, eventType, deep, event},
		Values: []*ir.Value{v}, Emits: []*ir.Emit{emit},
	}
}

// enumOf is an enum of package demo whose members are named and wired as given.
func enumOf(name string, members ...string) *ir.Enum {
	e := &ir.Enum{Pkg: "demo", Name: name}
	for i, m := range members {
		e.Members = append(e.Members, &ir.EnumMember{Name: m, Wire: m, Index: i})
	}
	return e
}

// appOf is d applied to the earlier field at wire path from (TYPES.md §11.1).
func appOf(d *ir.Dependent, from ...string) ir.TypeRef {
	return ir.TypeRef{Kind: types.TypeApp, Named: d, Args: []*ir.Source{{From: types.ArgField, WirePath: from}}}
}

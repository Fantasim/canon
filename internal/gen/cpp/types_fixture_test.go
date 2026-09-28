package cppgen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// typesTime is examples/sovcommon/time after stage E: a types-mode emit of an enum and two records (CODEGEN.md §5.13).
func typesTime() *ir.Package {
	day := &ir.Enum{Pkg: "sovcommon.time", Name: "Weekday"}
	for i, m := range []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"} {
		day.Members = append(day.Members, &ir.EnumMember{Name: m, Wire: m, Index: i})
	}
	tod := &ir.Record{Pkg: "sovcommon.time", Name: "TimeOfDay", Fields: []*ir.Field{
		field("hour", "hour", "", tInt), field("minute", "minute", "", tInt),
	}}
	tTod := ir.TypeRef{Kind: types.Record, Named: tod}
	window := &ir.Record{Pkg: "sovcommon.time", Name: "Window", Fields: []*ir.Field{
		field("day", "day", "", ir.TypeRef{Kind: types.Enum, Named: day}),
		field("startUtc", "startUtc", "", tTod), field("endUtc", "endUtc", "", tTod),
	}}
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: "out/time", Mode: ir.ModeTypes, Namespace: "sov::time"}
	return &ir.Package{Name: "sovcommon.time", Dir: "sovcommon/time", Types: []ir.Type{day, tod, window}, Emits: []*ir.Emit{emit}}
}

// typesEvents is examples/resource/events after stage E, its monsters those of the fixture's defineObj.h;
// Extras holds the source-wire forms events does not use: a path, a none marker, a union, a fraction.
func typesEvents(time *ir.Package) *ir.Package {
	roll := &ir.Enum{Pkg: "resource.events", Name: "RollMode", Members: []*ir.EnumMember{
		{Name: "authoritative", Wire: "authoritative"}, {Name: "local_budget", Wire: "local_budget", Index: 1},
	}}
	tRoll := ir.TypeRef{Kind: types.Enum, Named: roll}
	rect := &ir.Record{Pkg: "resource.events", Name: "Rect"}
	for _, side := range []string{"left", "top", "right", "bottom"} {
		rect.Fields = append(rect.Fields, field(side, side, "", tFloat))
	}
	kind := eventKind(ir.TypeRef{Kind: types.Record, Named: rect})
	window := ir.TypeRef{Kind: types.Record, Named: time.Types[2]}
	inline := &ir.Field{Name: "kind", Type: ir.TypeRef{Kind: types.Variant, Named: kind}, Inline: true}
	rollMode := field("rollMode", "rollMode", "", tRoll)
	rollMode.Default = &value.Member{Index: 1}
	event := &ir.Record{Pkg: "resource.events", Name: "Event", Fields: []*ir.Field{
		field("id", "id", "", tString), field("worldId", "worldId", "", sized(32, false)),
		field("targetCount", "targetCount", "", tInt), inline, field("schedule", "schedule", "", listOf(window)), rollMode,
	}}
	elem := ir.TypeRef{Kind: types.Record, Named: event}
	config := &ir.Record{Pkg: "resource.events", Name: "EventConfig", Fields: []*ir.Field{
		field("version", "version", "", tInt),
		field("events", "events", "", ir.TypeRef{Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"id"}}}),
	}}
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: "out/events", Mode: ir.ModeTypes, Namespace: "NMEvent::gen"}
	return &ir.Package{
		Defines: []*ir.DefineTable{{Pkg: "resource.events", Value: "monsters", Names: []string{"MI_AIBATT1", "MI_BANG1"}, Values: []int64{20, 44}}},
		Name:    "resource.events", Dir: "resource/events", Types: append([]ir.Type{rect, kind, roll, event, config, extras(tRoll)}, picks(tRoll)...),
		Imports: []*ir.PackageRef{{Name: time.Name, Dir: time.Dir, Emits: time.Emits}}, Emits: []*ir.Emit{emit},
	}
}

// eventKind is events' EventKind, tag "type": its lifetimes are in seconds on the wire, its counts default to [1, 1].
func eventKind(tRect ir.TypeRef) *ir.Variant {
	lifetime := func(name, wire string) *ir.Field {
		f := field(name, wire, "", tDuration)
		f.Optional, f.Unit, f.Default = true, types.UnitS, &value.None{}
		return f
	}
	itemID := func() *ir.Field {
		return field("itemId", "itemId", "", ir.TypeRef{Kind: types.Ref, Key: &tString, Ref: &ir.RefTarget{
			Coll: types.CollLet, Pkg: "resource.vocab", Value: "items", Keyed: true,
		}})
	}
	count := func() *ir.Field {
		f := field("itemCount", "itemCount", "", listOf(tInt))
		f.Default = &value.List{Elems: []value.Value{num(1), num(1)}}
		return f
	}
	level := func(name string) *ir.Field {
		f := field(name, name, "", tInt)
		f.Default = num(0)
		return f
	}
	return &ir.Variant{Pkg: "resource.events", Name: "EventKind", Tag: "type", Cases: []*ir.Case{
		{Name: "spawn_monster", Wire: "spawn_monster", Fields: []*ir.Field{
			field("monsterId", "monsterId", "", ir.TypeRef{Kind: types.Ref, Key: &tString, Ref: &ir.RefTarget{
				Coll: types.CollDefines, Pkg: "resource.events", Value: "monsters", Local: true,
			}}), field("spawnRegion", "spawnRegion", "", tRect), lifetime("monsterLifetime", "monsterLifetimeSec"),
		}},
		{Name: "spawn_item", Wire: "spawn_item", Fields: []*ir.Field{itemID(), count(), field("spawnRegion", "spawnRegion", "", tRect), lifetime("groundLifetime", "groundLifetimeSec")}},
		{Name: "monster_drop_inject", Wire: "monster_drop_inject", Fields: []*ir.Field{itemID(), count(), level("levelMin"), level("levelMax")}},
	}}
}

// extras: defaults under a path, beside a none marker, of a union and of a Duration in seconds (WIRE.md §5.4).
func extras(tRoll ir.TypeRef) *ir.Record {
	reqMp := &ir.Field{Name: "reqMp", WirePath: []string{"legacy", "reqMp"}, Type: tInt, Default: num(7)}
	label := field("label", "label", "", tString)
	label.Optional, label.Default, label.NoneWire = true, str("x"), []byte(`""`)
	mode := field("mode", "mode", "", ir.TypeRef{Kind: types.LitUnion, Elem: &tRoll, Literals: []string{"auto"}})
	mode.Default = &value.Member{Index: 0}
	delay := field("delay", "delaySec", "", tDuration)
	delay.Unit, delay.Default = types.UnitS, dur(1500)
	ratio := field("ratio", "ratio", "", tFloat32)
	ratio.Default = flt(0.5)
	return &ir.Record{Pkg: "resource.events", Name: "Extras", Fields: []*ir.Field{reqMp, label, mode, delay, ratio}}
}

// picks: a required union over an enum, then pairs whose slot values are such unions (CODEGEN.md §5.13).
func picks(tRoll ir.TypeRef) []ir.Type {
	union := ir.TypeRef{Kind: types.LitUnion, Elem: &tRoll, Literals: []string{"auto"}}
	slot := &ir.Record{Pkg: "resource.events", Name: "Slot", Fields: []*ir.Field{field("n", "n", "", tInt), field("mode", "mode", "", union)}}
	slots := &ir.Field{Name: "slots", Type: listOf(ir.TypeRef{Kind: types.Record, Named: slot}), Pairs: &types.Pairs{Keys: [2]string{"n{i}", "m{i}"}, Slots: 2}}
	return []ir.Type{slot, &ir.Record{Pkg: "resource.events", Name: "Picks", Fields: []*ir.Field{field("a", "a", "", union), slots}}}
}

func typesTimeFixture() *ir.Package { return typesTime() }

func typesEventsFixture() *ir.Package { return typesEvents(typesTime()) }

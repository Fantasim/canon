package ir_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// Vector 3, examples/resource/events/event.canon: an inline variant, units, sovcommon.time's
// records, and define-table refs as ref(String).
func vectorEventConfig() fpCase {
	rect := &ir.Record{Pkg: "resource.events", Name: "Rect", Fields: []*ir.Field{
		fld(float(), "left"), fld(float(), "top"), fld(float(), "right"), fld(float(), "bottom"),
	}}
	itemCount := listOf(integer())
	kind := &ir.Variant{Pkg: "resource.events", Name: "EventKind", Tag: "type", Cases: []*ir.Case{
		{Name: "spawn_monster", Wire: "spawn_monster", Fields: []*ir.Field{
			fld(refString(), "monsterId"), fld(named(rect), "spawnRegion"), unit(opt(fld(duration(), "monsterLifetimeSec"), ""), types.UnitS),
		}},
		{Name: "spawn_item", Wire: "spawn_item", Fields: []*ir.Field{
			fld(refString(), "itemId"), fld(itemCount, "itemCount"), fld(named(rect), "spawnRegion"),
			unit(opt(fld(duration(), "groundLifetimeSec"), ""), types.UnitS),
		}},
		{Name: "monster_drop_inject", Wire: "monster_drop_inject", Fields: []*ir.Field{
			fld(refString(), "itemId"), fld(itemCount, "itemCount"), fld(integer(), "levelMin"), fld(integer(), "levelMax"),
		}},
	}}
	weekday := enumOf("sovcommon.time", "Weekday", "Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat")
	timeOfDay := &ir.Record{Pkg: "sovcommon.time", Name: "TimeOfDay", Fields: []*ir.Field{fld(integer(), "hour"), fld(integer(), "minute")}}
	window := &ir.Record{Pkg: "sovcommon.time", Name: "Window", Fields: []*ir.Field{
		fld(named(weekday), "day"), fld(named(timeOfDay), "startUtc"), fld(named(timeOfDay), "endUtc"),
	}}
	rollMode := enumOf("resource.events", "RollMode", "authoritative", "local_budget")
	inline := &ir.Field{Name: "kind", Type: named(kind), Inline: true}
	event := &ir.Record{Pkg: "resource.events", Name: "Event", Fields: []*ir.Field{
		fld(str(), "id"), fld(ir.TypeRef{Kind: types.Int, Bits: 32}, "worldId"), fld(integer(), "targetCount"), inline,
		fld(listOf(named(window)), "schedule"), fld(named(rollMode), "rollMode"),
	}}
	elem := named(event)
	config := &ir.Record{Pkg: "resource.events", Name: "EventConfig", Fields: []*ir.Field{
		fld(integer(), "version"),
		fld(ir.TypeRef{Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"id"}}}, "events"),
	}}
	return fpCase{"resource.events", "eventConfig", named(config), nil}
}

// Vector 4, examples/resource/heistia/heistia.canon: `filterParam: Param(eventType)?` reads
// its discriminant from the entry the earlier field eventType refers to.
func vectorHeistia() fpCase {
	param := named(paramOf(questStyleOf()))
	param.Kind, param.Args = types.TypeApp, []*ir.Source{{From: types.ArgField, WirePath: []string{"eventType"}}}
	task := &ir.Record{Pkg: "resource.heistia", Name: "Task", Fields: []*ir.Field{
		fld(refString(), "eventType"), opt(fld(param, "filterParam"), `""`), fld(integer(), "targetPerPlayer"),
		unit(fld(duration(), "maxDurationMin"), types.UnitM), fld(str(), "description"),
	}}
	reward := &ir.Record{Pkg: "resource.heistia", Name: "Reward", Fields: []*ir.Field{
		fld(refString(), "itemId"), fld(integer(), "quantity"), fld(integer(), "weight"),
	}}
	buff := &ir.Record{Pkg: "resource.heistia", Name: "Buff", Fields: []*ir.Field{
		fld(integer(), "buffResId"), unit(fld(duration(), "durationSec"), types.UnitS), fld(integer(), "weight"),
	}}
	config := &ir.Record{Pkg: "resource.heistia", Name: "HeistiaConfig", Fields: []*ir.Field{
		fld(integer(), "version"), fld(listOf(named(task)), "tasks"), fld(listOf(listOf(named(reward))), "rewardPools"),
		fld(listOf(named(buff)), "buffPool"),
	}}
	return fpCase{"resource.heistia", "heistia", named(config), nil}
}

// Vector 10, examples/resource/adventurequest/adventurequest.canon: HourlyTarget(e) bound to
// the dependent map's key, SpecificKey(e) = Param(e) | "default" reading param0.
func vectorAdventureQuests() fpCase {
	qs := questStyleOf()
	stage := withCodes(enumOf("resource.vocab", "Stage", "Stage_1", "Stage_2", "Stage_3"), &ir.TypeRef{Kind: types.Int, Bits: 16}, 100)
	rewardType := withCodes(enumOf("resource.vocab", "RewardType", "activity_points", "season_pass_exp"), uint8Ref(), 1)
	levelDiff := &ir.Record{Pkg: "resource.adventurequest", Name: "LevelDiff", Fields: []*ir.Field{
		fld(integer(), "minMob"), fld(integer(), "maxMob"), fld(integer(), "minGiant"), fld(integer(), "maxGiant"),
	}}
	global := &ir.Record{Pkg: "resource.adventurequest", Name: "Global", Fields: []*ir.Field{
		opt(fld(integer(), "min_level"), ""), opt(fld(integer(), "max_quests_per_style"), ""),
		unit(fld(duration(), "completion_reroll_cooldown_sec"), types.UnitS), fld(named(levelDiff), "levelDiff"),
	}}
	rewards := &ir.Record{Pkg: "resource.adventurequest", Name: "Rewards", Fields: []*ir.Field{
		fld(named(rewardType), "type"), fld(mapOf(named(stage), integer()), "amounts"),
	}}
	style := &ir.Record{Pkg: "resource.adventurequest", Name: "Style", Fields: []*ir.Field{
		unit(opt(fld(duration(), "reroll_time_sec"), ""), types.UnitS), unit(fld(duration(), "target_time_sec"), types.UnitS),
		fld(integer(), "max_target_days"), fld(listOf(integer()), "task_slot_weights"),
		fld(mapOf(refString(), integer()), "task_weights"), opt(fld(named(rewards), "rewards"), "{}"),
	}}
	taskCount := enumOf("resource.adventurequest", "TaskCount", "2_tasks", "3_tasks")
	multiplier := &ir.Record{Pkg: "resource.adventurequest", Name: "Multiplier", Fields: []*ir.Field{
		fld(float(), "default"), fld(mapOf(refString(), float()), "overrides"),
	}}
	hourly := hourlyTarget(qs, stage)
	condition := enumOf("resource.adventurequest", "Condition", "net_worth", "level")
	amplifier := &ir.Record{Pkg: "resource.adventurequest", Name: "Amplifier", Fields: []*ir.Field{
		fld(named(condition), "condition"), opt(fld(integer(), "priority"), ""), fld(integer(), "min_value"),
		fld(integer(), "max_value"), fld(float(), "bonus_min"), fld(float(), "bonus_max"),
	}}
	levelRange := &ir.Record{Pkg: "resource.adventurequest", Name: "LevelRange", Fields: []*ir.Field{fld(integer(), "min"), fld(integer(), "max")}}
	taskFilter := &ir.Record{Pkg: "resource.adventurequest", Name: "TaskFilter", Fields: []*ir.Field{fld(named(levelRange), "level")}}
	hourlyRef, key := named(hourly), refString()
	hourlyRef.Args = []*ir.Source{{From: types.ArgKey}}
	config := &ir.Record{Pkg: "resource.adventurequest", Name: "AdventureQuestConfig", Fields: []*ir.Field{
		fld(integer(), "version"), fld(named(global), "global"), fld(listOf(refString()), "unique_per_batch"),
		fld(mapOf(named(qs), named(style)), "styles"), fld(mapOf(named(taskCount), named(multiplier)), "target_multipliers"),
		fld(ir.TypeRef{Kind: types.DepMap, Key: &key, Elem: &hourlyRef}, "hourly_targets"),
		fld(mapOf(refString(), listOf(named(amplifier))), "rate_amplifiers"), fld(mapOf(refString(), named(taskFilter)), "task_filters"),
	}}
	return fpCase{"resource.adventurequest", "adventureQuests", named(config), nil}
}

// hourlyTarget is `record HourlyTarget(e: EventType) @json(case: snake)`, its dependent keys
// SpecificKey(e) reading their discriminant from parameter 0.
func hourlyTarget(qs, stage *ir.Enum) *ir.Record {
	param := named(paramOf(qs))
	param.Kind, param.Args = types.TypeApp, []*ir.Source{{From: types.ArgParam, Param: 0}}
	specificKey := ir.TypeRef{Kind: types.LitUnion, Elem: &param, Literals: []string{"default"}}
	rates := mapOf(named(stage), integer())
	return &ir.Record{Pkg: "resource.adventurequest", Name: "HourlyTarget", Params: 1, Fields: []*ir.Field{
		opt(fld(rates, "rates"), ""), opt(fld(mapOf(specificKey, rates), "rates_by_specific"), ""),
		opt(fld(mapOf(specificKey, integer()), "daily_max_by_specific"), ""), fld(boolean(), "is_daily_flat"),
	}}
}

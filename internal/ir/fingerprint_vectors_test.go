package ir_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// Vector 1, examples/pipeline/potion.canon: healFor is translated, so not covered.
func vectorPotions() fpCase {
	potion := &ir.Record{Pkg: "pipeline", Name: "Potion", Fields: []*ir.Field{
		fld(str(), "dwID"), fld(str(), "szName"), fld(integer(), "nHeal"), fld(duration(), "dwCooldownMs"), fld(integer(), "nStack"),
	}, Methods: []*ir.ExportFn{
		{Name: "isStrong", Kind: ir.FnPrecomputed, Result: boolean()},
		{Name: "healFor", Kind: ir.FnTranslated, Params: []*ir.Param{{Name: "missingHp", Type: integer()}}, Result: integer()},
	}}
	elem := named(potion)
	return fpCase{"pipeline", "potions", ir.TypeRef{Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"dwID"}}}, nil}
}

// Vector 2, examples/teamboard/taxonomy.canon: Tone of sovcommon.ui is covered by its wire values.
func vectorStatuses() fpCase {
	tone := enumOf("sovcommon.ui", "Tone", "warning", "info", "accent", "success", "neutral", "danger",
		"series-1", "series-2", "series-3", "series-4", "series-5", "series-6")
	postField := enumOf("teamboard", "PostField", "assignee", "fixed_in", "reason", "duplicate_of")
	actor := enumOf("teamboard", "Actor", "reporter", "triager", "reporter_or_triager")
	status := &ir.Record{Pkg: "teamboard", Name: "Status", Fields: []*ir.Field{
		fld(named(tone), "tone"), fld(str(), "label"), fld(boolean(), "terminal"), fld(listOf(refString()), "next"),
		fld(listOf(named(postField)), "requires"), fld(listOf(named(postField)), "optional"), opt(fld(named(actor), "by"), ""),
	}}
	elem := named(status)
	return fpCase{"teamboard", "statuses", ir.TypeRef{Kind: types.Table, Elem: &elem}, nil}
}

// Vector 5, examples/balance/parity/sweep_plan.canon.
func vectorPlan() fpCase {
	profile := enumOf("balance.parity", "Profile", "pve", "pvp")
	skill := &ir.Record{Pkg: "balance.parity", Name: "PlannedSkill", Fields: []*ir.Field{
		fld(integer(), "slot"), fld(integer(), "id"), fld(integer(), "mp"),
	}}
	combo := &ir.Record{Pkg: "balance.parity", Name: "Combo", Fields: []*ir.Field{
		fld(str(), "job"), fld(str(), "jobToken"), fld(integer(), "level"), fld(str(), "weapon"), fld(integer(), "weaponId"),
		fld(str(), "weaponKind"), fld(boolean(), "ranged"), fld(listOf(named(profile)), "profiles"), fld(listOf(named(skill)), "skills"),
	}}
	plan := &ir.Record{Pkg: "balance.parity", Name: "Plan", Fields: []*ir.Field{
		fld(str(), "charName"), fld(integer(), "hitsPerBlock"), unit(fld(duration(), "actionDelayMs"), types.UnitMs),
		fld(mapOf(named(profile), integer()), "totemIndex"), fld(listOf(named(combo)), "combos"),
	}}
	return fpCase{"balance.parity", "plan", named(plan), nil}
}

// Vector 6, WIRE.md §8.3's fpdemo: a retired member is covered; Skill is @json(case: snake).
func vectorSkills() fpCase {
	element := withCodes(enumOf("fpdemo", "Element", "FIRE", "WATER", "WIND"), uint8Ref(), 1)
	element.JSONCodes, element.Members[2].Retired = true, true
	flag := enumOf("fpdemo", "Flag", "tradable", "droppable", "soulbound")
	flag.Codes = &ir.TypeRef{Kind: types.Int, Bits: 32}
	flag.Members[0].Code, flag.Members[1].Code, flag.Members[2].Code = 1, 2, 4
	side := enumOf("fpdemo", "Side", "left", "RIGHT")
	flags := fld(listOf(named(flag)), "flags")
	flags.Enc = types.EncBits
	twoHanded := fld(boolean(), "bTwoHanded")
	twoHanded.Enc = types.EncInt
	sideRef := named(side)
	skill := &ir.Record{Pkg: "fpdemo", Name: "Skill", Fields: []*ir.Field{
		fld(str(), "dwID"), fld(integer(), "legacy", "reqMp"), fld(integer(), "legacy", "reqFp"),
		opt(fld(named(element), "element"), "0"), flags, twoHanded,
		fld(ir.TypeRef{Kind: types.LitUnion, Elem: &sideRef, Literals: []string{"both"}}, "side"),
		unit(opt(fld(duration(), "cast_time"), ""), types.UnitS), fld(mapOf(named(element), float()), "weights"),
	}, Methods: []*ir.ExportFn{{Name: "isFree", Kind: ir.FnPrecomputed, Result: boolean()}}}
	elem := named(skill)
	return fpCase{"fpdemo", "skills", ir.TypeRef{Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"dwID"}}}, nil}
}

// Vector 7: `Layout` is an alias of a refined String, expanded and unrefined.
func vectorDeck() fpCase {
	deck := &ir.Record{Pkg: "teamboard", Name: "Deck", Fields: []*ir.Field{fld(listOf(str()), "layouts"), fld(integer(), "maxHidden")}}
	return fpCase{"teamboard", "deck", named(deck), nil}
}

// Vector 8: a value of another package's ordered enum.
func vectorRole() fpCase {
	role := enumOf("sovcommon.roles", "Role", "member", "gm_junior", "gm_senior", "maintainer", "owner", "admin")
	role.Ordered = true
	return fpCase{"teamboard", "assigneeMinRole", named(role), nil}
}

// Vector 9, WIRE.md §8.3's flow: the first emitted value carries the package fns.
func vectorFlow() fpCase {
	status := &ir.Record{
		Pkg: "flow", Name: "Status", Fields: []*ir.Field{fld(str(), "label"), fld(listOf(refString()), "next")},
		Methods: []*ir.ExportFn{{Name: "isTerminal", Kind: ir.FnPrecomputed, Result: boolean()}},
	}
	elem := named(status)
	canTransition := &ir.ExportFn{
		Name: "canTransition", Kind: ir.FnLookup, Result: boolean(),
		Params: []*ir.Param{{Name: "from", Type: refString()}, {Name: "to", Type: refString()}},
	}
	return fpCase{"flow", "statuses", ir.TypeRef{Kind: types.Table, Elem: &elem}, []*ir.ExportFn{canTransition}}
}

// paramOf is resource.vocab's `Param(e: EventType) = match e.param {…}` (vectors 4 and 10).
func paramOf(questStyle *ir.Enum) *ir.Dependent {
	paramKind := enumOf("resource.vocab", "ParamKind", "none_", "monster", "item", "dungeon", "element", "stat",
		"upgradeType", "gameMode", "questStyle")
	element := withCodes(enumOf("resource.vocab", "Element", "FIRE", "WATER", "ELECTRICITY", "WIND", "EARTH"), uint8Ref(), 1)
	disc := named(paramKind)
	return &ir.Dependent{
		Pkg: "resource.vocab", Name: "Param", Params: 1, DiscParam: 0, DiscPath: []string{"param"}, Disc: &disc,
		Branches: []*ir.Branch{
			{Name: "monster", Members: []int{1}, Type: refString()},
			{Name: "item", Members: []int{2}, Type: refString()},
			{Name: "dungeon", Members: []int{3}, Type: refString()},
			{Name: "element", Members: []int{4}, Type: named(element)},
			{Name: "quest_style", Members: []int{8}, Type: named(questStyle)},
			{Name: "upgrade_type", Members: []int{6}, Type: refString()},
			{Name: "game_mode", Members: []int{7}, Type: str()},
		},
		ByMember: []int{ir.NoBranch, 0, 1, 2, 3, ir.NoBranch, 5, 6, 4},
	}
}

// questStyleOf is resource.vocab's QuestStyle.
func questStyleOf() *ir.Enum {
	return withCodes(enumOf("resource.vocab", "QuestStyle", "daily", "weekly", "forever", "season_pass", "pvp", "azure"), uint8Ref(), 1)
}

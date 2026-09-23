package jsongen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

const tb = "teamboard"

// taxonomy is the enums and records of examples/teamboard/taxonomy.canon and its imports.
type taxonomy struct {
	tone, icon, role, actor, postField           enumDef
	intent, status, deck, areaGroup, area        recDef
	intents, statuses, areaGroups, areas         *tableDef
	statusRef, areaGroupRef, areaRef, layoutList ftype
}

func newTaxonomy() *taxonomy {
	x := &taxonomy{
		tone: newEnum("sovcommon.ui", "Tone", "warning", "info", "accent", "success", "neutral", "danger",
			"series_1=series-1", "series_2=series-2", "series_3=series-3", "series_4=series-4", "series_5=series-5", "series_6=series-6"),
		icon: newEnum("sovcommon.ui", "Icon", "alert_triangle=alert-triangle", "archive", "bar_chart=bar-chart", "check", "columns",
			"database", "gamepad", "globe", "hand", "inbox", "layers", "lightbulb", "lock", "monitor", "package", "scale", "server",
			"shield", "terminal"),
		role:      newEnum("sovcommon.roles", "Role", "member", "gm_junior", "gm_senior", "maintainer", "owner", "admin"),
		actor:     newEnum(tb, "Actor", "reporter", "triager", "reporter_or_triager"),
		postField: newEnum(tb, "PostField", "assignee", "fixed_in", "reason", "duplicate_of"),
		intents:   newTable(tb, "intents"), statuses: newTable(tb, "statuses"),
		areaGroups: newTable(tb, "areaGroups"), areas: newTable(tb, "areas"),
	}
	x.role.ir.Ordered = true
	x.statusRef, x.areaGroupRef, x.areaRef = refTo(x.statuses.coll), refTo(x.areaGroups.coll), refTo(x.areas.coll)
	x.layoutList = listOf(tString)
	x.intent = newRecord(tb, "Intent", fld("tone", x.tone.typ()), fld("icon", x.icon.typ()), fld("label", tString),
		fld("minRole", x.role.typ()))
	x.status = newRecord(tb, "Status", fld("tone", x.tone.typ()), fld("label", tString), fld("terminal", tBool),
		fld("next", listOf(x.statusRef)), fld("requires", listOf(x.postField.typ())), fld("optional", listOf(x.postField.typ())),
		fld("by", x.actor.typ()).opt())
	x.deck = newRecord(tb, "Deck", fld("layouts", x.layoutList), fld("maxHidden", tInt))
	x.areaGroup = newRecord(tb, "AreaGroup", fld("label", tString))
	x.area = newRecord(tb, "Area", fld("group", x.areaGroupRef), fld("tone", x.tone.typ()), fld("icon", x.icon.typ()),
		fld("label", tString), fld("hint", tString).opt(), fld("minRole", x.role.typ()), fld("routesTo", listOf(x.areaRef)))
	x.intents.of(x.intent)
	x.statuses.of(x.status)
	x.areaGroups.of(x.areaGroup)
	x.areas.of(x.area)
	return x
}

func (x *taxonomy) intentValues() *ir.Value {
	i := func(key, tone, icon, label, role string) *value.Record {
		return x.intents.entry(key, x.tone.m(tone), x.icon.m(icon), str(label), x.role.m(role))
	}
	return x.intents.value(
		i("issue", "warning", "alert_triangle", "Something is wrong", "gm_junior"),
		i("idea", "accent", "lightbulb", "Idea or request", "gm_junior"),
	)
}

func (x *taxonomy) statusValues() *ir.Value {
	fields := func(keys ...string) *value.List {
		l := list(x.postField.typ())
		for _, k := range keys {
			l.Elems = append(l.Elems, x.postField.m(k))
		}
		return l
	}
	rows := []struct {
		key, tone, label   string
		terminal           bool
		next               []string
		requires, optional *value.List
		by                 value.Value
	}{
		{"open", "warning", "Open", false, []string{"taken", "wont_do", "duplicate"}, fields(), fields(), nil},
		{"taken", "info", "Taken", false, []string{"open", "fixed", "wont_do", "duplicate"}, fields("assignee"), fields(), nil},
		{"fixed", "accent", "Fixed", false, []string{"verified", "open"}, fields(), fields("fixed_in"), nil},
		{"verified", "success", "Verified", true, []string{"open"}, fields(), fields(), x.actor.m("reporter_or_triager")},
		{"wont_do", "neutral", "Won't do", true, []string{"open"}, fields("reason"), fields(), nil},
		{"duplicate", "neutral", "Duplicate", true, []string{"open"}, fields("duplicate_of"), fields(), nil},
	}
	entries := make([]*value.Record, len(rows))
	for i, r := range rows {
		by := r.by
		if by == nil {
			by = none(x.actor.typ())
		}
		entries[i] = x.statuses.entry(r.key, x.tone.m(r.tone), str(r.label), boolean(r.terminal), x.statuses.refs(r.next...),
			r.requires, r.optional, by)
	}
	v := x.statuses.value(entries...)
	v.Schema = "teamboard.Status@4aed3fde" // FINGERPRINT.md vector 2
	return v
}

func (x *taxonomy) areaGroupValues() *ir.Value {
	return x.areaGroups.value(
		x.areaGroups.entry("in_game", str("In game")),
		x.areaGroups.entry("out_game", str("Out of game")),
		x.areaGroups.entry("internal", str("Internal")),
	)
}

// areaRows is taxonomy.canon's areas: key, group, tone, icon, label, minRole, hint, routesTo.
func areaRows() [][]string {
	return [][]string{
		{"game", "in_game", "series_1", "gamepad", "Game", "gm_junior", "Anything you saw while playing.", "game_server client resource balance"},
		{"balance", "in_game", "series_1", "scale", "Balance", "gm_junior", "Nothing is broken, the numbers feel wrong.", ""},
		{"website", "out_game", "series_2", "globe", "Website", "gm_junior", "The site, accounts, rankings, donations and payments.", ""},
		{"resource_studio", "internal", "series_3", "layers", "Resource Studio", "maintainer", "", ""},
		{"game_server", "in_game", "series_1", "server", "Game server", "admin", "", ""},
		{"client", "in_game", "series_1", "monitor", "Game client", "maintainer", "", ""},
		{"resource", "in_game", "series_1", "database", "Game data (Resource)", "maintainer", "", ""},
		{"sovcommon", "internal", "series_3", "package", "sovcommon", "admin", "", ""},
		{"admin_panel", "internal", "series_2", "shield", "Admin panel", "admin", "", ""},
		{"analytics", "internal", "series_2", "bar_chart", "Analytics panel", "admin", "", ""},
		{"team_board", "internal", "series_2", "columns", "Team board", "gm_junior", "This board itself: something broken or missing here.", ""},
		{"internal", "internal", "series_4", "terminal", "Internal (sov, boxes, deploy)", "admin", "", ""},
	}
}

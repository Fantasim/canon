package rules_test

import (
	"path"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const findingsFile = "findings.txt"

// cases builds the values and check runs of each findings case, by case name.
var cases = map[string]func(fx *fixture){
	"E5001_1": oneLineCase,
	"W5001_1": warnCase,
	"E5002_1": failCase,
	"W5002_1": blockWarnCase,
	"E5003_1": namesCase,
	"E5001_2": variantLevelCase,
	"E5003_2": variantNamesCase,
	"E5001_3": literalKeyCase,
	"E5004_1": expectNamesCase,
	"E5005_1": testNamesCase,
}

// IMPLEMENTATION-PLAN.md §7.2: each case prints the findings of stages C and D over its values.
func TestFindings(t *testing.T) {
	seen := map[string]bool{}
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		name := strings.TrimSuffix(path.Base(c.Path), path.Ext(c.Path))
		build, ok := cases[name]
		if !ok {
			t.Fatalf("no values for case %s", name)
		}
		seen[name] = true
		fx := fromArchive(t, c.Archive)
		build(fx)
		fx.run()
		return fx.render()
	}, golden.Expected(findingsFile))
	for name := range cases {
		if !seen[name] {
			t.Errorf("case %s has no testdata/findings/%s.txtar", name, name)
		}
	}
}

func oneLineCase(fx *fixture) {
	nonEmpty, short, ratio := fx.check("check not statuses"), fx.check("check short"), fx.check("check statuses.len()")
	label := fx.written("String(1..)", &types.Refined{Of: types.StringType, Range: &types.Bound{Lo: types.Limit{I: 1}, HasLo: true}})
	column := record("Column", field("label", label), field("statuses", &types.ListType{Elem: types.StringType}))
	column.Checks = append(column.Checks, nonEmpty, short, ratio)
	fx.typeName(column)
	table := &types.TableType{Elem: column, Stable: true}
	coll := &types.Collection{Kind: types.CollLet, Pkg: pkg, Name: "columns", Elem: column}
	entry := func(key, labelText, text string, statuses ...string) *value.Record {
		list := &value.List{T: column.Fields[1].Type, P: fx.lit("[", key+" {", "statuses: ")}
		for _, st := range statuses {
			list.Elems = append(list.Elems, str(st, fx.lit(`"`+st+`"`, key+" {", "statuses: ")))
		}
		lv := &value.Str{V: labelText, T: label, P: fx.lit(text, key+" {")}
		return fx.entry(coll, key, column, lv, list)
	}
	unclaimed := entry("unclaimed", "Unclaimed", `"Unclaimed"`)
	archive := entry("archive", "Everything archived", `"Everything archived"`, "done")
	broken := entry("broken", "", `"" + ""`)
	flaky := entry("flaky", "Flaky", `"Flaky"`, "x")
	fx.ev.scripts[nonEmpty] = failsFor("a column holds at least one status", unclaimed, broken)
	fx.ev.scripts[short] = failsFor("the label is too long", archive)
	fx.ev.scripts[ratio] = func(self value.Value) rules.Run {
		if self != flaky {
			return rules.Run{}
		}
		at := fx.at("statuses.len() / (label.len() - 5)")
		diag.E4102.At(at).Report(fx.bag)
		return rules.Run{Aborted: true}
	}
	fx.ev.scripts[fx.check("check columns")] = fails("the deck has exactly one column")
	fx.let("columns", &value.Table{T: table, Entries: []*value.Record{unclaimed, archive, broken, flaky}, P: fx.lit("{", "let columns")})
}

func warnCase(fx *fixture) {
	few := fx.check("warn few")
	deck := record("Deck", field("maxHidden", types.IntType))
	deck.Checks = append(deck.Checks, few)
	fx.typeName(deck)
	fx.ev.scripts[few] = fails("maxHidden is high")
	fx.ev.scripts[fx.check("warn deck")] = fails("maxHidden above 3")
	five := &value.Int{V: 5, T: types.IntType, P: fx.lit("5")}
	fx.let("deck", &value.Record{T: deck, Fields: []value.Value{five}, P: fx.lit("{", "let deck")})
}

// statusTable is `let statuses: stable table Status` of the E5002 and W5002 cases.
func statusTable(fx *fixture, status *types.RecordType, keys ...string) (*value.Table, map[string]*value.Record) {
	coll := &types.Collection{Kind: types.CollLet, Pkg: pkg, Name: "statuses", Elem: status}
	tv := &value.Table{T: &types.TableType{Elem: status, Stable: true}, P: fx.lit("{", "let statuses")}
	byKey := map[string]*value.Record{}
	for i := 0; i < len(keys); i += 2 {
		e := fx.entry(coll, keys[i], status, str(keys[i+1], fx.lit(`"`+keys[i+1]+`"`)))
		tv.Entries = append(tv.Entries, e)
		byKey[keys[i]] = e
	}
	return tv, byKey
}

func failCase(fx *fixture) {
	status := record("Status", field("label", types.StringType))
	fx.typeName(status)
	tv, s := statusTable(fx, status, "open", "Open", "taken", "Taken", "fixed", "Fixed")
	fx.ev.scripts[fx.check("check {\n  for s in statuses.active() {\n    if")] = func(value.Value) rules.Run {
		return rules.Run{Reports: []rules.Report{
			{At: s["taken"], Message: "status taken is not claimed by any column"},
			{At: s["fixed"].Fields[0], Message: "label Fixed is never shown"},
		}}
	}
	fx.ev.scripts[fx.check("check {\n  for s in statuses.active() {\n    fail")] = func(value.Value) rules.Run {
		at := fx.at("statuses.len() / 0")
		diag.E4102.At(at).Report(fx.bag)
		var reports []rules.Report
		for _, e := range tv.Entries {
			reports = append(reports, rules.Report{At: e, Message: e.Fields[0].CanonText() + " is never shown"})
		}
		return rules.Run{Aborted: true, Reports: reports}
	}
	fx.let("statuses", tv)
}

func blockWarnCase(fx *fixture) {
	block := fx.check("check {")
	status := record("Status", field("label", types.StringType))
	status.Checks = append(status.Checks, block)
	fx.typeName(status)
	tv, s := statusTable(fx, status, "open", "Open", "verified", "Verified and closed")
	fx.ev.scripts[block] = func(self value.Value) rules.Run {
		if self != s["verified"] {
			return rules.Run{}
		}
		return rules.Run{Reports: []rules.Report{{Warn: true, At: s["verified"].Fields[0], Message: "label Verified and closed is long"}}}
	}
	fx.let("statuses", tv)
}

func namesCase(fx *fixture) {
	column := record("Column", field("label", types.StringType))
	column.Checks = append(column.Checks, fx.check("check short: label.len()"), fx.check(`check short: label != ""`))
	reward := &types.VariantType{Pkg: pkg, Name: "Reward"}
	item := &types.CaseType{Variant: reward, Name: "item", Fields: []*types.Field{field("count", types.IntType)}}
	item.Checks = append(item.Checks, fx.check("check positive: count > 0"), fx.check("check positive: count < 100"))
	reward.Cases = []*types.CaseType{item}
	fx.typeName(column)
	fx.typeName(reward)
}

// rewardVariant is the fixture's variant Reward, declared by its source, with cases of the given fields.
func rewardVariant(fx *fixture, cases map[string][]*types.Field, order ...string) (*types.VariantType, map[string]*types.CaseType) {
	fx.t.Helper()
	reward := &types.VariantType{Pkg: pkg, Name: "Reward"}
	for _, d := range fx.file.Decls {
		if vd, ok := d.(*syntax.VariantDecl); ok {
			reward.Decl = vd
		}
	}
	byName := map[string]*types.CaseType{}
	for i, name := range order {
		ct := &types.CaseType{Variant: reward, Name: name, Index: i, Fields: cases[name]}
		reward.Cases = append(reward.Cases, ct)
		byName[name] = ct
	}
	fx.typeName(reward)
	return reward, byName
}

func variantLevelCase(fx *fixture) {
	reward, cs := rewardVariant(fx, map[string][]*types.Field{"gold": {field("amount", types.IntType)}}, "gold", "nothing")
	capped, spent := fx.check("check capped"), fx.check("check spent")
	cs["gold"].Checks = []*syntax.CheckDecl{capped}
	amount := &value.Int{V: 500, T: types.IntType, P: fx.lit("500")}
	gold := &value.Record{T: cs["gold"], Fields: []value.Value{amount}, P: fx.lit("gold {", "let rewards")}
	nothing := &value.Record{T: cs["nothing"], P: fx.lit("nothing", "let rewards")}
	fx.ev.scripts[spent] = failsFor("an unspent reward", gold)
	fx.ev.scripts[capped] = failsFor("too much gold", gold)
	list := &value.List{T: &types.ListType{Elem: reward}, Elems: []value.Value{gold, nothing}, P: fx.lit("[", "let rewards")}
	fx.let("rewards", list)
}

func variantNamesCase(fx *fixture) {
	_, cs := rewardVariant(fx, map[string][]*types.Field{"item": {field("count", types.IntType)}, "gold": {field("amount", types.IntType)}}, "item", "gold")
	cs["item"].Checks = []*syntax.CheckDecl{fx.check("check positive: count"), fx.check("check wide: count")}
}

// literalKeyCase is API.md P9: the literal of a literal-union map key is a JSON string in a path.
func literalKeyCase(fx *fixture) {
	fits := fx.check("check fits")
	slot := record("Slot", field("size", types.IntType))
	slot.Checks = []*syntax.CheckDecl{fits}
	fx.typeName(slot)
	keyType := &types.LitUnionType{Of: types.StringType, Literals: []string{"none"}}
	slotOf := func(key, line string, size int64, text string) (value.Value, value.Value) {
		k := str(key, fx.lit(key, "let slots", line))
		n := &value.Int{V: size, T: types.IntType, P: fx.lit(text, "let slots", line)}
		return k, &value.Record{T: slot, Fields: []value.Value{n}, P: fx.lit("{", "let slots", line)}
	}
	noneKey, noneSlot := slotOf("none", `"none": {`, 12, "12")
	otherKey, otherSlot := slotOf("other", "other: {", 14, "14")
	fx.ev.scripts[fits] = fails("the slot is too big")
	m := &value.Map{T: &types.MapType{Key: keyType, Value: slot}, Keys: []value.Value{noneKey, otherKey}, Vals: []value.Value{noneSlot, otherSlot}, P: fx.lit("{", "let slots")}
	fx.let("slots", m)
}

// testNamesCase is EVALUATION.md §10.1: the second test of a name is E5005, at its name.
func testNamesCase(*fixture) {}

// expectNamesCase is EVALUATION.md §10.3: E5004 for a name no reachable type declares.
func expectNamesCase(fx *fixture) {
	slot := record("Slot", field("size", types.IntType))
	slot.Checks = append(slot.Checks, fx.check("check fits"))
	deck := record("Deck", field("slots", &types.ListType{Elem: slot}))
	fx.typeName(slot)
	fx.typeName(deck)
	shape, cs := rewardVariant(fx, map[string][]*types.Field{"sq": {field("side", types.IntType)}}, "sq", "circle")
	cs["sq"].Checks = []*syntax.CheckDecl{fx.check("check sqOk")}
	box := &types.TypeFunc{Pkg: pkg, Name: "Box", Scrutinee: &types.Scrutinee{}, Arms: []*types.TypeArm{{Wildcard: true, Result: slot}}}
	holder := record("Holder", field("b", &types.TypeAppType{Fn: box}))
	fx.typeName(holder)
	subjects := map[string]types.Type{
		"deck": deck, "slot": slot, "shape": shape, "sq": cs["sq"], "circle": cs["circle"],
		"maybe": &types.OptionalType{Elem: slot}, "byName": &types.MapType{Key: types.StringType, Value: slot}, "holder": holder,
	}
	for text, t := range subjects { //canon:unordered each subject is recorded independently
		fx.subject(text, t)
	}
}

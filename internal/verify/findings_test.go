package verify_test

import (
	"math"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const findingsFile = "findings.txt"

// cases builds the values of each findings case, by case name, and verifies them.
var cases = map[string]func(fx *fixture){
	"E3102_1":  keyedListCase,
	"E3102_2":  stableValueCase,
	"E3102_3":  codesCase,
	"E3202_1":  floatCase,
	"E3204_1":  rangeCase,
	"E3205_1":  patternCase,
	"E3206_1":  whereCase,
	"E3501_1":  danglingCase,
	"E3502_1":  retiredRefCase,
	"E3506_1":  retiredUseCase,
	"E3701_1":  assetCase(`sword.png`, `Shield.png`),
	"E3701_2":  builtCase,
	"E3705_1":  builtCase,
	"E3702_1":  assetCase(`sword.png`, `shield.jpg`),
	"E3703_1":  assetCase(`../sword.png`, `Icon//shield.png`),
	"E3201_1":  builtCase,
	"E3302_1":  builtCase,
	"E3322_1":  builtCase,
	"E3501_2":  builtCase,
	"E3801_1":  builtCase,
	"E3802_1":  builtCase,
	"E3802_2":  builtCase,
	"E3801_2":  builtCase,
	"E3802_3":  builtCase,
	"E3802_4":  builtCase,
	"E3801_3":  builtCase,
	"E3802_5":  builtCase,
	"E3503_1":  builtCase,
	"E3802_6":  builtCase,
	"E3506_2":  builtCase,
	"E3502_2":  builtCase,
	"E3501_3":  builtCase,
	"E3502_3":  builtCase,
	"E3502_4":  builtCase,
	"E3802_7":  builtCase,
	"E3317_1":  builtCase,
	"E3802_8":  builtCase,
	"E3501_4":  builtCase,
	"E3501_5":  builtCase,
	"E3501_6":  builtCase,
	"E3502_5":  builtCase,
	"E5001_9":  builtCase,
	"E5001_8":  builtCase,
	"E5002_9":  builtCase,
	"E3102_4":  builtCase,
	"E3501_7":  builtCase,
	"E8102_1":  builtCase,
	"E8102_2":  builtCase,
	"E8102_3":  builtCase,
	"E8102_4":  builtCase,
	"E8102_5":  builtCase,
	"E8102_6":  builtCase,
	"E3502_6":  builtCase,
	"E3502_7":  builtCase,
	"E3506_3":  builtCase,
	"E3506_4":  builtCase,
	"E3506_5":  builtCase,
	"E3502_8":  builtCase,
	"E3502_9":  orderedCase,
	"E3506_6":  builtCase,
	"E3502_10": builtCase,
	"E3801_4":  builtCase,
	"E3802_9":  builtCase,
	"E3501_8":  orderedCase,
}

// IMPLEMENTATION-PLAN.md §7.2: each case prints the findings of verifying its values.
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
		if fx.out != nil {
			return fx.out
		}
		return fx.render()
	}, golden.Expected(findingsFile))
	for name := range cases {
		if !seen[name] {
			t.Errorf("case %s has no testdata/findings/%s.txtar", name, name)
		}
	}
}

// TYPES.md §9.1: a keyed list's keys are unique.
func keyedListCase(fx *fixture) {
	level := record("Level", field("level", types.IntType), field("cost", types.IntType))
	levels := fx.written("[Level] keyed by level", &types.ListType{Elem: level, KeyedBy: level.Fields[0]})
	farm := record("Farm", field("levels", levels))
	coll := &types.Collection{Kind: types.CollField, Owner: farm, FieldPath: []string{"levels"}, Elem: level}
	elem := func(within, keyText string, key, cost int64) *value.Record {
		return &value.Record{
			T: level, Ident: &value.Identity{Coll: coll, Key: value.Key{I: key, IsInt: true}},
			Fields: []value.Value{
				integer(key, types.IntType, fx.lit(keyText, within)),
				integer(cost, types.IntType, fx.lit(strconv.FormatInt(cost, 10), within)),
			},
			P: fx.lit("{", within),
		}
	}
	list := &value.List{T: levels, P: fx.lit("[", "let farm", "levels: ["), Elems: []value.Value{
		elem("{ level: 1, cost: 10 }", "1", 1, 10), elem("{ level: 2, cost: 20 }", "2", 2, 20),
		elem("{ level: 0 + 1, cost: 30 }", "0 + 1", 1, 30),
	}}
	fx.let("farm", farm, &value.Record{T: farm, Fields: []value.Value{list}, P: fx.lit("{", "let farm")})
	fx.verify("farm")
}

// LOCK.md §1: a @stable value is unique among the entries, retired ones included.
func stableValueCase(fx *fixture) {
	ev := record("EventType", field("code", types.UInt16Type), field("label", types.StringType))
	ev.Fields[0].Stable = true
	table := fx.written("stable table EventType", &types.TableType{Elem: ev, Stable: true})
	coll := collection("eventTypes", ev)
	entry := func(key string, code int64, label string) *value.Record {
		return fx.entry(coll, key, ev,
			integer(code, types.UInt16Type, fx.lit(string(rune('0'+code)), key+" {")),
			str(label, types.StringType, fx.lit(`"`+label+`"`)))
	}
	retired := entry("KILL_GIANT", 1, "Kill a giant")
	retired.Ident.Retired = true
	fx.let("eventTypes", table, &value.Table{T: table.(*types.TableType), P: fx.lit("{", "let eventTypes"), Entries: []*value.Record{
		entry("KILL_MONSTER", 0, "Kill a monster"), retired, entry("KILL_RAID", 1, "Kill a raid boss"),
	}})
	fx.verify("eventTypes")
}

// TYPES.md §8.1: the codes of a @codes enum are unique, retired members included.
func codesCase(fx *fixture) {
	var decl *syntax.EnumDecl
	for _, d := range fx.file.Decls {
		if e, ok := d.(*syntax.EnumDecl); ok && e.Name.Name == "Element" {
			decl = e
		}
	}
	member := func(name string, code int64, retired bool) *types.Member {
		return &types.Member{Name: name, Code: code, HasCode: true, Retired: retired}
	}
	u8 := types.UInt8Type
	enum := &types.EnumType{Pkg: pkg, Name: "Element", Codes: &u8, Decl: decl, Members: []*types.Member{
		member("FIRE", 1, false), member("WIND", 4, true), member("WATER", 2, false),
		member("STORM", 4, false), member("BLAZE", 1, false),
	}}
	for i, m := range enum.Members {
		m.Index = i
	}
	plain := &types.EnumType{Pkg: pkg, Name: "Tone", Members: []*types.Member{member("warm", 1, false), member("cold", 1, false)}}
	v := fx.verifier()
	for _, obj := range []*object{
		{kind: check.ObjTypeName, name: "Element", typ: enum, decl: decl, file: fx.file},
		{kind: check.ObjTypeName, name: "Tone", typ: plain, file: fx.file},
	} {
		if _, err := v.Codes(obj); err != nil {
			fx.t.Fatal(err)
		}
	}
}

// TYPES.md §7.3: a finite product stored into a Float32 that overflows binary32.
func floatCase(fx *fixture) {
	tuning := record("Tuning", field("scale", types.Float32Type), field("base", types.Float32Type))
	fx.let("tuning", tuning, &value.Record{T: tuning, P: fx.lit("{", "let tuning"), Fields: []value.Value{
		&value.Float{V: 1e39, T: types.Float32Type, P: fx.computed("big * 10.0")},
		&value.Float{V: math.MaxFloat32, T: types.Float32Type, P: fx.lit("3.4028234e38")},
	}})
	fx.verify("tuning")
}

// TYPES.md §7.4: ranges on strings (bytes), lists (elements) and integers.
func rangeCase(fx *fixture) {
	label := fx.written("String(1..=8)", &types.Refined{Of: types.StringType, Range: &types.Bound{
		Lo: types.Limit{I: 1}, HasLo: true, Hi: types.Limit{I: 8}, HasHi: true, HiIncluded: true,
	}})
	group := record("AreaGroup", field("label", label))
	groups := &types.TableType{Elem: group, Stable: true}
	coll := collection("areaGroups", group)
	fx.let("areaGroups", groups, &value.Table{T: groups, P: fx.lit("{", "let areaGroups"), Entries: []*value.Record{
		fx.entry(coll, "in_game", group, str("In game", label, fx.lit(`"In game"`))),
		fx.entry(coll, "out_game", group, str("Out of game", label, fx.computed(`"Out of " + "game"`))),
	}})
	layouts := fx.written("[String](1..)", &types.Refined{Of: &types.ListType{Elem: types.StringType}, Range: from(1)})
	hidden := fx.written("Int(1..4)", &types.Refined{Of: types.IntType, Range: &types.Bound{
		Lo: types.Limit{I: 1}, HasLo: true, Hi: types.Limit{I: 4}, HasHi: true,
	}})
	deck := record("Deck", field("layouts", layouts), field("maxHidden", hidden))
	fx.let("deck", deck, &value.Record{T: deck, P: fx.lit("{", "let deck"), Fields: []value.Value{
		&value.List{T: layouts, P: fx.inCall("[]", "noLayouts", "noLayouts()", "let deck")},
		integer(4, hidden, fx.computed("2 + 2")),
	}})
	fx.verify("areaGroups", "deck")
}

// TYPES.md §7.4: a regex refinement is an RE2 search.
func patternCase(fx *fixture) {
	const re = `^[1-9]:[1-9]$`
	layout := &types.Alias{Pkg: pkg, Name: "Layout", Def: fx.written("String(/"+re+"/)", &types.Refined{
		Of: types.StringType, Pattern: regexp.MustCompile(re),
	})}
	nick := fx.written("String(/[a-z]/)", &types.Refined{Of: types.StringType, Pattern: regexp.MustCompile(`[a-z]`)})
	layouts := &types.ListType{Elem: layout}
	deck := record("Deck", field("layouts", layouts), field("nick", nick))
	fx.let("deck", deck, &value.Record{T: deck, P: fx.lit("{", "let deck"), Fields: []value.Value{
		&value.List{T: layouts, P: fx.lit(`["4:1"`), Elems: []value.Value{
			str("4:1", layout, fx.lit(`"4:1"`)), str("2x2", layout, fx.computed(`"2" + "x2"`)),
		}},
		str("ABc", nick, fx.computed(`"AB" + "c"`)),
	}})
	fx.verify("deck")
}

// TYPES.md §7.4: a `where` predicate that is false.
func whereCase(fx *fixture) {
	count := fx.written("Int where it % 5 == 0", &types.Refined{Of: types.IntType, Where: &types.Predicate{Text: "it % 5 == 0"}})
	reward := record("Reward", field("count", count))
	fx.let("reward", reward, &value.Record{T: reward, P: fx.lit("{", "let reward"), Fields: []value.Value{
		integer(7, count, fx.computed("3 + 4")),
	}})
	fx.verify("reward")
}

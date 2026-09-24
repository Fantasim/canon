package wire_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// vocab is resource.vocab's event types and their `Param(e)` (vocab.canon), with the entries
// the samples name, served by the host.
type vocab struct {
	eventTypes, items *types.Collection
	param             *types.TypeFunc
	eventType         *types.RecordType
}

func newVocab(h *host) vocab {
	kind := enumWires("resource.vocab", "ParamKind", "none_", "none_", "monster", "monster", "item", "item",
		"game_mode", "gameMode", "stat", "stat")
	e := record("resource.vocab", "EventType", field("code", types.UInt16Type), field("param", kind))
	v := vocab{eventType: e}
	v.eventTypes = &types.Collection{Kind: types.CollLet, Pkg: "resource.vocab", Name: "eventTypes", Elem: e}
	v.items = &types.Collection{Kind: types.CollLet, Pkg: "resource.vocab", Name: "items"}
	monsters := &types.Collection{Kind: types.CollDefines, Pkg: "resource.vocab", Name: "monsters"}
	p := &types.Param{Name: "e", Type: e}
	v.param = &types.TypeFunc{Pkg: "resource.vocab", Name: "Param", Params: []*types.Param{p},
		Scrutinee: &types.Scrutinee{Param: p, Path: []*types.Field{e.Fields[1]}, Type: kind},
		Arms: []*types.TypeArm{
			{Members: []int{1}, Result: &types.RefType{Target: monsters}},
			{Members: []int{2}, Result: &types.RefType{Target: v.items}},
			{Members: []int{3}, Result: types.StringType},
			{Members: []int{0, 4}, Result: types.NeverType},
		}}
	for i, name := range []string{"COMBAT_KILL_MONSTER", "ECONOMY_DROP_ITEM", "COMBAT_KILL_FFA", "COMBAT_KILL_GIANT", "COMBAT_GAME_MODE_START"} {
		param := []int{1, 2, 0, 1, 3}[i]
		h.entries[v.eventTypes] = append(h.entries[v.eventTypes],
			entry(v.eventTypes, name, false, rec(e, &value.Int{V: int64(i), T: types.UInt16Type}, member(kind, param))))
	}
	return v
}

// heistia is resource.heistia's HeistiaConfig (heistia.canon).
func heistia(h *host, v vocab) (*types.RecordType, *types.RecordType) {
	eventType := field("eventType", &types.RefType{Target: v.eventTypes})
	filter := field("filterParam", opt(&types.TypeAppType{Fn: v.param, Args: []*types.Arg{{Source: types.ArgField, Path: []*types.Field{eventType}}}}))
	filter.NoneWire = []byte(`""`)
	h.withDefault(filter, none(filter.Type))
	maxDuration := field("maxDuration", types.DurationType, "maxDurationMin")
	maxDuration.Unit = types.UnitM
	task := record("resource.heistia", "Task", eventType, filter, field("targetPerPlayer", types.IntType), maxDuration,
		field("description", types.StringType))
	reward := record("resource.heistia", "Reward", field("item", &types.RefType{Target: v.items}, "itemId"),
		field("quantity", types.IntType), field("weight", types.IntType))
	duration := field("duration", types.DurationType, "durationSec")
	duration.Unit = types.UnitS
	buff := record("resource.heistia", "Buff", field("buffResId", types.IntType), duration, field("weight", types.IntType))
	config := record("resource.heistia", "HeistiaConfig", field("version", types.IntType), field("tasks", listOf(task)),
		h.withDefault(field("rewardPools", listOf(listOf(reward))), list(listOf(reward))),
		h.withDefault(field("buffPool", listOf(buff)), list(buff)))
	return config, task
}

var taskCases = []struct{ json, want string }{
	{`{"eventType": "COMBAT_KILL_FFA", "filterParam": "", "targetPerPlayer": 1, "maxDurationMin": 1, "description": "d"}`, "none"},
	{`{"eventType": "COMBAT_KILL_FFA", "filterParam": "MI_AIBATT1", "targetPerPlayer": 1, "maxDurationMin": 1, "description": "d"}`, "symbol MI_AIBATT1"},
	{`{"eventType": "ECONOMY_DROP_ITEM", "filterParam": "II_GEN_MAT_MOONSTONE", "targetPerPlayer": 1, "maxDurationMin": 1, "description": "d"}`, "ref II_GEN_MAT_MOONSTONE"},
	{`{"eventType": "COMBAT_GAME_MODE_START", "filterParam": "pvp", "targetPerPlayer": 1, "maxDurationMin": 1, "description": "d"}`, "string pvp"},
	{`{"eventType": "ECONOMY_DROP_ITEM", "filterParam": 5, "targetPerPlayer": 1, "maxDurationMin": 1, "description": "d"}`, at(diag.E7110, "1:51", "/filterParam")},
	{`{"eventType": "COMBAT_KILL_FFA", "filterParam": {"a": [1, true]}, "targetPerPlayer": 1, "maxDurationMin": 1, "description": "d"}`, `symbol {"a": [1, true]}`},
	{`{"eventType": "COMBAT_KILL_FFA", "filterParam": null, "targetPerPlayer": 1, "maxDurationMin": 1, "description": "d"}`, "none"},
}

// WIRE.md §5.9, §9: a dependent value reads as its branch, a symbol on Never (DECISIONS 175).
func TestDecodeDependent(t *testing.T) {
	h := newHost()
	_, task := heistia(h, newVocab(h))
	for _, c := range taskCases {
		got := decodeJSON(t, wire.Decoder{Host: h}, c.json, task)
		if text := branchText(got); text != c.want {
			t.Errorf("%s: %q, want %q", c.json, text, c.want)
		}
	}
	got := decodeJSON(t, wire.Decoder{Host: h}, `{"eventType": "NOPE", "filterParam": "x", "targetPerPlayer": 1, "maxDurationMin": 1, "description": "d"}`, task)
	if got.ok || len(got.findings) != 0 {
		t.Errorf("an unknown discriminant is the host's to report: %q", got.text())
	}
	if got = decodeJSON(t, wire.Decoder{}, taskCases[2].json, task); !errors.Is(got.err, wire.ErrNoHost) {
		t.Errorf("a dereference without a host is a misuse")
	}
}

// branchText names what filterParam decoded as.
func branchText(d decoded) string {
	if !d.ok {
		return d.text()
	}
	switch x := d.v.(*value.Record).Fields[1].(type) {
	case *value.None:
		return "none"
	case *value.Symbol:
		return "symbol " + x.Name
	case *value.Ref:
		return "ref " + x.Key.Text()
	case *value.Str:
		return "string " + x.V
	}
	return d.text()
}

// WIRE.md §9: the decode samples the document lists, with the pipeline's Potion.
func TestDecodeSamples(t *testing.T) {
	h := newHost()
	potion := potionFixture(h)
	body := `"dwID": "II_POT_HEAL_S", "szName": "IDS_PROPITEM_TXT_POT_S", "nHeal": 500, "dwCooldownMs": 3000`
	cases := []struct{ json, want string }{
		{`{` + body + `}`, `Potion{id: "II_POT_HEAL_S", name: "IDS_PROPITEM_TXT_POT_S", heal: 500, cooldown: 3s, stack: 99}`},
		{`{` + body + `, "nStack": null}`, at(diag.E3315, "1:109", "/nStack")},
		{`{` + body + `, "nstack": 5}`, at(diag.E3301, "1:99", "/nstack")},
		{`{"dwID": "II_POT_T", "szName": "N", "nHeal": 1e3, "dwCooldownMs": 1}`, at(diag.E7103, "1:46", "/nHeal")},
		{`{"dwID": "X", "szName": "N", "nHeal": 1, "dwCooldownMs": 1500.5}`, at(diag.E3203, "1:58", "/dwCooldownMs")},
		{"\ufeff{" + body + `}`, `Potion{id: "II_POT_HEAL_S", name: "IDS_PROPITEM_TXT_POT_S", heal: 500, cooldown: 3s, stack: 99}`},
	}
	for _, c := range cases {
		if got := decodeJSON(t, wire.Decoder{Host: h}, c.json, potion).text(); got != c.want {
			t.Errorf("%s: %q, want %q", c.json, got, c.want)
		}
	}
}

// readJSON parses an example file.
func readJSON(t *testing.T, fs *source.FileSet, bag *diag.Bag, path string) *jsonsrc.Node {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := fs.Add(filepath.ToSlash(path), path, b)
	if err != nil {
		t.Fatal(err)
	}
	root, err := jsonsrc.Parse(f, bag)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// WIRE.md §6.5, §8.3 (GEN-01): the pipeline's data decodes, then encodes as its golden.
func TestDecodePipelineData(t *testing.T) {
	h := newHost()
	potion := potionFixture(h)
	fs := &source.FileSet{}
	bag := diag.NewBag(fs, "pipeline")
	paths, _ := filepath.Glob("../../examples/pipeline/data/*.json")
	var files []wire.File
	for _, p := range paths {
		files = append(files, wire.File{Sel: wire.Selection{Node: readJSON(t, fs, bag, p)}})
	}
	potions := keyed(potion, 0)
	dec := wire.Decoder{Bag: bag, Pkg: "pipeline", Host: h}
	v, ok, err := dec.Dir(context.Background(), files, potions)
	if !ok || err != nil || len(paths) != 2 {
		t.Fatalf("pipeline data: %v %v %v", ok, err, short(fs, bag.Findings()))
	}
	isStrong := methods(potion, "isStrong", func(r *value.Record) value.Value {
		return boolean(r.Fields[2].(*value.Int).V >= 2000)
	})
	got := encode(t, &wire.Document{Schema: "pipeline.Potion@f750790e", Kind: types.List, V: v, Methods: isStrong})
	want, err := os.ReadFile("../../examples/pipeline/expected/potions.json")
	if err != nil || got != string(want) {
		t.Errorf("decode then encode:\n%s\nwant:\n%s", got, want)
	}
	if id := v.(*value.List).Elems[1].(*value.Record).Ident; id == nil || id.Key.S != "II_POT_HEAL_S" {
		t.Errorf("keyed-list identity %+v", id)
	}
}

// decodeFile decodes one example file, optionally at a top-level key, and fails on a finding.
func decodeFile(t *testing.T, dec wire.Decoder, path, at string, typ types.Type) value.Value {
	t.Helper()
	fs := &source.FileSet{}
	dec.Bag = diag.NewBag(fs, "p")
	root := readJSON(t, fs, dec.Bag, path)
	for _, m := range root.Members {
		if m.Key == at {
			root = m.Value
		}
	}
	v, ok, err := dec.Decode(context.Background(), wire.Selection{Node: root}, typ)
	if !ok || err != nil {
		var b bytes.Buffer
		_ = diag.Render(&b, fs, dec.Bag.Findings(), diag.RenderOptions{Golden: true})
		t.Fatalf("%s: %v\n%s", path, err, b.String())
	}
	return v
}

// The committed example data decodes against types mirroring its declarations.
func TestDecodeExampleData(t *testing.T) {
	h := newHost()
	v := newVocab(h)
	config, _ := heistia(h, v)
	got := decodeFile(t, wire.Decoder{Host: h}, "../../examples/_fixtures/resource/Server/System/heistia_config.json", "", config).CanonText()
	for _, want := range []string{"filterParam: none", "filterParam: II_GEN_MAT_MOONSTONE", "maxDuration: 2h", "duration: 1h", "rewardPools: [[Reward{item: II_SYS_SYS_SCR_FAERY_EXP_5000"} {
		if !strings.Contains(got, want) {
			t.Errorf("heistia_config.json lacks %q:\n%s", want, got)
		}
	}
	events := record("resource.events", "EventConfig", field("version", types.IntType), field("events", keyed(eventFixture(h), 0)))
	got = decodeFile(t, wire.Decoder{Host: h}, "../../examples/_fixtures/resource/Server/Event/EventConfig.json", "", events).CanonText()
	for _, want := range []string{"monsterLifetime: 30m", "spawnRegion: Rect{left: 6800, top: 3200", "itemCount: [1, 2], levelMin: 20", "rollMode: authoritative", "day: Sat"} {
		if !strings.Contains(got, want) {
			t.Errorf("EventConfig.json lacks %q:\n%s", want, got)
		}
	}
	tree, _ := talentFixture()
	got = decodeFile(t, wire.Decoder{Partial: true}, "../../examples/_fixtures/resource/Server/System/GuildTalentTree.json", "", tree).CanonText()
	if want := "GuildTalentTree{nodes: [TalentNode{id: 0, parent: none}, TalentNode{id: 1, parent: 0}, TalentNode{id: 2, parent: 1}, TalentNode{id: 3, parent: 0}]}"; got != want {
		t.Errorf("GuildTalentTree.json: %s", got)
	}
}

// hourly is adventurequest's `{e in eventTypes: HourlyTarget(e)}` with a Bool match added.
func hourly(v vocab) types.Type {
	e := &types.Param{Name: "e", Type: v.eventType}
	k := &types.Param{Name: "e", Type: v.eventType}
	specific := &types.TypeFunc{Pkg: "resource.adventurequest", Name: "SpecificKey", Params: []*types.Param{k},
		Body: &types.LitUnionType{Of: &types.TypeAppType{Fn: v.param, Args: []*types.Arg{{Source: types.ArgParam, Param: k}}}, Literals: []string{"default"}}}
	b := &types.Param{Name: "on", Type: v.eventType}
	flagged := &types.TypeFunc{Pkg: "p", Name: "Flagged", Params: []*types.Param{b},
		Scrutinee: &types.Scrutinee{Param: b, Path: []*types.Field{v.eventType.Fields[2]}, Type: types.BoolType},
		Arms:      []*types.TypeArm{{Members: []int{0}, Result: types.IntType}, {Members: []int{1}, Result: types.StringType}}}
	target := record("resource.adventurequest", "HourlyTarget",
		field("rates", opt(&types.MapType{Key: &types.TypeAppType{Fn: specific, Args: []*types.Arg{{Source: types.ArgParam, Param: e}}}, Value: types.IntType})),
		field("mark", opt(&types.TypeAppType{Fn: flagged, Args: []*types.Arg{{Source: types.ArgParam, Param: e}}})))
	target.Params = []*types.Param{e}
	return &types.DepMapType{Binder: "x", Coll: v.eventTypes,
		Value: &types.AppliedRecord{Rec: target, Args: []*types.Arg{{Source: types.ArgKey, Binder: "x"}}}}
}

// WIRE.md §5.8, §5.9: dependent maps bind their key; dependent keys read as their branch.
func TestDecodeDependentMaps(t *testing.T) {
	h := newHost()
	v := newVocab(h)
	v.eventType.Fields = append(v.eventType.Fields, field("big", types.BoolType))
	v.eventType.Fields[2].Index = 2
	v.eventTypes.Elem = v.eventType
	for i, e := range h.entries[v.eventTypes] {
		e.Fields = append(e.Fields, boolean(i == 1))
	}
	typ := hourly(v)
	cases := []struct{ json, want string }{
		{`{"ECONOMY_DROP_ITEM": {"rates": {"II_A": 5, "default": 1}, "mark": "m"}}`, `{ECONOMY_DROP_ITEM: HourlyTarget{rates: {II_A: 5, "default": 1}, mark: "m"}}`},
		{`{"COMBAT_KILL_FFA": {"rates": {"x": 1}, "mark": 3}}`, `{COMBAT_KILL_FFA: HourlyTarget{rates: {x: 1}, mark: 3}}`},
		{`{"COMBAT_KILL_MONSTER": {"mark": "m"}}`, at(diag.E7110, "1:34", "/COMBAT_KILL_MONSTER/mark")},
		{`{"NOPE": {"rates": {"a": 1}}}`, ""},
	}
	for _, c := range cases {
		if got := decodeJSON(t, wire.Decoder{Host: h}, c.json, typ).text(); got != c.want {
			t.Errorf("%s: %q, want %q", c.json, got, c.want)
		}
	}
	got := decodeJSON(t, wire.Decoder{Host: h}, cases[1].json, typ)
	if _, isSymbol := got.v.(*value.Map).Vals[0].(*value.Record).Fields[0].(*value.Map).Keys[0].(*value.Symbol); !isSymbol {
		t.Errorf("a key on a Never branch stays a symbol")
	}
}

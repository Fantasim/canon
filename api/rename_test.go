package canon_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// renameCase is one successful rename of the renames example: the files it changes and a line
// each must then hold.
type renameCase struct {
	name, newName string
	files         []string
	holds         []string
	undo          string // the Undo's name
}

// renameCases are one of each name kind, on the renames example.
var renameCases = []renameCase{
	{"features.renames:Mission", "Quest",
		[]string{"features/renames/renames.canon", "features/renames/renames.fr.canon", "features/renames/renames.view.canon", "features/renames/shop/shop.canon"},
		[]string{"record Quest {", "Quest.help \"Une quête.\"", "view Quest {", "import features.renames { MAX_LEVEL, MissionLevel, Quest, Rarity, missions, xpFor }"},
		"features.renames:Quest"},
	{"features.renames:Capacity.players", "members",
		[]string{"features/renames/renames.canon", "features/renames/staging.layer.canon"},
		[]string{"  members: Int(1..) @json(\"players\")", "capacity: { members: 100, queue: 5 }", "  capacity.members: 10"},
		"features.renames:Capacity.members"},
	{"features.renames:Setup.maxPlayers", "playerCap",
		[]string{"features/renames/renames.canon"},
		[]string{"  playerCap: Int(1..) @json(\"max_players\")"},
		"features.renames:Setup.playerCap"},
	{"features.renames:Scratch.memo", "note",
		[]string{"features/renames/renames.canon"},
		[]string{"  note: String = \"\"\n", "let scratch: Scratch = { note: \"none yet\" }"},
		"features.renames:Scratch.note"},
	{"features.renames:Mission.name", "title",
		[]string{"features/renames/renames.canon", "features/renames/renames.fr.canon", "features/renames/renames.view.canon", "features/renames/store/rare/hunt.canon"},
		[]string{"  title: String(1..) @json(\"name\")", "Mission.field.title \"Nom\"", "  title \"{title}\"", "  title: \"The long hunt\""},
		"features.renames:Mission.title"},
	{"features.renames:Mission.rarity", "scarcity",
		[]string{"features/renames/renames.canon", "features/renames/renames.view.canon", "features/renames/store/rare/hunt.canon"},
		[]string{"@files(\"store/{scarcity}/{id}.canon\")", "  scarcity: Rarity @json(\"rarity\")", "  subtitle \"{scarcity}\"", "  scarcity: rare"},
		"features.renames:Mission.scarcity"},
	{"features.renames:Mission.label", "caption",
		[]string{"features/renames/renames.canon", "features/renames/shop/shop.canon"},
		[]string{"  fn caption(self) -> String", "missions.intro.caption()"},
		"features.renames:Mission.caption"},
	{"features.renames:Prize.item.count", "quantity",
		[]string{"features/renames/renames.canon", "features/renames/store/rare/hunt.canon"},
		[]string{"    quantity: Int = 1 @json(\"count\")", "item { itemId: \"relic\", quantity: 2 }"},
		"features.renames:Prize.item.quantity"},
	{"features.renames:xpFor", "experienceFor",
		[]string{"features/renames/renames.canon", "features/renames/shop/shop.canon"},
		[]string{"export fn experienceFor(level: Int", "expect experienceFor(level: 2, extra: 5) == 26", "worth: experienceFor(level: 1, extra: 5)"},
		"features.renames:experienceFor"},
	{"features.renames:xpFor.extra", "bonusXp",
		[]string{"features/renames/renames.canon", "features/renames/shop/shop.canon"},
		[]string{"xpFor(level: Int, bonusXp: Int = 0)", "level * 10 + bonusXp + bonus(level)", "xpFor(level: 1, bonusXp: 5)"},
		"features.renames:xpFor.bonusXp"},
	{"features.renames:missions", "quests",
		[]string{"features/renames/renames.canon", "features/renames/shop/shop.canon", "features/renames/store/rare/hunt.canon"},
		[]string{"let quests: table Mission = {", "values: [quests, capacity, setup, ladder]", "entry quests.hunt {", "quest: ref quests"},
		"features.renames:quests"},
	{"features/renames/renames.canon:112:9", "inner",
		[]string{"features/renames/renames.canon"},
		[]string{"    let inner = level - 10\n    return inner\n", "  let step = level - 1\n"},
		"features.renames:bonus.inner"},
	{"features.renames:MissionLevel", "Tier",
		[]string{"features/renames/renames.canon", "features/renames/shop/shop.canon"},
		[]string{"type Tier = Int(1..=99)", "  level: Tier\n"},
		"features.renames:Tier"},
	{"features.renames:MAX_LEVEL", "TOP_LEVEL",
		[]string{"features/renames/renames.canon", "features/renames/shop/shop.canon"},
		[]string{"const TOP_LEVEL = 99", "let ceiling: Int = TOP_LEVEL"},
		"features.renames:TOP_LEVEL"},
	{"features.renames:Rarity", "Scarcity",
		[]string{"features/renames/renames.canon", "features/renames/shop/shop.canon"},
		[]string{"enum Scarcity { common, rare }", "  rarity: Scarcity\n"},
		"features.renames:Scarcity"},
}

// API.md E27, API.md E33, API.md E34, API.md E37, API.md E23: each name kind renamed everywhere
// it occurs, a field on the wire given its old wire name; the Undo, one reverse RenameName,
// gives every file back byte for byte, on disk.
func TestRenameNameKinds(t *testing.T) {
	for _, c := range renameCases {
		dir := copyRenames(t)
		before := tree(t, dir)
		p := openDisk(t, dir, canon.Options{})
		res, err := renameOnce(p, c.name, c.newName)
		if err != nil {
			t.Errorf("%s -> %s: %v", c.name, c.newName, err)
			continue
		}
		after := tree(t, dir)
		if got := changedFiles(before, after); !slices.Equal(got, c.files) {
			t.Errorf("API.md E33, %s -> %s: changed %v, want %v", c.name, c.newName, got, c.files)
		}
		joined := strings.Join(slices.Collect(func(yield func(string) bool) {
			for _, f := range c.files {
				yield(after[f])
			}
		}), "\n")
		for _, h := range c.holds {
			if !strings.Contains(joined, h) {
				t.Errorf("API.md E33, E34, %s -> %s: no %q", c.name, c.newName, h)
			}
		}
		want := []canon.Op{canon.RenameName(c.undo, nameOf(c))}
		if !res.Applied || len(res.Undo) != 1 || res.Undo[0] != want[0] {
			t.Errorf("API.md E37, %s: applied %v, undo %+v, want %+v", c.name, res.Applied, res.Undo, want)
			continue
		}
		if _, err := p.Edit(context.Background(), canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
			t.Errorf("API.md E37, undo of %s: %v", c.name, err)
			continue
		}
		if got := changedFiles(before, tree(t, dir)); len(got) > 0 {
			t.Errorf("API.md E37, E22, undo of %s: %v differ", c.name, got)
		}
	}
}

// nameOf is the old name of a rename case: the last word of its name, or the local the position names.
func nameOf(c renameCase) string {
	if strings.Contains(c.name, ".canon:") {
		return "step"
	}
	return c.name[strings.LastIndexAny(c.name, ":.")+1:]
}

// API.md E27: a name naming nothing is ErrNoPath, one that does not parse ErrBadPath, a local
// declared twice ErrAmbiguousPath listing positions.
func TestRenameNameResolution(t *testing.T) {
	dir := copyRenames(t)
	p := openDisk(t, dir, canon.Options{})
	cases := []struct {
		name string
		want error
	}{
		{"features.renames:Nothing", canon.ErrNoPath},
		{"Nothing", canon.ErrNoPath},
		{"features.renames:Mission.nothing", canon.ErrNoPath},
		{"features.renames:Mission..name", canon.ErrBadPath},
		{"features.renames:bonus.step", canon.ErrAmbiguousPath},
		{"features/renames/renames.canon:113:5", canon.ErrNoPath},
		{"features/renames/renames.canon:999:1", canon.ErrNoPath},
		{"nowhere.canon:1:1", canon.ErrNoPath},
	}
	for _, c := range cases {
		_, err := renameOnce(p, c.name, "other")
		var pe *canon.PathError
		if !errors.Is(err, c.want) || !errors.As(err, &pe) || pe.Op != 0 || pe.Path != c.name {
			t.Errorf("API.md E27, %s: %v, want %v", c.name, err, c.want)
		}
	}
	_, err := renameOnce(p, "features.renames:bonus.step", "other")
	var pe *canon.PathError
	want := []string{"features/renames/renames.canon:110:7", "features/renames/renames.canon:112:9"}
	if !errors.As(err, &pe) || !slices.Equal(pe.Candidates, want) {
		t.Errorf("API.md E27: ambiguous local %v, candidates %v", err, pe)
	}
	if _, err := renameOnce(p, "Mission", "Quest"); err != nil {
		t.Errorf("API.md E27: an unprefixed public name: %v", err)
	}
}

// API.md E28, API.md E29, API.md E30: data names are ErrBadOp (ErrStableKey when stable), an
// entry key's naming Rename; what canon.lock names ErrStableKey, a new name not a word a
// *ValueError; the old name again changes nothing.
func TestRenameNameRefusals(t *testing.T) {
	dir := copyRenames(t)
	before := tree(t, dir)
	p := openDisk(t, dir, canon.Options{})
	cases := []struct {
		name, newName, rule string
		want                error
	}{
		{"features.renames:Rarity.common", "usual", "E28", canon.ErrBadOp},
		{"features.renames:Prize.gold", "coins", "E28", canon.ErrBadOp},
		{"features.renames:missions.intro", "first", "E28", canon.ErrBadOp},
		{"features.renames:ladder.bronze", "copper", "E28", canon.ErrStableKey},
		{"features.renames:ladder", "rungs", "E29", canon.ErrStableKey},
		{"features.renames:Mission", "check", "E30", canon.ErrBadValue},
		{"features.renames:Mission", "_", "E30", canon.ErrBadValue},
		{"features.renames:Mission", "9lives", "E30", canon.ErrBadValue},
		{"features.renames:Mission", "a-b", "E30", canon.ErrBadValue},
		{"features.renames:ladder", "check", "E21", canon.ErrStableKey},
	}
	for _, c := range cases {
		_, err := renameOnce(p, c.name, c.newName)
		if !errors.Is(err, c.want) {
			t.Errorf("API.md %s, %s -> %s: %v, want %v", c.rule, c.name, c.newName, err, c.want)
		}
	}
	_, err := renameOnce(p, "features.renames:missions.intro", "first")
	if err == nil || !strings.Contains(err.Error(), "Rename") {
		t.Errorf("API.md E28: the detail of an entry key names Rename: %v", err)
	}
	var ve *canon.ValueError
	if _, err := renameOnce(p, "features.renames:Mission", "check"); !errors.As(err, &ve) || ve.Expected != "a name" || ve.Op != 0 {
		t.Errorf("API.md E30: %v", err)
	}
	res, err := renameOnce(p, "features.renames:Mission", "Mission")
	if err != nil || len(res.Changes) != 0 || len(res.Undo) != 0 {
		t.Errorf("API.md E30: the old name again: %+v, %v", res, err)
	}
	if got := changedFiles(before, tree(t, dir)); len(got) > 0 {
		t.Errorf("API.md E28, E29, E30: refusals wrote %v", got)
	}
}

// API.md E31, API.md E21: a RenameName is alone in its request and without AllowErrors, else
// ErrBadOp before its name is even read; under an edit layer it is *NotEditableError reason
// layer; an Evaluate draft never holds one.
func TestRenameNameRequest(t *testing.T) {
	dir := copyRenames(t)
	p := openDisk(t, dir, canon.Options{})
	ctx := context.Background()
	rename := canon.RenameName("features.renames:Nothing", "Quest")
	cases := []canon.Edit{
		{Ops: []canon.Op{canon.Set("features.renames:scratch.memo", canon.Str("x")), rename}},
		{Ops: []canon.Op{rename, rename}},
		{Ops: []canon.Op{rename}, AllowErrors: true},
	}
	for i, e := range cases {
		var pe *canon.PathError
		if _, err := p.Edit(ctx, e); !errors.Is(err, canon.ErrBadOp) || !errors.As(err, &pe) {
			t.Errorf("API.md E31, request %d: %v", i, err)
		}
	}
	layered := openDisk(t, dir, canon.Options{EditLayer: "staging"})
	var ne *canon.NotEditableError
	if _, err := layered.Edit(ctx, canon.Edit{Ops: []canon.Op{rename}}); !errors.As(err, &ne) || ne.Reason != canon.ReasonLayer {
		t.Errorf("API.md E31: under an edit layer: %v", err)
	}
	draft := canon.EvalRequest{Path: "features.renames:missions.intro", Draft: []canon.Op{canon.RenameName("features.renames:Mission", "Quest")}}
	if _, err := p.Evaluate(ctx, draft); !errors.Is(err, canon.ErrBadOp) {
		t.Errorf("API.md E31: a draft: %v", err)
	}
}

// API.md E24, API.md E23: the JSON form of RenameName is `renameName`, its name as `path` and its
// new name as `name`; it decodes back to the same op, and one without `name` does not decode.
func TestRenameNameJSON(t *testing.T) {
	op := canon.RenameName("pipeline:Potion.heal", "hp")
	data, err := json.Marshal(op)
	if want := `{"op":"renameName","path":"pipeline:Potion.heal","name":"hp"}`; err != nil || string(data) != want {
		t.Fatalf("API.md E24: %s, %v; want %s", data, err, want)
	}
	var back canon.Op
	if err := json.Unmarshal(data, &back); err != nil || back != op || back.Kind != canon.OpRenameName {
		t.Errorf("API.md E24: decoded %+v, %v", back, err)
	}
	for _, bad := range []string{
		`{"op":"renameName","path":"a:b"}`,
		`{"op":"renameName","path":"a:b","name":"c","key":"d"}`,
		`{"op":"set","path":"a:b","name":"c","value":1}`,
	} {
		if err := json.Unmarshal([]byte(bad), &back); !errors.Is(err, canon.ErrBadOp) {
			t.Errorf("API.md E24: %s decoded: %v", bad, err)
		}
	}
}

// API.md E33 (I18N.md K4): a translation key segment gains its kind word when the renamed field
// becomes a reserved segment, keeps it from one reserved segment to another, and loses it when
// the name stops being one.
func TestRenameNameKindWord(t *testing.T) {
	dir := copyRenames(t)
	p := openDisk(t, dir, canon.Options{})
	steps := []struct{ name, newName, key string }{
		{"features.renames:Mission.name", "title", "Mission.field.title \"Nom\""},
		{"features.renames:Mission.title", "subtitle", "Mission.field.subtitle \"Nom\""},
		{"features.renames:Mission.subtitle", "heading", "Mission.heading \"Nom\""},
	}
	for _, s := range steps {
		if _, err := renameOnce(p, s.name, s.newName); err != nil {
			t.Fatalf("API.md E33, %s: %v", s.name, err)
		}
		if got := tree(t, dir)["features/renames/renames.fr.canon"]; !strings.Contains(got, s.key) {
			t.Errorf("API.md E33, I18N.md K4, %s -> %s: %q", s.name, s.newName, got)
		}
	}
}

// API.md E37, API.md M5, API.md E22: a rename that makes a line too long re-prints its item;
// the Undo then gives back every value, and every file but that item's bytes.
func TestRenameNameReprintedUndo(t *testing.T) {
	dir := copyRenames(t)
	before := tree(t, dir)
	p := openDisk(t, dir, canon.Options{})
	ctx := context.Background()
	was, err := p.Value(ctx, "features.renames:missions")
	if err != nil {
		t.Fatal(err)
	}
	res, err := renameOnce(p, "features.renames:Mission.next", "followUp")
	if err != nil {
		t.Fatalf("API.md M5: %v", err)
	}
	if !strings.Contains(tree(t, dir)["features/renames/renames.canon"], "    followUp: hunt\n") {
		t.Error("API.md M5: the entry too long for its line is not broken")
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md E37: %v", err)
	}
	now, err := p.Value(ctx, "features.renames:missions")
	if err != nil || now.Text != was.Text {
		t.Errorf("API.md E22: missions %q, was %q (%v)", now.Text, was.Text, err)
	}
	if got := changedFiles(before, tree(t, dir)); !slices.Equal(got, []string{"features/renames/renames.canon"}) {
		t.Errorf("API.md E37: changed %v, want only the re-printed item's file", got)
	}
}

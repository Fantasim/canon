package edit_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// editCase is a path, the op and edit layer asked, and the editability expected: its mode,
// reason, file, the source text its span covers, its origin and its layer.
type editCase struct {
	in     string
	op     edit.Op
	layer  string
	mode   edit.Mode
	reason edit.Reason
	file   string
	text   string
	origin string
	amends string
}

func checkEdits(t *testing.T, f fixture, cases []editCase) {
	t.Helper()
	for _, c := range cases {
		e := mustEdit(t, f, resolve(t, f, c.in), c.op, c.layer)
		text := ""
		if e.Span.File != 0 {
			text = f.text(e.Span)
		}
		if e.Mode != c.mode || e.Reason != c.reason || e.File != c.file || text != c.text || e.Origin != c.origin || e.Layer != c.amends {
			t.Errorf("%s (op %d, layer %q) = %+v %q, want %+v", c.in, c.op, c.layer, e, text, c)
		}
	}
}

// API.md W1, W4: a let's literal is a .canon source tree, a load of JSON a JSON one (load.dir
// has no one file); a load of another format is `format`, any other initializer `computed`.
func TestW1SourceTrees(t *testing.T) {
	farm := "@resource/Server/System/farm_config.json"
	checkEdits(t, examples(t, nil, "teamboard"), []editCase{
		{in: "teamboard:deck", mode: edit.ModeCanon, file: "teamboard/taxonomy.canon", text: `{ layouts: ["4:1", "2:2"], maxHidden: 2 }`},
		{in: "teamboard:VERSION", mode: edit.ModeCanon, file: "teamboard/taxonomy.canon", text: "7"},
		{in: "teamboard:assigneeMinRole", mode: edit.ModeCanon, file: "teamboard/taxonomy.canon", text: "maintainer"},
		{in: "teamboard:initialStatus", reason: edit.ReasonComputed, origin: "teamboard:columns.unclaimed.statuses[0]"},
	})
	f := examples(t, nil, "resource.farm")
	checkEdits(t, f, []editCase{
		{in: "resource.farm:farm", mode: edit.ModeJSON, file: farm, text: strings.TrimSuffix(string(f.files.Content(resolve(t, f, "resource.farm:farm").Target.Prov().Span.File)), "\n")},
		{in: "resource.farm:farm.global.visitCost", mode: edit.ModeJSON, file: farm, text: "100000"},
		{in: "resource.farm:texts", reason: edit.ReasonFormat},
	})
	checkEdits(t, examples(t, nil, "pipeline"), []editCase{{in: "pipeline:potions", mode: edit.ModeJSON}})
	checkEdits(t, lawFixture(t, nil, "a"), []editCase{
		{in: "a:holder.data.n", mode: edit.ModeJSON, file: "a/item.json", text: "5"},
		{in: "a:holder", mode: edit.ModeCanon, file: "a/a.canon", text: `{ data: load("item.json") }`},
		{in: "a:notes", reason: edit.ReasonFormat},
		{in: "a:computed", reason: edit.ReasonComputed},
		{in: "a:shape", mode: edit.ModeCanon, file: "a/a.canon", text: "circle { r: 3 }"},
	})
}

// API.md W2: the entries `entry t.key` declares are children of t's source tree, in their own file.
func TestW2EntryDeclarations(t *testing.T) {
	checkEdits(t, lawFixture(t, nil, "a"), []editCase{
		{in: "a:entries.two.v", mode: edit.ModeCanon, file: "a/Extra/more.canon", text: "2"},
		{in: "a:entries.two", mode: edit.ModeCanon, file: "a/Extra/more.canon", text: "entry entries.two { v: 2 }"},
		{in: "a:entries.one.v", mode: edit.ModeCanon, file: "a/a.canon", text: "1"},
	})
}

// API.md W3, W4: a path whose every segment moves to a child of the same tree is editable in
// the file of that tree, at the node that states the value.
func TestW3W4StructuralPaths(t *testing.T) {
	checkEdits(t, examples(t, nil, "teamboard"), []editCase{
		{in: "teamboard:statuses.open.next[0]", mode: edit.ModeCanon, file: "teamboard/taxonomy.canon", text: "taken"},
		{in: "teamboard:statuses.fixed.optional[0]", mode: edit.ModeCanon, file: "teamboard/taxonomy.canon", text: "fixed_in"},
		{in: "teamboard:areas.game.routesTo[1]", mode: edit.ModeCanon, file: "teamboard/taxonomy.canon", text: "client"},
	})
	checkEdits(t, examples(t, nil, "resource.farm"), []editCase{
		{in: "resource.farm:farm.modelTypes[1].levels[1].productionItem", mode: edit.ModeJSON,
			file: "@resource/Server/System/farm_config.json", text: `"II_FARM_PRODUCTION_BLE"`},
	})
	checkEdits(t, examples(t, nil, "pipeline"), []editCase{
		{in: "pipeline:potions[II_POT_HEAL_L].cooldown", mode: edit.ModeJSON, file: "pipeline/data/II_POT_HEAL_L.json", text: "8000"},
	})
	checkEdits(t, lawFixture(t, nil, "a"), []editCase{
		{in: "a:byColor[green]", mode: edit.ModeCanon, file: "a/a.canon", text: "2"},
		{in: "a:tones[green].w", mode: edit.ModeCanon, file: "a/a.canon", text: "{ c: green }"},
		{in: "a:mixed[1]", mode: edit.ModeCanon, file: "a/a.canon", text: "2"},
		{in: "a:shape.r", mode: edit.ModeCanon, file: "a/a.canon", text: "3"},
	})
}

// API.md W6: a @deprecated field is editable through the API.
func TestW6DeprecatedEditable(t *testing.T) {
	checkEdits(t, examples(t, nil, "resource.farm"), []editCase{
		{in: "resource.farm:farm.global.maxModels", mode: edit.ModeJSON, file: "@resource/Server/System/farm_config.json", text: "10"},
	})
}

// API.md W7: a field its literal or JSON object omits for its default is editable by inserting
// it: the span is the literal or object it goes into.
func TestW7DefaultedFields(t *testing.T) {
	tb := examples(t, nil, "teamboard")
	open := f(tb, "teamboard:statuses.open")
	checkEdits(t, tb, []editCase{
		{in: "teamboard:statuses.open.requires", mode: edit.ModeCanon, file: "teamboard/taxonomy.canon", text: open},
		{in: "teamboard:statuses.open.by", mode: edit.ModeCanon, file: "teamboard/taxonomy.canon", text: open},
	})
	pl := examples(t, nil, "pipeline")
	checkEdits(t, pl, []editCase{
		{in: "pipeline:potions[II_POT_HEAL_S].stack", mode: edit.ModeJSON, file: "pipeline/data/II_POT_HEAL_S.json", text: f(pl, "pipeline:potions[II_POT_HEAL_S]")},
	})
}

// API.md W8: a field of a record that exists only through its default is materialized into the
// nearest literal.
func TestW8Materialize(t *testing.T) {
	checkEdits(t, examples(t, []string{"louis"}, "service.resourcestudio"), []editCase{
		{in: "service.resourcestudio:config.server.host", mode: edit.ModeCanon, file: "service/resourcestudio/resourcestudio.canon", text: "{}"},
		{in: "service.resourcestudio:config.paths.cacheDir", mode: edit.ModeCanon, file: "service/resourcestudio/resourcestudio.canon", text: "{}"},
	})
	checkEdits(t, lawFixture(t, nil, "a"), []editCase{{in: "a:base.sub.n", mode: edit.ModeCanon, file: "a/a.canon", text: "{ x: 5 }"}})
}

// API.md W9: a field written after a spread is edited in place; one the spread supplies gets an
// override in the spreading literal; below it the path goes through `base`, computed, whose own
// source is `base`'s.
func TestW9Spread(t *testing.T) {
	checkEdits(t, lawFixture(t, nil, "a"), []editCase{
		{in: "a:derived.y", mode: edit.ModeCanon, file: "a/a.canon", text: "9"},
		{in: "a:derived.x", mode: edit.ModeCanon, file: "a/a.canon", text: "{ ...base, y: 9 }"},
		{in: "a:derived.sub", mode: edit.ModeCanon, file: "a/a.canon", text: "{ ...base, y: 9 }"},
		{in: "a:derived.sub.n", reason: edit.ReasonComputed, origin: "a:base.sub.n"},
	})
}

// API.md W10: without an edit layer, a value an active layer sets is `layered`, with the layer's
// name; paths the layers do not touch are edited normally.
func TestW10Layered(t *testing.T) {
	checkEdits(t, examples(t, []string{"louis"}, "service.resourcestudio"), []editCase{
		{in: "config.server.port", reason: edit.ReasonLayered, amends: "louis"},
		{in: "config.paths.resourceRoot", reason: edit.ReasonLayered, amends: "louis"},
		{in: "config.server.port", layer: "work", reason: edit.ReasonLayered, amends: "louis"},
		{in: "config.server.readOnly", mode: edit.ModeCanon, file: "service/resourcestudio/resourcestudio.canon", text: "{}"},
	})
}

// API.md W11, W11a: with an edit layer, Set writes that layer's file (created beside the
// package's sources when missing), inside the amendment already holding the path; the root
// and a const have no amendment form.
func TestW11EditLayer(t *testing.T) {
	studio := "service/resourcestudio/"
	checkEdits(t, examples(t, []string{"louis"}, "service.resourcestudio"), []editCase{
		{in: "config.server.port", layer: "louis", mode: edit.ModeCanon, file: studio + "louis.layer.canon", text: "9000"},
		{in: "config.server.host", layer: "louis", mode: edit.ModeCanon, file: studio + "louis.layer.canon"},
		{in: "config.server.host", op: edit.OpReset, layer: "work", mode: edit.ModeCanon, file: studio + "work.layer.canon"},
		{in: "config", layer: "louis", reason: edit.ReasonLayer},
	})
	checkEdits(t, lawFixture(t, []string{"dev"}, "a"), []editCase{
		{in: "a:base.x", layer: "dev", mode: edit.ModeCanon, file: "a/dev.layer.canon", text: "7"},
		{in: "a:holder.data.n", op: edit.OpAddEntry, layer: "dev", mode: edit.ModeCanon, file: "a/dev.layer.canon", text: "1"},
		{in: "a:KS[0]", layer: "dev", reason: edit.ReasonLayer},
	})
}

// API.md §7.2 computed, the first row: a call, operator or name on the way; Origin is the own structural source.
func TestReasonComputed(t *testing.T) {
	checkEdits(t, examples(t, nil, "teamboard"), []editCase{
		{in: "teamboard:defaultSeverity", reason: edit.ReasonComputed, origin: "teamboard:severities.normal"},
	})
	checkEdits(t, lawFixture(t, []string{"dev"}, "a"), []editCase{
		{in: "a:computed", reason: edit.ReasonComputed},
		{in: "a:mixed[0]", reason: edit.ReasonComputed, origin: "a:shared"},
		{in: "a:viaName", reason: edit.ReasonComputed, origin: "a:items[a]"},
		{in: "a:viaName.n", reason: edit.ReasonComputed, origin: "a:items[a].n"},
		{in: "a:wrap.code", reason: edit.ReasonLayered, amends: "dev"},
		{in: "a:wrap.label", reason: edit.ReasonLayered, amends: "dev"},
	})
}

// API.md §7.2 format: a value of load.csv, load.defines or load.text.
func TestReasonFormat(t *testing.T) {
	checkEdits(t, examples(t, nil, "resource.farm"), []editCase{
		{in: "resource.farm:texts.TID_BLANK.value", reason: edit.ReasonFormat},
		{in: "resource.farm:texts.TID_BLANK.id", reason: edit.ReasonFormat},
	})
}

// API.md §7.2 input: an input field has no value at build time.
func TestReasonInput(t *testing.T) {
	checkEdits(t, examples(t, []string{"louis"}, "service.resourcestudio"), []editCase{
		{in: "config.gen.apiKey", reason: edit.ReasonInput},
		{in: "config.gen.apiKey", layer: "louis", reason: edit.ReasonInput},
	})
}

// API.md §7.2 key: a keyed-list element's key field; a Rename in a dependent map (E13).
func TestReasonKey(t *testing.T) {
	checkEdits(t, examples(t, nil, "resource.farm"), []editCase{{in: "resource.farm:farm.modelTypes[7].typeId", reason: edit.ReasonKey}})
	checkEdits(t, examples(t, nil, "pipeline"), []editCase{{in: "pipeline:potions[II_POT_HEAL_L].id", reason: edit.ReasonKey}})
	checkEdits(t, lawFixture(t, nil, "a"), []editCase{
		{in: "a:rows[7].code", reason: edit.ReasonKey},
		{in: "a:links[one].to", reason: edit.ReasonKey},
		{in: "a:perEntry[two]", op: edit.OpRename, reason: edit.ReasonKey},
		{in: "a:perEntry[two]", mode: edit.ModeCanon, file: "a/a.canon", text: "2"},
		{in: "a:byColor[red]", op: edit.OpRename, mode: edit.ModeCanon, file: "a/a.canon", text: "1"},
	})
}

// API.md §7.2 pseudo: `.id`, `.retired`, `.kind` (P3).
func TestReasonPseudo(t *testing.T) {
	checkEdits(t, examples(t, nil, "teamboard"), []editCase{
		{in: "teamboard:statuses.open.id", reason: edit.ReasonPseudo},
		{in: "teamboard:statuses.open.retired", reason: edit.ReasonPseudo},
	})
	checkEdits(t, lawFixture(t, nil, "a"), []editCase{{in: "a:shape.kind", reason: edit.ReasonPseudo}})
}

// API.md §7.2 order: Insert into, or Move inside, a collection ordered by file paths.
func TestReasonOrder(t *testing.T) {
	checkEdits(t, examples(t, nil, "pipeline"), []editCase{
		{in: "pipeline:potions", op: edit.OpInsert, reason: edit.ReasonOrder},
		{in: "pipeline:potions[II_POT_HEAL_S]", op: edit.OpMove, reason: edit.ReasonOrder},
	})
	checkEdits(t, lawFixture(t, nil, "a"), []editCase{
		{in: "a:entries", op: edit.OpInsert, reason: edit.ReasonOrder},
		{in: "a:filed", op: edit.OpInsert, reason: edit.ReasonOrder},
		{in: "a:entries.one", op: edit.OpMove, reason: edit.ReasonOrder},
		{in: "a:items", op: edit.OpInsert, mode: edit.ModeCanon, file: "a/a.canon", text: `[{ id: "a" }, { id: "b", n: 2 }]`},
		{in: "a:items[b]", op: edit.OpMove, mode: edit.ModeCanon, file: "a/a.canon", text: `{ id: "b", n: 2 }`},
	})
}

// API.md §7.2 layer, W11: with an edit layer, every op but Set, Reset and AddEntry is refused.
func TestReasonLayer(t *testing.T) {
	f := lawFixture(t, []string{"dev"}, "a")
	r := resolve(t, f, "a:items[b].n")
	for _, op := range []edit.Op{edit.OpAdd, edit.OpInsert, edit.OpRemove, edit.OpMove, edit.OpRename, edit.OpRetire, edit.OpUnretire, edit.OpSetCase} {
		if e := mustEdit(t, f, r, op, "dev"); e.Reason != edit.ReasonLayer || e.Mode != edit.ModeNone {
			t.Errorf("op %d with an edit layer: %+v", op, e)
		}
		if e := mustEdit(t, f, r, op, ""); e.Reason != edit.ReasonNone {
			t.Errorf("op %d without an edit layer: %+v", op, e)
		}
	}
	for _, op := range []edit.Op{edit.OpSet, edit.OpReset, edit.OpAddEntry} {
		if e := mustEdit(t, f, r, op, "dev"); e.Mode != edit.ModeCanon || e.File != "a/dev.layer.canon" {
			t.Errorf("op %d with an edit layer: %+v", op, e)
		}
	}
}

// API.md P7a: an enum member's source is its declaration, which Retire edits.
func TestMemberEditability(t *testing.T) {
	checkEdits(t, examples(t, nil, "teamboard"), []editCase{
		{in: "teamboard:Actor.triager", op: edit.OpRetire, mode: edit.ModeCanon, file: "teamboard/taxonomy.canon", text: "triager"},
	})
}

// A Resolved another snapshot produced, or whose steps are not this snapshot's values, is ErrForeign.
func TestEditableForeignPath(t *testing.T) {
	f, g := lawFixture(t, nil, "a"), lawFixture(t, nil, "a")
	other := resolve(t, g, "a:items[b].n")
	tampered := resolve(t, f, "a:items[b].n")
	tampered.Steps = append([]edit.Step(nil), tampered.Steps...)
	tampered.Steps[1].Value = other.Steps[1].Value
	for _, r := range []edit.Resolved{{Canonical: "a:nothing"}, {Canonical: "a:["}, other, tampered} {
		if e, err := f.Editable(r, edit.OpSet, ""); !errors.Is(err, edit.ErrForeign) {
			t.Errorf("%s: %+v, %v, want ErrForeign", r.Canonical, e, err)
		}
		if _, err := f.Type(r); !errors.Is(err, edit.ErrForeign) {
			t.Errorf("Type(%s): %v, want ErrForeign", r.Canonical, err)
		}
	}
	mustEdit(t, f, resolve(t, f, "a:items[b].n"), edit.OpSet, "")
}

// API.md §7.2: `layered` precedes `format`, an amended value of load.defines is layered (W10).
func TestReasonLayeredBeforeFormat(t *testing.T) {
	checkEdits(t, lawFixture(t, []string{"dev"}, "a"), []editCase{
		{in: "a:defs.A.value", reason: edit.ReasonLayered, amends: "dev"},
		{in: "a:defs.B.value", reason: edit.ReasonFormat},
		{in: "a:defs.A.value", layer: "dev", mode: edit.ModeCanon, file: "a/dev.layer.canon", text: "5"},
	})
}

// API.md W11a: the edit layer's amendment is a source: a literal is walked, a JSON load reads its
// file, any other expression is replaced whole and nothing below it is editable.
func TestW11aAmendmentRightHandSide(t *testing.T) {
	checkEdits(t, lawFixture(t, []string{"dev"}, "a"), []editCase{
		{in: "a:holder.data.n", layer: "dev", mode: edit.ModeCanon, file: "a/dev.layer.canon", text: "1"},
		{in: "a:holder.data", layer: "dev", mode: edit.ModeCanon, file: "a/dev.layer.canon", text: `{ id: "L", n: 1 }`},
		{in: "a:holder.data.n", reason: edit.ReasonLayered, amends: "dev"},
		{in: "a:other.data.n", layer: "dev", mode: edit.ModeJSON, file: "a/item.json", text: "5"},
		{in: "a:other.data", layer: "dev", mode: edit.ModeCanon, file: "a/dev.layer.canon", text: `load("item.json")`},
		{in: "a:base.sub", layer: "dev", mode: edit.ModeCanon, file: "a/dev.layer.canon", text: "mk()"},
		{in: "a:base.sub.n", layer: "dev", reason: edit.ReasonComputed},
	})
}

// API.md W11a: an inactive edit layer's amendment of the path or an ancestor is found by its path.
func TestW11aInactiveEditLayer(t *testing.T) {
	checkEdits(t, lawFixture(t, []string{"dev"}, "a"), []editCase{
		{in: "a:items[b].n", layer: "stage", mode: edit.ModeCanon, file: "a/stage.layer.canon", text: "9"},
		{in: "a:items[a].n", layer: "stage", mode: edit.ModeCanon, file: "a/stage.layer.canon", text: "3"},
		{in: "a:items[a]", layer: "stage", mode: edit.ModeCanon, file: "a/stage.layer.canon", text: `{ id: "a", n: 3 }`},
		{in: "a:rows[7].label", layer: "stage", mode: edit.ModeCanon, file: "a/stage.layer.canon"},
	})
	lc := "a/lc.layer.canon"
	checkEdits(t, lawFixture(t, nil, "a"), []editCase{
		{in: "a:holder.data.n", layer: "lc", reason: edit.ReasonComputed},
		{in: "a:holder.data", layer: "lc", mode: edit.ModeCanon, file: lc, text: "mk()"},
		{in: "a:other.data.n", layer: "lc", mode: edit.ModeJSON, file: "a/item.json"},
		{in: "a:other.data", layer: "lc", mode: edit.ModeCanon, file: lc, text: `load("item.json")`},
	})
}

// EVALUATION.md §9.2: qualified keys, negative indexes and positions name a step; W11a edits its node.
func TestW11aAmendPathSegments(t *testing.T) {
	ld, le := "a/ld.layer.canon", "a/le.layer.canon"
	checkEdits(t, lawFixture(t, nil, "a"), []editCase{
		{in: "a:bag.byC[green]", layer: "ld", mode: edit.ModeCanon, file: ld, text: "7"},
		{in: "a:bag.xs[2]", layer: "ld", mode: edit.ModeCanon, file: ld, text: "9"},
		{in: "a:bag.xs[0]", layer: "ld", mode: edit.ModeCanon, file: ld, text: "8"},
		{in: "a:bag.xs[1]", layer: "ld", mode: edit.ModeCanon, file: ld},
		{in: "a:bag.byC[green]", layer: "le", mode: edit.ModeCanon, file: le, text: "5"},
		{in: "a:bag.xs[1]", layer: "le", mode: edit.ModeCanon, file: le, text: "8"},
		{in: "a:bag.ts[red].w", layer: "le", mode: edit.ModeCanon, file: le, text: "6"},
		{in: "a:bag.ts[red]", layer: "le", mode: edit.ModeCanon, file: le, text: "{ c: red, w: 6 }"},
	})
}

// W11a with an ancestor's amendment that lacks the child (log-2026-09-28 U4b round 3): a record
// field is inserted, below a spread-supplied field is computed, an element, entry or key is `layer`.
func TestW11aAmendmentLacksChild(t *testing.T) {
	lf := "a/lf.layer.canon"
	checkEdits(t, lawFixture(t, nil, "a"), []editCase{
		{in: "a:bag.xs[0]", layer: "lf", mode: edit.ModeCanon, file: lf, text: "5"},
		{in: "a:bag.xs[2]", layer: "lf", reason: edit.ReasonLayer},
		{in: "a:bag.ts[red]", layer: "lf", reason: edit.ReasonLayer},
		{in: "a:bag.ts[red].w", layer: "lf", reason: edit.ReasonLayer},
		{in: "a:bag.byC[green]", layer: "lf", reason: edit.ReasonLayer},
		{in: "a:other.data.n", layer: "lf", mode: edit.ModeCanon, file: lf, text: `{ id: "q" }`},
		{in: "a:bag.b.y", layer: "lf", mode: edit.ModeCanon, file: lf, text: "1"},
		{in: "a:bag.b.sub", layer: "lf", mode: edit.ModeCanon, file: lf, text: "{ ...base, y: 1 }"},
		{in: "a:bag.b.sub.n", layer: "lf", reason: edit.ReasonComputed},
	})
}

// API.md §7.2 layered with stacked layers: judged against the layer that sets the final value.
func TestStackedLayers(t *testing.T) {
	checkEdits(t, lawFixture(t, []string{"la", "lb"}, "a"), []editCase{
		{in: "a:holder.data.n", layer: "lb", mode: edit.ModeCanon, file: "a/lb.layer.canon", text: "3"},
		{in: "a:holder.data.n", layer: "la", reason: edit.ReasonLayered, amends: "lb"},
		{in: "a:holder.data.n", reason: edit.ReasonLayered, amends: "lb"},
		{in: "a:holder.data.id", layer: "la", mode: edit.ModeCanon, file: "a/la.layer.canon", text: `"A"`},
		{in: "a:holder.data.id", reason: edit.ReasonLayered, amends: "la"},
	})
}

// API.md W11: AddEntry on a root table or map is an amendment; Set or Reset of a root is not;
// an enum member has none; a new layer file goes in the package's own directory.
func TestW11RootsMembersAndNewFiles(t *testing.T) {
	checkEdits(t, lawFixture(t, []string{"dev"}, "a"), []editCase{
		{in: "a:entries", op: edit.OpAddEntry, layer: "dev", mode: edit.ModeCanon, file: "a/dev.layer.canon"},
		{in: "a:byColor", op: edit.OpAddEntry, layer: "dev", mode: edit.ModeCanon, file: "a/dev.layer.canon"},
		{in: "a:entries", layer: "dev", reason: edit.ReasonLayer},
		{in: "a:entries", op: edit.OpReset, layer: "dev", reason: edit.ReasonLayer},
		{in: "a:Color.red", op: edit.OpRetire, layer: "dev", reason: edit.ReasonLayer},
		{in: "a:items[b].n", layer: "work", mode: edit.ModeCanon, file: "a/work.layer.canon"},
	})
}

// API.md W7, W8: a field of a case written as a bare name is inserted at that name.
func TestW7BareCase(t *testing.T) {
	checkEdits(t, lawFixture(t, nil, "a"), []editCase{
		{in: "a:sq.side", mode: edit.ModeCanon, file: "a/a.canon", text: "square"},
	})
}

// f is the source text of the node that states the value at path.
func f(fx fixture, path string) string {
	e, _ := fx.Editable(resolveQuiet(fx, path), edit.OpSet, "")
	return fx.text(e.Span)
}

func resolveQuiet(fx fixture, path string) edit.Resolved {
	p, _ := edit.Parse(path)
	r, _ := edit.Resolve(fx.Snapshot, p)
	return r
}

// VIEWMODEL.md §12.6 `editable`/`reason`: the mode at each value's root (log-2026-09-28 call 6).
func TestRootLevelEditability(t *testing.T) {
	type root struct {
		path   string
		mode   edit.Mode
		reason edit.Reason
	}
	cases := []struct {
		f     fixture
		roots []root
	}{
		{examples(t, nil, "teamboard"), []root{
			{"teamboard:intents", edit.ModeCanon, edit.ReasonNone}, {"teamboard:columns", edit.ModeCanon, edit.ReasonNone},
			{"teamboard:triageMinRole", edit.ModeCanon, edit.ReasonNone}, {"teamboard:defaultSeverity", edit.ModeNone, edit.ReasonComputed},
		}},
		{examples(t, nil, "resource.farm"), []root{
			{"resource.farm:farm", edit.ModeJSON, edit.ReasonNone}, {"resource.farm:attributes", edit.ModeNone, edit.ReasonFormat},
			{"resource.farm:FARM_MAX_MODELS", edit.ModeCanon, edit.ReasonNone},
		}},
		{examples(t, nil, "pipeline"), []root{{"pipeline:potions", edit.ModeJSON, edit.ReasonNone}}},
		{lawFixture(t, nil, "a"), []root{{"a:notes", edit.ModeNone, edit.ReasonFormat}, {"a:viaName", edit.ModeNone, edit.ReasonComputed}}},
	}
	for _, c := range cases {
		for _, r := range c.roots {
			if e := mustEdit(t, c.f, resolve(t, c.f, r.path), edit.OpSet, ""); e.Mode != r.mode || e.Reason != r.reason {
				t.Errorf("%s: %+v, want mode %d reason %d", r.path, e, r.mode, r.reason)
			}
		}
	}
}

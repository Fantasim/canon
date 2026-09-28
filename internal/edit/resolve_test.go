package edit_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/value"
)

// canonCase is a path, the canonical path it resolves to, and the target's text form.
type canonCase struct{ in, canon, text string }

func checkCanon(t *testing.T, f fixture, cases []canonCase) {
	t.Helper()
	for _, c := range cases {
		r := resolve(t, f, c.in)
		if r.Canonical != c.canon || r.Target.CanonText() != c.text {
			t.Errorf("%s = %s (%s), want %s (%s)", c.in, r.Canonical, r.Target.CanonText(), c.canon, c.text)
		}
	}
}

// errCase is a path Resolve refuses, the sentinel and the segment it names.
type errCase struct {
	in  string
	err error
	seg int
}

func checkErrs(t *testing.T, f fixture, cases []errCase) {
	t.Helper()
	for _, c := range cases {
		err := resolveErr(t, f, c.in)
		var pe *edit.PathError
		if !errors.Is(err, c.err) || !errors.As(err, &pe) || pe.Seg != c.seg {
			t.Errorf("%s: %v, want %v at segment %d", c.in, err, c.err, c.seg)
		}
	}
}

// API.md P1: `[k]` on a keyed list is a key, read as a literal of the key field's type, never a
// position; `.k` is a word key; `[#n]` is the position.
func TestP1KeyedListKeys(t *testing.T) {
	checkCanon(t, examples(t, nil, "resource.farm"), []canonCase{
		{"resource.farm:farm.modelTypes[7].typeName", "resource.farm:farm.modelTypes[7].typeName", "Premium"},
		{"resource.farm:farm.modelTypes[#1].typeId", "resource.farm:farm.modelTypes[7].typeId", "7"},
		{"resource.farm:farm.modelTypes[1].levels[1].level", "resource.farm:farm.modelTypes[1].levels[1].level", "2"},
	})
	checkCanon(t, examples(t, nil, "pipeline"), []canonCase{
		{"pipeline:potions[II_POT_HEAL_L].cooldown", "pipeline:potions[II_POT_HEAL_L].cooldown", "8s"},
		{`pipeline:potions["II_POT_HEAL_S"].heal`, "pipeline:potions[II_POT_HEAL_S].heal", "500"},
		{"pipeline:potions.II_POT_HEAL_S.heal", "pipeline:potions[II_POT_HEAL_S].heal", "500"},
	})
	f := lawFixture(t, nil, "a")
	checkCanon(t, f, []canonCase{
		{"a:rows[7].code", "a:rows[7].code", "7"},
		{"a:tones[green].c", "a:tones[green].c", "green"},
		{`a:tones["GREEN"].c`, "a:tones[green].c", "green"},
		{"a:links[one].to", "a:links[one].to", "one"},
	})
	checkErrs(t, f, []errCase{
		{"a:rows[x]", edit.ErrBadPath, 0},
		{`a:rows["7"]`, edit.ErrBadPath, 0},
		{"a:items[3]", edit.ErrBadPath, 0},
		{"a:tones[3]", edit.ErrBadPath, 0},
		{"a:rows[8]", edit.ErrNoPath, 0},
		{"a:items[c]", edit.ErrNoPath, 0},
		{"a:tones[blue]", edit.ErrNoPath, 0},
	})
}

// API.md P2: a map key is read by the key type: a word or JSON string for String, digits for
// integers, a member name or wire value for an enum, a ref's target key for a dependent map,
// and a JSON string equal to a literal of a literal union as that literal.
func TestP2MapKeys(t *testing.T) {
	checkCanon(t, lawFixture(t, nil, "a"), []canonCase{
		{"a:byColor[red]", "a:byColor[red]", "1"},
		{`a:byColor["GREEN"]`, "a:byColor[green]", "2"},
		{"a:named[plain]", "a:named[plain]", "1"},
		{`a:named["plain"]`, "a:named[plain]", "1"},
		{`a:named["two words"]`, `a:named["two words"]`, "2"},
		{`a:named["say \"hi\""]`, `a:named["say \"hi\""]`, "3"},
		{"a:numbered[3]", "a:numbered[3]", "three"},
		{"a:numbered[-4]", "a:numbered[-4]", "minus four"},
		{"a:perEntry[two]", "a:perEntry[two]", "2"},
		{`a:perEntry["one"]`, "a:perEntry[one]", "1"},
	})
	u := lawFixture(t, nil, "u")
	checkCanon(t, u, []canonCase{
		{`u:tagged["none"]`, `u:tagged["none"]`, "0"},
		{"u:tagged[#1]", `u:tagged["none"]`, "0"},
		{"u:tagged[red]", "u:tagged[red]", "1"},
	})
	if r := resolve(t, u, resolve(t, u, "u:tagged[#1]").Canonical); r.Target.CanonText() != "0" {
		t.Errorf("the canonical literal-union key names %s", r.Target.CanonText())
	}
	checkErrs(t, u, []errCase{{"u:tagged[none]", edit.ErrNoPath, 0}})
	checkCanon(t, lawFixture(t, nil, "a"), []canonCase{
		{"a:picked.w[red]", "a:picked.w[red]", "1"},
		{`a:picked.w["GREEN"]`, "a:picked.w[green]", "2"},
	})
	checkCanon(t, examples(t, nil, "resource.adventurequest"), []canonCase{
		{"resource.adventurequest:adventureQuests.targetMultipliers[two]", "resource.adventurequest:adventureQuests.targetMultipliers[two]",
			resolve(t, examples(t, nil, "resource.adventurequest"), `resource.adventurequest:adventureQuests.targetMultipliers["2_tasks"]`).Target.CanonText()},
	})
	checkErrs(t, lawFixture(t, nil, "a"), []errCase{
		{"a:numbered[x]", edit.ErrBadPath, 0},
		{`a:numbered["3"]`, edit.ErrBadPath, 0},
		{"a:byColor[1]", edit.ErrBadPath, 0},
		{"a:byColor[blue]", edit.ErrNoPath, 0},
		{`a:byColor["green"]`, edit.ErrNoPath, 0},
		{"a:numbered[5]", edit.ErrNoPath, 0},
	})
}

// API.md P3: `.id` and `.retired` of a table entry, `.kind` of a variant are readable; a record
// field of the same name wins (RES-08: a keyed-list element may declare `id`).
func TestP3PseudoFields(t *testing.T) {
	tb := examples(t, nil, "teamboard")
	checkCanon(t, tb, []canonCase{
		{"teamboard:statuses.open.id", "teamboard:statuses.open.id", "open"},
		{"teamboard:statuses.open.retired", "teamboard:statuses.open.retired", "false"},
	})
	checkCanon(t, lawFixture(t, nil, "a"), []canonCase{{"a:shape.kind", "a:shape.kind", "circle"}})
	r := resolve(t, examples(t, nil, "pipeline"), "pipeline:potions[II_POT_HEAL_L].id")
	if p := r.Target.Prov(); p == nil || p.Kind != value.ProvJSON || p.Pointer != "/dwID" {
		t.Errorf("potions[II_POT_HEAL_L].id is %+v, want the field read from /dwID", p)
	}
	checkErrs(t, lawFixture(t, nil, "a"), []errCase{{"a:items[a].retired", edit.ErrNoPath, 1}, {"a:base.kind", edit.ErrNoPath, 0}})
}

// API.md P4: `[#n]` counts from 0; a negative position is ErrBadPath, one past the end ErrNoPath;
// canonical paths never hold one.
func TestP4Positions(t *testing.T) {
	f := examples(t, nil, "teamboard")
	checkCanon(t, f, []canonCase{
		{"teamboard:deck.layouts[#1]", "teamboard:deck.layouts[1]", "2:2"},
		{"teamboard:statuses[#1].label", "teamboard:statuses.taken.label", "Taken"},
		{"teamboard:statuses.open.next[#2]", "teamboard:statuses.open.next[2]", "duplicate"},
	})
	checkCanon(t, lawFixture(t, nil, "a"), []canonCase{{"a:byColor[#1]", "a:byColor[green]", "2"}})
	checkErrs(t, f, []errCase{
		{"teamboard:deck.layouts[#2]", edit.ErrNoPath, 1},
		{"teamboard:deck.layouts[2]", edit.ErrNoPath, 1},
		{"teamboard:statuses[#6]", edit.ErrNoPath, 0},
		{"teamboard:deck.layouts[-1]", edit.ErrBadPath, 1},
	})
	built := edit.Path{Package: "teamboard", Root: "deck", Segs: []edit.Seg{{Kind: edit.SegField, Name: "layouts"}, {Kind: edit.SegPos, Pos: -1}}}
	if _, err := edit.Resolve(f.Snapshot, built); !errors.Is(err, edit.ErrBadPath) {
		t.Errorf("a negative position: %v, want ErrBadPath", err)
	}
}

// API.md P5: an unsupported segment is ErrBadPath naming it; a field the record or current case lacks, ErrNoPath.
func TestP5UnsupportedSegments(t *testing.T) {
	checkErrs(t, examples(t, nil, "teamboard"), []errCase{
		{"teamboard:deck[0]", edit.ErrBadPath, 0},
		{"teamboard:deck[#0]", edit.ErrBadPath, 0},
		{"teamboard:statuses.open.next.x", edit.ErrBadPath, 2},
		{"teamboard:statuses.open.tone.x", edit.ErrBadPath, 2},
		{"teamboard:statuses[1]", edit.ErrBadPath, 0},
		{"teamboard:deck.nothing", edit.ErrNoPath, 0},
	})
	checkErrs(t, lawFixture(t, nil, "a"), []errCase{
		{"a:byColor.red", edit.ErrBadPath, 0},
		{"a:shape.side", edit.ErrNoPath, 0},
		{"a:shape[0]", edit.ErrBadPath, 0},
	})
}

// API.md P6: an unqualified root is a public value or constant of exactly one loaded package.
func TestP6UnqualifiedRoot(t *testing.T) {
	checkCanon(t, examples(t, nil, "teamboard"), []canonCase{
		{"statuses.open.label", "teamboard:statuses.open.label", "Open"},
		{"VERSION", "teamboard:VERSION", "7"},
	})
	f := lawFixture(t, nil, "a", "b")
	checkErrs(t, f, []errCase{{"hidden", edit.ErrNoPath, -1}, {"nothing", edit.ErrNoPath, -1}, {"nope:shared", edit.ErrNoPath, -1}})
	var pe *edit.PathError
	if err := resolveErr(t, f, "shared"); !errors.Is(err, edit.ErrAmbiguousPath) || !errors.As(err, &pe) ||
		!reflect.DeepEqual(pe.Candidates, []string{"a:shared", "b:shared"}) {
		t.Errorf("shared: %v %+v, want ErrAmbiguousPath listing a:shared and b:shared", err, pe)
	}
}

// API.md P7: with a prefix, any let or const of the package, local ones included.
func TestP7QualifiedRoot(t *testing.T) {
	checkCanon(t, lawFixture(t, nil, "a", "b"), []canonCase{
		{"a:hidden", "a:hidden", "3"},
		{"b:shared", "b:shared", "2"},
		{"a:KS[1]", "a:KS[1]", "2"},
	})
	checkCanon(t, examples(t, nil, "resource.farm"), []canonCase{
		{"resource.farm:texts.TID_BLANK.value", "resource.farm:texts.TID_BLANK.value", "0"},
		{"resource.farm:FARM_MAX_MODELS", "resource.farm:FARM_MAX_MODELS", "100"},
	})
}

// API.md P7a: a root may name an enum; the path is exactly `Enum.member`; a value and an enum of
// the same name are ambiguous unless qualified.
func TestP7aEnumRoot(t *testing.T) {
	checkCanon(t, examples(t, nil, "teamboard"), []canonCase{
		{"Actor.triager", "teamboard:Actor.triager", "triager"},
		{"teamboard:PostField.reason", "teamboard:PostField.reason", "reason"},
	})
	f := lawFixture(t, nil, "a", "b")
	r := resolve(t, f, "a:Color.green")
	if m, ok := r.Target.(*value.Member); !ok || m.Enum.Members[m.Index].Wire != "GREEN" || len(r.Steps) != 1 {
		t.Errorf("a:Color.green = %#v", r.Target)
	}
	checkErrs(t, f, []errCase{
		{"Color.red", edit.ErrAmbiguousPath, -1},
		{"a:Color", edit.ErrBadPath, -1},
		{"a:Color[0]", edit.ErrBadPath, 0},
		{"a:Color.red.x", edit.ErrBadPath, 1},
		{"a:Color.blue", edit.ErrNoPath, 0},
	})
}

// API.md P8, P9: canonical segments are `.name` for fields and table entries, `[key]` for
// keyed-list elements and map entries, `[n]` for plain-list elements; a key is a word when it
// is one, an integer in decimal, an enum key its member name, anything else a JSON string.
func TestP8P9CanonicalForm(t *testing.T) {
	checkCanon(t, examples(t, nil, "teamboard"), []canonCase{
		{"teamboard:statuses[open].next[#0]", "teamboard:statuses.open.next[0]", "taken"},
		{`teamboard:statuses["taken"].requires[0]`, "teamboard:statuses.taken.requires[0]", "assignee"},
	})
	f := lawFixture(t, nil, "a")
	checkCanon(t, f, []canonCase{
		{"a:named[#1]", `a:named["two words"]`, "2"},
		{"a:numbered[#1]", "a:numbered[-4]", "minus four"},
		{`a:tones[#0]`, "a:tones[green]", "Tone{c: green, w: 0}"},
		{"a:entries[#1].v", "a:entries.two.v", "2"},
	})
	for _, in := range []string{"a:named[#2]", "a:items[#1].n", "a:perEntry[#0]", "a:links[#0].w"} {
		r := resolve(t, f, in)
		if strings.Contains(r.Canonical, "[#") || resolve(t, f, r.Canonical).Target.CanonText() != r.Target.CanonText() {
			t.Errorf("%s: canonical %s does not name the same value", in, r.Canonical)
		}
	}
}

// API.md P10: Value.Path carries the `package:` prefix, a path given without one included.
func TestP10PackagePrefix(t *testing.T) {
	f := examples(t, nil, "teamboard")
	for _, in := range []string{"deck", "deck.maxHidden", "Actor.reporter"} {
		if r := resolve(t, f, in); r.Canonical != "teamboard:"+in {
			t.Errorf("%s: %s", in, r.Canonical)
		}
	}
}

// API.md R6: a poisoned root is ErrNoValue; a root of a package the analysis did not select has
// no settled value (build analysis handle, log-2026-09-28).
func TestResolveUnsettledRoots(t *testing.T) {
	broken := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/p/p.canon":     file("/// P.\npackage p\n\n/// Out of range.\nlet bad: Int = [1][5]\n"),
	}
	p, err := buildOpen(broken)
	if err != nil {
		t.Fatal(err)
	}
	checkErrs(t, analyze(t, p, []string{"p"}, false), []errCase{{"p:bad", edit.ErrNoValue, -1}})
	checkErrs(t, lawFixture(t, nil, "u"), []errCase{{"a:shared", edit.ErrNotAnalyzed, -1}})
}

// API.md §6.2: a step keeps its container's static type and its value; an input field has none (TYP-17).
func TestResolveSteps(t *testing.T) {
	r := resolve(t, examples(t, nil, "teamboard"), "teamboard:statuses.open.next[0]")
	want := []string{"stable table teamboard.Status", "teamboard.Status", "[ref teamboard.statuses]"}
	for i, st := range r.Steps {
		if st.Container.String() != want[i] || st.Value == nil {
			t.Errorf("step %d: %s in %v, want %s", i, st.Seg.Name, st.Container, want[i])
		}
	}
	if r.Target != r.Steps[len(r.Steps)-1].Value {
		t.Error("the target is not the last step's value")
	}
	f := examples(t, []string{"louis"}, "service.resourcestudio")
	if r := resolve(t, f, "config.gen.apiKey"); r.Target != nil {
		t.Errorf("an input field has %v", r.Target)
	}
	checkErrs(t, f, []errCase{{"config.gen.apiKey.x", edit.ErrNoPath, 2}})
}

// API.md §5.2 TypeInfo: the declared type of the root, field, element, entry or pseudo-field.
func TestType(t *testing.T) {
	cases := []struct {
		f     fixture
		paths [][2]string
	}{
		{examples(t, nil, "resource.farm"), [][2]string{
			{"resource.farm:farm", "resource.farm.FarmConfig"},
			{"resource.farm:farm.global.maxModels", "Int?"},
			{"resource.farm:farm.modelTypes", "[resource.farm.ModelType](..=100) keyed by typeId"},
			{"resource.farm:farm.modelTypes[1]", "resource.farm.ModelType"},
			{"resource.farm:farm.modelTypes[1].unlockPrice", "resource.farm.Penya"},
		}},
		{examples(t, nil, "teamboard"), [][2]string{
			{"teamboard:statuses.open.id", "String"},
			{"teamboard:statuses.open.retired", "Bool"},
			{"teamboard:Actor.triager", "teamboard.Actor"},
			{"teamboard:statuses.open.label", "String(1..)"},
		}},
		{lawFixture(t, nil, "a"), [][2]string{{"a:shape.r", "Int"}, {"a:shape.kind", "Kind(a.Shape)"}, {"a:byColor[red]", "Int"}, {"a:perEntry[one]", "Int"}}},
	}
	for _, c := range cases {
		for _, pw := range c.paths {
			if got, err := c.f.Type(resolve(t, c.f, pw[0])); err != nil || got == nil || got.String() != pw[1] {
				t.Errorf("Type(%s) = %v, %v, want %s", pw[0], got, err, pw[1])
			}
		}
	}
	if got, err := lawFixture(t, nil, "a").Type(edit.Resolved{Canonical: "a:nothing"}); !errors.Is(err, edit.ErrForeign) {
		t.Errorf("Type of a foreign path = %v, %v", got, err)
	}
}

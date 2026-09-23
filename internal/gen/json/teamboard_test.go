package jsongen_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"testing"

	jsongen "github.com/fantasim/canonlang/internal/gen/json"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/value"
)

const teamboardGolden = "testdata/teamboard.txtar"

func (x *taxonomy) areaValues() *ir.Value {
	var entries []*value.Record
	for _, r := range areaRows() {
		hint := value.Value(none(tString))
		if r[6] != "" {
			hint = str(r[6])
		}
		entries = append(entries, x.areas.entry(r[0], x.areaGroups.ref(r[1]), x.tone.m(r[2]), x.icon.m(r[3]), str(r[4]), hint,
			x.role.m(r[5]), x.areas.refs(strings.Fields(r[7])...)))
	}
	return x.areas.value(entries...)
}

// fns are two of taxonomy.canon's package fns, over a table and an enum domain (WIRE.md §5.11).
func (x *taxonomy) fns(statuses, areas *value.Table) []*ir.ExportFn {
	domain := make([]value.Value, len(statuses.Entries))
	for i, e := range statuses.Entries {
		domain[i] = x.statuses.ref(e.Ident.Key.S)
	}
	var cells []value.Value
	for _, from := range statuses.Entries {
		for _, to := range statuses.Entries {
			cells = append(cells, boolean(slices.ContainsFunc(from.Fields[3].(*value.List).Elems, func(n value.Value) bool {
				return n.(*value.Ref).Key == to.Ident.Key
			})))
		}
	}
	canTransition := &ir.ExportFn{Name: "canTransition", Kind: ir.FnLookup, Result: tBool.ir,
		Params: []*ir.Param{{Name: "from", Type: x.statusRef.ir}, {Name: "to", Type: x.statusRef.ir}},
		Table:  &ir.LookupTable{Domains: [][]value.Value{domain, domain}, Cells: cells}}
	var visible []value.Value
	for role := range x.role.t.Members {
		l := list(x.areaRef)
		for _, a := range areas.Entries {
			if role >= a.Fields[5].(*value.Member).Index {
				l.Elems = append(l.Elems, x.areas.ref(a.Ident.Key.S))
			}
		}
		visible = append(visible, l)
	}
	areasVisibleTo := &ir.ExportFn{Name: "areasVisibleTo", Kind: ir.FnLookup, Result: listOf(x.areaRef).ir,
		Params: []*ir.Param{{Name: "role", Type: x.role.typ().ir}},
		Table:  &ir.LookupTable{Domains: [][]value.Value{x.role.all()}, Cells: visible}}
	return []*ir.ExportFn{canTransition, areasVisibleTo}
}

// teamboard is examples/teamboard after stage E, trimmed to seven values, with its emits:
// go and ts baked, json in directory mode with the default values.
func teamboard() *ir.Package {
	x := newTaxonomy()
	intents, statuses, groups, areas := x.intentValues(), x.statusValues(), x.areaGroupValues(), x.areaValues()
	values := []*ir.Value{intents, statuses,
		{Name: "assigneeMinRole", Type: x.role.typ().ir, V: x.role.m("maintainer"), Schema: "sovcommon.roles.Role@dc935485"},
		{Name: "deck", Type: x.deck.typ().ir, V: x.deck.rec(list(tString, str("4:1"), str("2:2")), num(2)), Schema: "teamboard.Deck@02af81fb"},
		groups, areas,
		{Name: "initialStatus", Type: x.statusRef.ir, V: x.statuses.ref("open")},
	}
	return &ir.Package{Name: tb, Dir: tb, Values: values, Fns: x.fns(statuses.V.(*value.Table), areas.V.(*value.Table)),
		Types: []ir.Type{x.actor.ir, x.postField.ir, x.intent.ir, x.status.ir, x.deck.ir, x.areaGroup.ir, x.area.ir},
		Emits: []*ir.Emit{
			{Target: ir.TargetGo, Out: "@sovcommon/teamboard", Dir: "sovcommon/teamboard", Mode: ir.ModeBaked, GoPackage: tb,
				GoImport: "gitlab.com/sovereign15/sovcommon/teamboard"},
			{Target: ir.TargetTS, Out: "@web/teamboard/generated.ts", Dir: "web/teamboard", FileName: "generated.ts", Mode: ir.ModeBaked},
			{Target: ir.TargetJSON, Out: "@sovcommon/teamboard/data/", Dir: "sovcommon/teamboard/data"},
		}}
}

func generate(t *testing.T, p *ir.Package, e *ir.Emit) []ir.File {
	t.Helper()
	files, err := jsongen.Generate(p, e)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// WIRE.md §8.1 directory mode, against testdata/teamboard.txtar (written by -update).
func TestTeamboardDirectory(t *testing.T) {
	p := teamboard()
	files := generate(t, p, p.Emits[2])
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
		t.Run(f.Path, func(t *testing.T) {
			golden.Run(t, teamboardGolden, func(*testing.T, golden.Case) []byte { return f.Content }, golden.Expected(f.Path))
		})
	}
	want := []string{"intents.json", "statuses.json", "assigneeMinRole.json", "deck.json", "areaGroups.json", "areas.json", "initialStatus.json"}
	if !slices.Equal(paths, want) {
		t.Errorf("paths %v, want %v", paths, want)
	}
	cases, err := golden.Load(teamboardGolden)
	if err != nil {
		t.Fatal(err)
	}
	var archived []string
	for _, f := range cases[0].Archive.Files {
		archived = append(archived, f.Name)
	}
	if !slices.Equal(archived, want) {
		t.Errorf("golden archive holds %v, want %v", archived, want)
	}
}

// WIRE.md §5.11: the first value listed carries `$fns`, and its `$schema` covers them.
func TestFnsGoToTheFirstValue(t *testing.T) {
	p := teamboard()
	for i, f := range generate(t, p, p.Emits[2]) {
		if has := bytes.Contains(f.Content, []byte(`"$fns"`)); has != (i == 0) {
			t.Errorf("%s: $fns present %v", f.Path, has)
		}
	}
	p.Values[3].Schema = ""
	files := generate(t, p, jsonEmit("data", "deck", "intents"))
	if !bytes.Contains(files[0].Content, []byte(`"$fns"`)) || bytes.Contains(files[1].Content, []byte(`"$fns"`)) {
		t.Errorf("values [deck, intents]: $fns not in deck.json only")
	}
	if bytes.Contains(files[0].Content, []byte("Deck@02af81fb")) {
		t.Errorf("deck.json with $fns kept the schema without them:\n%s", files[0].Content)
	}
}

// WIRE.md §8.3: the record and scalar samples, byte for byte.
func TestTeamboardSamples(t *testing.T) {
	p := teamboard()
	files := generate(t, p, p.Emits[2])
	role := "{\n  \"$schema\": \"sovcommon.roles.Role@dc935485\",\n  \"value\": \"maintainer\"\n}\n"
	sum := sha256.Sum256(files[3].Content)
	if string(files[2].Content) != role || hex.EncodeToString(sum[:]) != "3d47a2ebd8ea6ff139c1479016e382b989bc81678df76ee2c974ca2f00c9d87c" {
		t.Errorf("samples:\n%s\n%s", files[2].Content, files[3].Content)
	}
}

// DOCTRINE §5: fresh fixtures (new pointers, new map seeds) give the same files every time.
func TestDeterminism(t *testing.T) {
	first := teamboard()
	want := generate(t, first, first.Emits[2])
	for range 20 {
		p := teamboard()
		if got := generate(t, p, p.Emits[2]); !slices.EqualFunc(got, want, func(a, b ir.File) bool {
			return a.Path == b.Path && bytes.Equal(a.Content, b.Content)
		}) {
			t.Fatal("two runs differ")
		}
	}
}

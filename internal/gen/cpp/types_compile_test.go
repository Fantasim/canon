package cppgen_test

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// eventConfigFixture is the committed EventConfig.json that examples/resource/events loads.
var eventConfigFixture = filepath.Join("..", "..", "..", "examples", "_fixtures", "resource", "Server", "Event", "EventConfig.json")

// typesFile is one file the types driver decodes: an edit of the fixture (from → to, "" for none)
// or, for Extras, its own text; and the lines it prints, through Decode and through Parse (the
// same unless parse says otherwise, DECISIONS 361; parseOnly: the file is not valid JSON to Decode).
type typesFile struct {
	name, arg, from, to, text, want, parse string
	parseOnly                              bool
}

// nestedExtras is an Extras file holding an unknown key under `arrays` arrays (WIRE.md §3.1: depth 512, the object counted).
func nestedExtras(arrays int) string {
	return `{"zz": ` + strings.Repeat("[", arrays) + strings.Repeat("]", arrays) + `}`
}

// parseFiles are the files only Parse sees as a text: WIRE.md §3.1's byte rules and §3.2's repeated key, a duplicate refused at its path (DECISIONS 361; CODEGEN.md §5.13).
var parseFiles = []typesFile{
	{name: "dupnested.json", arg: "c", from: `"levelMax": 60`, to: `"levelMax": 60, "levelMax": 61`, parseOnly: true,
		parse: "dupnested.json: error EventConfig: $.events[1].levelMax: duplicate key\n"},
	{name: "duproot.json", arg: "x", text: `{"mode": "auto", "mode": "auto"}`, parseOnly: true, parse: "duproot.json: error Extras: $.mode: duplicate key\n"},
	{name: "dupescaped.json", arg: "x", text: `{"mode": "auto", "m\u006fde": "auto"}`, parseOnly: true, parse: "dupescaped.json: error Extras: $.mode: duplicate key\n"},
	{name: "dupcase.json", arg: "x", text: `{"mode": "auto", "MODE": "auto"}`, parseOnly: true, parse: "dupcase.json: reqMp=7 label=x mode=auto"},
	{name: "bom.json", arg: "x", text: "\xef\xbb\xbf" + `{"mode": "auto"}`, parseOnly: true, parse: "bom.json: reqMp=7 label=x mode=auto"},
	{name: "utf8.json", arg: "x", text: "{\"label\": \"\xff\"}", parseOnly: true, parse: "utf8.json: error Extras: $.label: not UTF-8\n"},
	{name: "utf16.json", arg: "x", text: "\xff\xfe{\x00}\x00", parseOnly: true, parse: "utf16.json: error Extras: invalid JSON at byte 0\n"},
	{name: "surrogate.json", arg: "x", text: `{"label": "\ud800"}`, parseOnly: true, parse: "surrogate.json: error Extras: $.label: unpaired surrogate\n"},
	{name: "trailing.json", arg: "x", text: `{"mode": "auto"} 1`, parseOnly: true, parse: "trailing.json: error Extras: invalid JSON at byte 17\n"},
	{name: "at6a.json", arg: "x", text: `{"d": `, parseOnly: true, parse: "at6a.json: error Extras: invalid JSON at byte 6\n"},
	{name: "at6b.json", arg: "x", text: `{"d": x}`, parseOnly: true, parse: "at6b.json: error Extras: invalid JSON at byte 6\n"},
	{name: "at3a.json", arg: "x", text: `{} {}`, parseOnly: true, parse: "at3a.json: error Extras: invalid JSON at byte 3\n"},
	{name: "at3b.json", arg: "x", text: `{} 123`, parseOnly: true, parse: "at3b.json: error Extras: invalid JSON at byte 3\n"},
	{name: "at9.json", arg: "x", text: `{"a": tru}`, parseOnly: true, parse: "at9.json: error Extras: invalid JSON at byte 9\n"},
	{name: "at11.json", arg: "x", text: `{"d":"hp"} x`, parseOnly: true, parse: "at11.json: error Extras: invalid JSON at byte 11\n"},
	{name: "at8.json", arg: "x", text: "\xef\xbb\xbf" + `{"d" 1}`, parseOnly: true, parse: "at8.json: error Extras: invalid JSON at byte 8\n"},
	{name: "at1.json", arg: "x", text: `[,`, parseOnly: true, parse: "at1.json: error Extras: invalid JSON at byte 1\n"},
	{name: "empty.txt", arg: "x", text: ``, parseOnly: true, parse: "empty.txt: error Extras: invalid JSON at byte 0\n"},
	{name: "scalar.json", arg: "x", text: `7`, parseOnly: true, parse: "scalar.json: error Extras: $: expected an object\n"},
	{name: "depth512.json", arg: "x", text: nestedExtras(511), parseOnly: true, parse: "depth512.json: reqMp=7 label=x mode=authoritative"},
	{name: "depth513.json", arg: "x", text: nestedExtras(512), parseOnly: true, parse: "nested deeper than 512 levels"},
}

// typesFiles: the fixture, broken copies, then Extras' absent keys, markers and fractions (CODEGEN.md §5.13).
var typesFiles = []typesFile{
	{name: "EventConfig.json", arg: "c", want: "EventConfig.json: version=1 events=2\n" +
		" madrigal_bang_hunt world=1 target=200 roll=local_budget\n" +
		"  spawn_monster MI_BANG1=44 region=6800,3200,7100,3500 lifetime=1800000\n" +
		"  Sat 20:00-22:00\n  Sun 20:00-22:00\n" +
		" moonstone_rain world=1 target=50 roll=authoritative\n" +
		"  monster_drop_inject II_GEN_MAT_MOONSTONE count=1..2 levels=20..60\n" +
		"  Wed 18:00-20:00\n" +
		" find moonstone_rain: moonstone_rain\n"},
	{name: "fraction.json", arg: "c", from: `"monsterLifetimeSec": 1800`, to: `"monsterLifetimeSec": 0.5e1`, want: "lifetime=5000"},
	{name: "nulllife.json", arg: "c", from: `"monsterLifetimeSec": 1800`, to: `"monsterLifetimeSec": null`, want: "lifetime=none"},
	{name: "notms.json", arg: "c", from: `"monsterLifetimeSec": 1800`, to: `"monsterLifetimeSec": 1800.0005`,
		want: "notms.json: error EventConfig: $.events[0].monsterLifetimeSec: expected a whole number of milliseconds\n"},
	{name: "range.json", arg: "c", from: "\"worldId\": 1,\n      \"targetCount\": 50", to: "\"worldId\": -1,\n      \"targetCount\": 50",
		want: "range.json: error EventConfig: $.events[1].worldId: expected an integer from 0 to 4294967295\n"},
	{name: "fractional.json", arg: "c", from: `"levelMax": 60`, to: `"levelMax": 6.5`,
		want: "fractional.json: error EventConfig: $.events[1].levelMax: expected an integer\n"},
	{name: "member.json", arg: "c", from: `"day": "Wed"`, to: `"day": "Wednesday"`,
		want: "member.json: error EventConfig: $.events[1].schedule[0].day: unknown value Wednesday\n"},
	{name: "tag.json", arg: "c", from: `"type": "spawn_monster"`, to: `"type": "spawn_boss"`,
		want: "tag.json: error EventConfig: $.events[0].type: unknown case spawn_boss\n"},
	{name: "missing.json", arg: "c", from: `"version": 1,`, to: ``,
		want: "missing.json: error EventConfig: $.version: missing\n"},
	{name: "kind.json", arg: "c", from: `"schedule": [`, to: `"schedule": {}, "x": [`,
		want: "kind.json: error EventConfig: $.events[0].schedule: expected an array\n"},
	{name: "define.json", arg: "c", from: `"MI_BANG1"`, to: `"MI_BANG2"`,
		want: "define.json: error EventConfig: $.events[0].monsterId: define MI_BANG2 is not in this program's Monsters table\n"},
	{name: "root.json", arg: "c", text: `[]`, want: "root.json: error EventConfig: $: expected an object\n"},
	{name: "absent.json", arg: "x", text: `{"unknown": 1, "LABEL": 2}`, want: "absent.json: reqMp=7 label=x mode=authoritative delay=1500 ratio=0.5\n"},
	{name: "marker.json", arg: "x", text: `{"legacy": {"reqMp": 3}, "label": "", "mode": "auto", "delaySec": 0.25}`,
		want: "marker.json: reqMp=3 label=none mode=auto delay=250 ratio=0.5\n"},
	{name: "empty.json", arg: "x", text: `{"legacy": {}, "label": null, "mode": "local_budget", "delaySec": 2}`,
		want: "empty.json: reqMp=7 label=none mode=local_budget delay=2000 ratio=0.5\n"},
	{name: "step.json", arg: "x", text: `{"legacy": null}`, want: "step.json: error Extras: $.legacy: null\n"},
	{name: "exact.json", arg: "x", text: `{"ratio": 16777217}`, want: "exact.json: reqMp=7 label=x mode=authoritative delay=1500 ratio=1.67772e+07\n"},
	{name: "midpoint.json", arg: "x", text: `{"ratio": 16777217.0}`,
		want:  "midpoint.json: error Extras: $.ratio: expected a Float32: the number lies halfway between two Float32 values\n",
		parse: "midpoint.json: reqMp=7 label=x mode=authoritative delay=1500 ratio=1.67772e+07\n"},
	{name: "pick.json", arg: "x", text: `{"mode": "bogus"}`, want: "pick.json: error Extras: $.mode: unknown value bogus\n"},
	{name: "rollmode.json", arg: "x", text: `{"mode": "replayed"}`, want: "rollmode.json: reqMp=7 label=x mode=replayed delay=1500 ratio=0.5\n"}, // RollModeFromWire (DECISIONS 392)
	{name: "picks.json", arg: "p", text: `{"a": "auto", "n0": 1, "m0": "authoritative", "n1": 2, "m1": "auto"}`,
		want: "picks.json: a=auto slots= 1=authoritative 2=auto\n"},
	{name: "required.json", arg: "p", text: `{"a": "bogus"}`, want: "required.json: error Picks: $.a: unknown value bogus\n"},
	{name: "slot.json", arg: "p", text: `{"a": "auto", "n0": 1, "m0": "bogus"}`, want: "slot.json: error Picks: $.m0: unknown value bogus\n"},
	{name: "tenth.json", arg: "x", text: `{"delaySec": 1e-4}`, want: "tenth.json: error Extras: $.delaySec: expected a whole number of milliseconds\n"},
	{name: "nullmode.json", arg: "x", text: `{"mode": null}`, want: "nullmode.json: error Extras: $.mode: null\n"},
}

// CODEGEN.md §5.13, §7.2: Decode accepts EventConfig.json; a broken copy gives no value and its JSON path.
func TestTypesDecodeCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	time := typesTime()
	for _, p := range []*ir.Package{time, typesEvents(time)} {
		out := filepath.Join(dir, filepath.FromSlash(cppEmit(p).Dir))
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, out, generate(t, p))
	}
	copyFile(t, filepath.Join("testdata", "main", "types_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	fixture, err := os.ReadFile(eventConfigFixture)
	if err != nil {
		t.Fatal(err)
	}
	files := slices.Concat(typesFiles, parseFiles, documentOrderFiles)
	args := []string{dir}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.name), typesBody(t, string(fixture), f), 0o644); err != nil {
			t.Fatal(err)
		}
		if !f.parseOnly {
			args = append(args, f.arg+":"+f.name)
		}
		args = append(args, strings.ToUpper(f.arg)+":"+f.name)
	}
	sources := []string{"out/time/time.gen.cpp", "out/events/events.gen.cpp", "main.cpp"}
	for _, out := range buildAndRun(t, dir, sources, args...) {
		sections := typesSections(out)
		for _, f := range files {
			wantParse := cmp.Or(f.parse, f.want)
			if got := sections[strings.ToUpper(f.arg)+":"+f.name]; !strings.Contains(got, wantParse) {
				t.Errorf("Parse %s: output lacks %q:\n%s", f.name, wantParse, got)
			}
			if got := sections[f.arg+":"+f.name]; !f.parseOnly && !strings.Contains(got, f.want) {
				t.Errorf("Decode %s: output lacks %q:\n%s", f.name, f.want, got)
			}
		}
	}
}

// typesSections splits the driver's output into what it printed per argument ("== c:name" and "== C:name" headers).
func typesSections(out string) map[string]string {
	sections := map[string]string{}
	for _, part := range strings.Split(out, "== ")[1:] {
		head, body, _ := strings.Cut(part, "\n")
		sections[head] = body
	}
	return sections
}

// typesBody is the text of f: the fixture edited, or its own.
func typesBody(t *testing.T, fixture string, f typesFile) []byte {
	t.Helper()
	if f.text != "" || f.arg != "c" {
		return []byte(f.text)
	}
	if !strings.Contains(fixture, f.from) {
		t.Fatalf("%s: the fixture has no %s", f.name, f.from)
	}
	return []byte(strings.Replace(fixture, f.from, f.to, 1))
}

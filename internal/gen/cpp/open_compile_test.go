package cppgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// openCase is one text file the open driver reads (kind: cfg, rates, names, rows, bystat) and the lines it prints.
type openCase struct {
	name, kind, text, want string
}

const openGoodConfig = `{"primary": "DST_NEW", "extra": "DST_MP", "list": ["DST_HP", "DST_X", "DST_HP", "DST_OLD"], "stat": "ZZZ", "rank": 200,
  "entries": [{"dst": "DST_HP", "n": 1}, {"dst": "DST_NEW", "n": 2}], "mode": "all", "flags": 5, "shadeOn": "DST_MP", "shade": 7}`

// openCases: DECISIONS 362 (an opened enum keeps an unknown member, compares and hashes by its wire, numeric wires and literal unions stay closed, an unknown member that decides a dependent type fails) and 363 (Decode<Fn>File reads a map in file order, a list and a keyed list, refusals at their $ paths).
var openCases = []openCase{
	{"good", "cfg", openGoodConfig, "primary=DST_NEW/DST_NEW/unknown equal_hp=0 less_mp=1\nextra=DST_MP/mp/known\n" +
		"list=DST_HP/hp/known,DST_X/DST_X/unknown,DST_HP/hp/known,DST_OLD/old/known\ncounts hp=2 unknown=0 size=3\n" +
		"stat=ZZZ/ZZZ/unknown code=none\nrank=high/high/known code=200\nentry DST_HP/hp/known n=1\nentry DST_NEW/DST_NEW/unknown n=2\n" +
		"find hp=1 unknown=found\nmode=all\nflags=atk,spd\nshadeOn=DST_MP/mp/known branch=1\n"},
	{"known", "cfg", strings.Replace(strings.Replace(openGoodConfig, `"stat": "ZZZ"`, `"stat": "DEF"`, 1), `"primary": "DST_NEW"`, `"primary": "DST_HP"`, 1),
		"primary=DST_HP/hp/known equal_hp=1 less_mp=1\n"},
	{"rankcode", "cfg", strings.Replace(openGoodConfig, `"rank": 200`, `"rank": 99`, 1), "error Config: $.rank: unknown value 99\n"},
	{"mode", "cfg", strings.Replace(openGoodConfig, `"mode": "all"`, `"mode": "bogus"`, 1), "error Config: $.mode:"},
	{"nobranch", "cfg", strings.Replace(openGoodConfig, `"shadeOn": "DST_MP"`, `"shadeOn": "DST_NEW"`, 1), "error Config: $.shade: no branch for this value\n"},
	{"retiredbranch", "cfg", strings.Replace(openGoodConfig, `"shadeOn": "DST_MP"`, `"shadeOn": "DST_OLD"`, 1), "error Config: $.shade: no branch for this value\n"},
	{"kind", "cfg", strings.Replace(openGoodConfig, `"primary": "DST_NEW"`, `"primary": 5`, 1), "error Config: $.primary: expected a string\n"},
	{"element", "cfg", strings.Replace(openGoodConfig, `"DST_X"`, `7`, 1), "error Config: $.list[1]: expected a string\n"},
	{"dupkey", "cfg", strings.Replace(openGoodConfig, `"primary": "DST_NEW",`, `"primary": "DST_NEW", "primary": "DST_HP",`, 1), "error Config: $.primary: duplicate key\n"},
	{"dupentry", "cfg", strings.Replace(openGoodConfig, `{"dst": "DST_NEW", "n": 2}`, `{"dst": "DST_HP", "n": 2}`, 1), "error Config: $.entries[1].dst: duplicate id \"DST_HP\""},
	{"rates", "rates", `{"DST_MP": 2, "DST_NEW": 3, "DST_HP": 1}`, "DST_MP/mp/known=2\nDST_NEW/DST_NEW/unknown=3\nDST_HP/hp/known=1\nfind mp=2\n"},
	{"ratesdup", "rates", `{"DST_HP": 1, "DST_HP": 2}`, "error RatesFile: $.DST_HP: duplicate key\n"},
	{"ratesroot", "rates", `[]`, "error RatesFile: $: expected an object\n"},
	{"ratesvalue", "rates", `{"DST_HP": "x"}`, "error RatesFile: $.DST_HP: expected an integer\n"},
	{"ratesnull", "rates", `{"DST_HP": null}`, "error RatesFile: $.DST_HP: null\n"},
	{"ratessyntax", "rates", `{"DST_HP": `, "error RatesFile: invalid JSON at byte 11\n"},
	{"ratesutf8", "rates", "{\"\xff\": 1}", "error RatesFile: $: not UTF-8\n"},
	{"names", "names", `["DST_B", "DST_HP", "DST_B"]`, "names DST_B/DST_B/unknown,DST_HP/hp/known,DST_B/DST_B/unknown\n"},
	{"nameskind", "names", `["DST_HP", 1]`, "error NamesFile: $[1]: expected a string\n"},
	{"namesroot", "names", `{}`, "error NamesFile: $: expected an array\n"},
	{"rows", "rows", `[{"dst": "DST_NEW", "n": 1}, {"dst": "DST_HP", "n": 2}]`, "row DST_NEW/DST_NEW/unknown n=1\nrow DST_HP/hp/known n=2\nfind new=found\n"},
	{"rowsdup", "rows", `[{"dst": "DST_NEW", "n": 1}, {"dst": "DST_NEW", "n": 2}]`, "error RowsFile: $[1].dst: duplicate id \"DST_NEW\""},
	{"rowsfield", "rows", `[{"dst": "DST_NEW"}]`, "error RowsFile: $[0].n: missing\n"},
	{"bystat", "bystat", `{"SPD": {"dst": "DST_MP", "n": 3}, "ATK": {"dst": "DST_HP", "n": 4}, "NEW": {"dst": "DST_Z", "n": 5}}`,
		"SPD/spd/known -> DST_MP/mp/known n=3\nATK/atk/known -> DST_HP/hp/known n=4\nNEW/NEW/unknown -> DST_Z/DST_Z/unknown n=5\n"},
	{"bystaterr", "bystat", `{"ATK": {"dst": 1, "n": 4}}`, "error ByStatFile: $.ATK.dst: expected a string\n"},
}

// DECISIONS 362, 363, CODEGEN.md §5.2, §5.13: an opened enum compiles and runs as a class; Decode<Fn>File reads a map in file order, a list and a keyed list.
func TestOpenEnumsCompileAndRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "out", "open")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, out, generate(t, openPackage()))
	copyFile(t, filepath.Join("testdata", "main", "open_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	args := []string{dir}
	for _, c := range openCases {
		if err := os.WriteFile(filepath.Join(dir, c.name+".json"), []byte(c.text), 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, c.kind+":"+c.name+".json")
	}
	for _, got := range buildAndRun(t, dir, []string{"main.cpp", "out/open/open.gen.cpp"}, args...) {
		sections := typesSections(got)
		for _, c := range openCases {
			if body := sections[c.kind+":"+c.name+".json"]; !strings.Contains(body, c.want) {
				t.Errorf("%s: output lacks %q:\n%s", c.name, c.want, body)
			}
		}
	}
}

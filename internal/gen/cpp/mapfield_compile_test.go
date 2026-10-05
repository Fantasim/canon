package cppgen_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mapFieldOrder is the wire order of Board's fields; mapRow fills each one a case does not give with an empty map.
var mapFieldOrder = []string{"power", "names", "levels", "groups", "slots", "ranks", "tones", "byLit", "moods", "small", "byBoard", "byNode", "deep", "rankSlots", "elemSlots", "$totals", "$byBoardTotals", "$perTone"}

// mapRow is a board's JSON object: id, then every field of mapFieldOrder, `{}` unless a case gives it as a name, value pair.
func mapRow(id string, pairs ...string) string {
	given := map[string]string{}
	for i := 0; i < len(pairs); i += 2 {
		given[pairs[i]] = pairs[i+1]
	}
	var b strings.Builder
	fmt.Fprintf(&b, `{"$id": %q`, id)
	for _, f := range mapFieldOrder {
		v, ok := given[f]
		if !ok {
			v = "{}"
			if f == "$perTone" {
				v = `{"plain": {}, "series-1": {}}`
			}
		}
		fmt.Fprintf(&b, `, %q: %s`, f, v)
	}
	if v, ok := given["extra"]; ok {
		fmt.Fprintf(&b, `, "extra": %s`, v)
	}
	return b.String() + "}"
}

// mapCases: WIRE.md §5.8's map in file order (water before fire, 10 before -3), its keys of every kind (an integer, an enum, an enum by its @json(codes) code, a String, a literal union, a ref), list, record and map values, a stored fn's map, an empty map and an absent optional one, and the refusals at the place they occur, the path showing each key as the file wrote it (an empty one adds nothing).
var mapCases = []tableFileCase{
	{"good", mapRow("b1", "power", `{"water": 2, "fire": 5}`, "names", `{"z": "Z", "a": "A"}`, "levels", `{"10": "ten", "-3": "minus", "2": "two"}`,
		"groups", `{"b": [1, 2], "a": []}`, "slots", `{"y": {"n": 1, "home": "b2"}, "x": {"n": 2, "home": "b1"}}`, "extra", `{"k": 7}`,
		"ranks", `{"200": 1, "1": 2}`, "tones", `{"series-1": 3, "all": 4, "plain": 5}`, "byLit", `{"all": {"n": 5, "home": "b2"}}`, "moods", `{"a": "all", "b": "series-1"}`, "small", `{"-5": "m", "7": "p"}`,
		"byBoard", `{"b2": 1, "b1": 2}`, "byNode", `{"-2": "x", "5": "y"}`, "deep", `{"q": {"9": "n", "1": "o"}, "p": {}}`,
		"rankSlots", `{"200": {"n": 3, "home": "b1"}}`, "elemSlots", `{"water": {"n": 4, "home": "b2"}}`, "$totals", `{"z": 1, "a": 2}`, "$byBoardTotals", `{"b2": 7, "b1": 8}`, "$perTone", `{"plain": {"b1": 1}, "series-1": {"b2": 2, "b1": 3}}`) + ", " + mapRow("b2"),
		"b1[power=water=2,fire=5 names=z=Z,a=A levels=10=ten,-3=minus,2=two groups=b=[1,2],a=[] slots=y:1->b2,x:2->b1 extra=k=7 ranks=200=1,1=2 tones=series-1=3,all=4,plain=5 byLit=all:5->b2 moods=a=all,b=series-1 small=-5=m,7=p byBoard=b2=1,b1=2 byNode=-2=x,5=y deep=q=[9=n,1=o],p=[] rankSlots=200:3->b1 elemSlots=water:4->b2 totals=z=1,a=2 byBoardTotals=b2=7,b1=8 perTone=plain:b1=1;series-1:b2=2,b1=3] " +
			"b2[power= names= levels= groups= slots= extra=none ranks= tones= byLit= moods= small= byBoard= byNode= deep= rankSlots= elemSlots= totals= byBoardTotals= perTone=plain:;series-1:]"},
	{"intkey", mapRow("b1", "levels", `{"01": "x"}`), `error good: rows[0].levels.01: expected a canonical decimal integer key`},
	{"plus", mapRow("b1", "levels", `{"+1": "x"}`), `error good: rows[0].levels.+1: expected a canonical decimal integer key`},
	{"negzero", mapRow("b1", "small", `{"-0": "x"}`), `error good: rows[0].small.-0: expected a canonical decimal integer key`},
	{"emptykey", mapRow("b1", "small", `{"": "x"}`), `error good: rows[0].small: expected a canonical decimal integer key`},
	{"emptyname", mapRow("b1", "names", `{"": 5}`), `error good: rows[0].names: expected a string`},
	{"range", mapRow("b1", "small", `{"300": "x"}`), `error good: rows[0].small.300: expected an integer from -128 to 127`},
	{"huge", mapRow("b1", "levels", `{"99999999999999999999": "x"}`), `error good: rows[0].levels.99999999999999999999: expected an integer`},
	{"enumkey", mapRow("b1", "power", `{"earth": 1}`), "error good: rows[0].power.earth: unknown value earth"},
	{"rankcode", mapRow("b1", "ranks", `{"3": 1}`), "error good: rows[0].ranks.3: unknown value 3"},
	{"rankname", mapRow("b1", "ranks", `{"low": 1}`), "error good: rows[0].ranks.low: expected a canonical decimal integer key"},
	{"tonekey", mapRow("b1", "tones", `{"loud": 1}`), "error good: rows[0].tones.loud: unknown value loud"},
	{"moodvalue", mapRow("b1", "moods", `{"a": "loud"}`), "error good: rows[0].moods.a: unknown value loud"},
	{"resultkey", mapRow("b1", "$byBoardTotals", `{"zz": 1}`), "error good: rows[0].$byBoardTotals.zz: no entry zz"},
	{"litnohome", mapRow("b1", "byLit", `{"all": {"n": 1, "home": "zz"}}`), "error good: rows[0].byLit.all.home: no entry zz"},
	{"pertone", mapRow("b1", "$perTone", `{"plain": {}, "series-1": {"zz": 1}}`), "error good: rows[0].$perTone.series-1.zz: no entry zz"},
	{"boardkey", mapRow("b1", "byBoard", `{"zz": 1}`), "error good: rows[0].byBoard.zz: no entry zz"},
	{"value", mapRow("b1", "power", `{"fire": "hot"}`), "error good: rows[0].power.fire: expected an integer"},
	{"deepkey", mapRow("b1", "deep", `{"q": {"x": "y"}}`), "error good: rows[0].deep.q.x: expected a canonical decimal integer key"},
	{"deepvalue", mapRow("b1", "deep", `{"q": {"1": 2}}`), "error good: rows[0].deep.q.1: expected a string"},
	{"totalskey", mapRow("b1", "$totals", `{"a": "x"}`), "error good: rows[0].$totals.a: expected an integer"},
	{"array", mapRow("b1", "power", `[]`), "error good: rows[0].power: expected an object"},
	{"dup", mapRow("b1", "power", `{"fire": 1, "fire": 2}`), "error good: rows[0].power.fire: duplicate key"},
	{"nohome", mapRow("b1", "slots", `{"s": {"n": 1, "home": "nowhere"}}`), "error good: rows[0].slots.s.home: no entry nowhere"},
	{"ranknohome", mapRow("b1", "rankSlots", `{"200": {"n": 1, "home": "nowhere"}}`), "error good: rows[0].rankSlots.200.home: no entry nowhere"},
	{"elemnohome", mapRow("b1", "elemSlots", `{"water": {"n": 1, "home": "nowhere"}}`), "error good: rows[0].elemSlots.water.home: no entry nowhere"},
}

// CODEGEN.md §5.9, §5.8, WIRE.md §5.8, DECISIONS 312: a data loader compiles and reads map fields in file order, keys by their wire form, and refuses what its type does not read.
func TestMapFieldDataCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, mapsPackage()), "maps_main.cpp")
	args := []string{dir}
	var want strings.Builder
	for _, c := range mapCases {
		body := `{"$schema": "` + mapsSchema + `", "rows": [` + c.body + `]}`
		if err := os.WriteFile(filepath.Join(dir, c.name+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, c.name+".json")
		fmt.Fprintf(&want, "%s.json: %s\n", c.name, strings.Replace(c.want, "error good:", "error "+c.name+".json:", 1))
	}
	for _, out := range buildAndRun(t, dir, []string{"main.cpp", "maps.gen.cpp"}, args...) {
		if out != want.String() {
			t.Errorf("got:\n%s\nwant:\n%s", out, want.String())
		}
	}
}

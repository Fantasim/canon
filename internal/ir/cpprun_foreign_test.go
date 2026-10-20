package ir_test

import (
	"strings"
	"testing"
)

const (
	cppRunForeign = "testdata/e2e/cpprun/foreign_keys.txtar"
	// foreignData is the data file dt's json emit writes, under the build's directory.
	foreignData = "dt/out/offer.json"
	// foreignOffer is an Offer whose every key names a shelf or is the literal, and foreignShown how the probe prints it.
	foreignOffer = `{"s": "a", "u": "x", "uo": "b", "ul": ["a", "x"], "uk": {"a": 1, "x": 2}, "os": [null, "b"], "ou": [null, "x", "a"], "mv": {"m": "a"}, "mk": {"b": 3}, "nl": [["a"], []], "pl": ["b"]}`
	foreignShown = "s=a u=x uo=b ul=[a,x] uk={a=1,x=2} os=[none,b] ou=[none,x,a] mv={m=a} mk={b=3} nl=[[a],[]] pl=[b]"
	// foreignUnions are foreignOffer's unions each naming no shelf, which a types reader reads unchecked, and foreignUnionsShown how the probe prints them.
	foreignUnions      = `{"s": "a", "u": "zz", "uo": "zz", "ul": ["zz"], "uk": {"zz": 1}, "os": [], "ou": ["zz"], "mv": {}, "mk": {}, "nl": [], "pl": []}`
	foreignUnionsShown = "s=a u=zz uo=zz ul=[zz] uk={zz=1} os=[] ou=[zz] mv={} mk={} nl=[] pl=[]"
)

// foreignKeyCase is foreignOffer with one position naming no shelf, from → to, and where the error names it.
type foreignKeyCase struct {
	name, from, to, at string
	union              bool // a union's key, which only a data reader checks (CODEGEN.md §5.13)
}

// foreignKeyCases: every position of CODEGEN.md §2.8's Refs, plain refs and unions (DECISIONS 387), optional elements included (log-2026-10-20).
var foreignKeyCases = []foreignKeyCase{
	{"s.json", `"s": "a"`, `"s": "zz"`, "s", false},
	{"u.json", `"u": "x"`, `"u": "zz"`, "u", true},
	{"uo.json", `"uo": "b"`, `"uo": "zz"`, "uo", true},
	{"ul.json", `"ul": ["a", "x"]`, `"ul": ["a", "zz"]`, "ul[1]", true},
	{"uk.json", `"uk": {"a": 1`, `"uk": {"zz": 1`, "uk.zz", true},
	{"os.json", `"os": [null, "b"]`, `"os": [null, "zz"]`, "os[1]", false},
	{"ou.json", `"ou": [null, "x", "a"]`, `"ou": [null, "x", "zz"]`, "ou[2]", true},
	{"mv.json", `"mv": {"m": "a"}`, `"mv": {"m": "zz"}`, "mv.m", false},
	{"mk.json", `"mk": {"b": 3}`, `"mk": {"zz": 3}`, "mk.zz", false},
	{"nl.json", `"nl": [["a"], []]`, `"nl": [[], ["zz"]]`, "nl[1][0]", false},
	{"pl.json", `"pl": ["b"]`, `"pl": ["zz"]`, "pl[0]", false},
}

// TestCppForeignKeysRun is CODEGEN.md §2.8 (Refs) and DECISIONS 387 compiled and run: a reader of another package's class refuses a ref naming no entry of the owner's keyed list in every position, its data reader a union's key too, with the loader text `no entry <key>`.
func TestCppForeignKeysRun(t *testing.T) {
	t.Parallel()
	cppRunCheck(t, cppRunForeign, func(dir string) []cppRunFile {
		head := `{"$schema": "` + cppRunSchema(t, dir, foreignData) + `", "value": `
		files := []cppRunFile{
			{name: "data:../" + foreignData, want: foreignShown},
			{name: "book.json", body: `{"offer": ` + foreignOffer + "}", want: "decode " + foreignShown + " | parse " + foreignShown},
			{name: "unions.json", body: `{"offer": ` + foreignUnions + "}", want: "decode " + foreignUnionsShown + " | parse " + foreignUnionsShown},
		}
		for _, c := range foreignKeyCases {
			offer := strings.Replace(foreignOffer, c.from, c.to, 1)
			files = append(files, cppRunFile{name: c.name, body: head + offer + "}", quiet: true},
				cppRunFile{name: "data:" + c.name, want: "error " + c.name + ": value." + c.at + ": no entry zz"})
			if !c.union {
				want := "error Book: $.offer." + c.at + ": no entry zz"
				files = append(files, cppRunFile{name: "t_" + c.name, body: `{"offer": ` + offer + "}", want: "decode " + want + " | parse " + want})
			}
		}
		return files
	})
}

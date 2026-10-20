package ir_test

import (
	"cmp"
	"strings"
	"testing"
)

const (
	cppRunUnions = "testdata/e2e/cpprun/ref_unions.txtar"
	// unionsData and unionsPaints are the data files ud's json emit writes, under the build's directory.
	unionsData   = "ud/out/exchanges.json"
	unionsPaints = "ud/out/paints.json"
	// unionsRows and unionsPaintRows are the containers ub bakes and ud's json emit writes, as the probe prints them.
	unionsRows = "give=b take=PENYA alias=PENYA all=[a,PENYA] counts={b=1,PENYA=2} maybe=[none,b,PENYA] refs=[b,none]; " +
		"give=PENYA take=none alias=a all=[] counts={} maybe=[] refs=[]"
	unionsPaintRows = "c=red p=GREEN ps=[red,none] by={GREEN=1,none=2}; c=GREEN p=none ps=[] by={}"
	// unionsRowA, unionsRowB, unionsPaintA and unionsPaintB are those rows in a data file, each first row's keys replaced where a case gives them.
	unionsRowA     = `{"$id": "a", "give": "b", "take": "PENYA", "alias": "PENYA", "all": ["a", "PENYA"], "counts": {"b": 1, "PENYA": 2}, "maybe": [null, "b", "PENYA"], "refs": ["b", null]}`
	unionsRowB     = `{"$id": "b", "give": "PENYA", "take": null, "alias": "a", "all": [], "counts": {}, "maybe": [], "refs": []}`
	unionsPaintA   = `{"c": "red", "p": "GREEN", "ps": ["red", "none"], "by": {"GREEN": 1, "none": 2}}`
	unionsPaintB   = `{"c": "GREEN", "p": "none", "ps": [], "by": {}}`
	unionsDataArg  = "data:"
	unionsPaintArg = "paints:"
)

// unionDataCase is a data file whose first row names, at one position, a key no entry has.
type unionDataCase struct{ name, from, to, want string }

// unionDataCases: refused with the ref check's text where a plain ref is checked (CODEGEN.md §5.8, DECISIONS 312, 387), and at each present element of a list of optional unions or refs (log-2026-10-20).
var unionDataCases = []unionDataCase{
	{"give.json", `"give": "b"`, `"give": "zz"`, "rows[0].give: no entry zz"},
	{"take.json", `"take": "PENYA"`, `"take": "zz"`, "rows[0].take: no entry zz"},
	{"alias.json", `"alias": "PENYA"`, `"alias": "zz"`, "rows[0].alias: no entry zz"},
	{"all.json", `"all": ["a", "PENYA"]`, `"all": ["a", "zz"]`, "rows[0].all[1]: no entry zz"},
	{"counts.json", `"counts": {"b": 1`, `"counts": {"zz": 1`, "rows[0].counts.zz: no entry zz"},
	{"maybe.json", `"maybe": [null, "b", "PENYA"]`, `"maybe": [null, "zz", "PENYA"]`, "rows[0].maybe[1]: no entry zz"},
	{"refs.json", `"refs": ["b", null]`, `"refs": ["zz", null]`, "rows[0].refs[0]: no entry zz"},
}

// unionPaintCases: a union over a keyed list keyed by an enum reads its key with the enum's FromWire, so a member's name that is not its wire names no entry (DECISIONS 387).
var unionPaintCases = []unionDataCase{
	{"paint_p.json", `"p": "GREEN"`, `"p": "green"`, "rows[0].p: no entry green"},
	{"paint_ps.json", `"ps": ["red", "none"]`, `"ps": ["zz", "none"]`, "rows[0].ps[0]: no entry zz"},
	{"paint_by.json", `"by": {"GREEN": 1`, `"by": {"zz": 1`, "rows[0].by.zz: no entry zz"},
}

// unionTypesCases are types files: a types-mode decoder reads a key unchecked (CODEGEN.md §5.13).
var unionTypesCases = []mapCase{
	{"types.json", `{"give": "zz", "take": "a", "alias": "PENYA", "all": ["PENYA", "q"], "counts": {"z": 1, "PENYA": 2}, "maybe": ["zz", null], "refs": [null, "q"]}`,
		"give=zz take=a alias=PENYA all=[PENYA,q] counts={PENYA=2,z=1} maybe=[zz,none] refs=[none,q]",
		"give=zz take=a alias=PENYA all=[PENYA,q] counts={z=1,PENYA=2} maybe=[zz,none] refs=[none,q]"},
	{"kind.json", `{"give": 1, "alias": "a", "all": [], "counts": {}}`, "error Exchange: $.give: expected a string", ""},
	{"p_types.json", `{"c": "red", "p": "zz", "ps": ["q"], "by": {"x": 1, "none": 2}}`, "c=red p=zz ps=[q] by={none=2,x=1}", "c=red p=zz ps=[q] by={x=1,none=2}"},
}

// unionDataFiles are the quiet data files of cases, each one's first row edited, and the probe's argument reading each through the loader arg names.
func unionDataFiles(head, rowA, rowB, arg string, cases []unionDataCase) []cppRunFile {
	var files []cppRunFile
	for _, c := range cases {
		body := head + strings.Replace(rowA, c.from, c.to, 1) + ", " + rowB + "]}"
		files = append(files, cppRunFile{name: c.name, body: body, quiet: true}, cppRunFile{name: arg + c.name, want: "error " + c.name + ": " + c.want})
	}
	return files
}

// TestCppRefUnionsRun is DECISIONS 387 (CODEGEN.md §4.1, §5.8, §5.13) compiled and run: a literal union over a ref is a std::string in baked, data and types mode; the data loader accepts a literal or checks the key as a plain ref, an enum key through its FromWire, the types decoder reads it unchecked.
func TestCppRefUnionsRun(t *testing.T) {
	t.Parallel()
	cppRunCheck(t, cppRunUnions, func(dir string) []cppRunFile {
		files := []cppRunFile{
			{name: "baked", want: unionsRows + " || " + unionsPaintRows},
			{name: unionsDataArg + "../" + unionsData, want: unionsRows},
			{name: unionsPaintArg + "../" + unionsPaints, want: unionsPaintRows},
		}
		head := func(rel string) string { return `{"$schema": "` + cppRunSchema(t, dir, rel) + `", "rows": [` }
		files = append(files, unionDataFiles(head(unionsData), unionsRowA, unionsRowB, unionsDataArg, unionDataCases)...)
		files = append(files, unionDataFiles(head(unionsPaints), unionsPaintA, unionsPaintB, unionsPaintArg, unionPaintCases)...)
		for _, c := range unionTypesCases {
			files = append(files, cppRunFile{name: c.name, body: c.body, want: "decode " + c.decode + " | parse " + cmp.Or(c.parse, c.decode)})
		}
		return files
	})
}

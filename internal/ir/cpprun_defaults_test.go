package ir_test

import "testing"

const (
	cppRunDefaults = "testdata/e2e/cpprun/defaults.txtar"
	// defaultsAll is what the probe prints of a Marker after its name when every other key is absent: each field's default.
	defaultsAll = " at=0:0 maybe=2:0 shape=circle3 all=[4:0,0:5] slots=[a=6,b=1] origin=7:0 deep=8:0 band=" + defaultsBand
	// defaultsBand is lib's Band when its keys are all absent, as d's reader builds it.
	defaultsBand = "1:0/[0:2]/[s=3:0]"
)

// defaultCases: every default kind DECISIONS 386 builds, taken where its key is absent, the intermediate object of a path included (WIRE.md §5.5.3), and the file's own values where present (CODEGEN.md §5.13).
var defaultCases = []mapCase{
	{"absent.json", `{"name": "a"}`, "name=a" + defaultsAll, ""},
	{"given.json", `{"name": "g", "at": {"x": 1}, "maybe": null, "shape": {"kind": "dot"}, "all": [], "slots": {}, "origin": {"y": 9}, "under": {"at": {"y": 2}}}`,
		"name=g at=1:0 maybe=none shape=dot all=[] slots=[] origin=0:9 deep=0:2 band=" + defaultsBand, ""},
	{"band.json", `{"name": "b", "band": {"at": {"y": 3}, "named": {"t": {}}}}`, "name=b at=0:0 maybe=2:0 shape=circle3 all=[4:0,0:5] slots=[a=6,b=1] origin=7:0 deep=8:0 band=0:3/[0:2]/[t=0:0]", ""},
	{"under.json", `{"name": "u", "under": {}, "slots": {"c": {}}, "maybe": {"y": 1}}`, "name=u at=0:0 maybe=0:1 shape=circle3 all=[4:0,0:5] slots=[c=1] origin=7:0 deep=8:0 band=" + defaultsBand, ""},
	{"missing.json", `{}`, "error Marker: $.name: missing", ""},
	{"shape.json", `{"name": "s", "shape": {"kind": "circle"}}`, "error Marker: $.shape.r: missing", ""},
}

// TestCppTypesDefaultsRun is DECISIONS 386 (CODEGEN.md §5.13) compiled and run: a cpp types-mode decoder copies a default holding a record, a variant, a list of records, a table or another package's record from a function-local static const its baked filler's statements build, so a value moved away leaves the next decode its default.
func TestCppTypesDefaultsRun(t *testing.T) {
	t.Parallel()
	files := []cppRunFile{{name: "twice", want: "moved name=t" + defaultsAll + " | again name=t" + defaultsAll}}
	for _, c := range defaultCases {
		files = append(files, cppRunFile{name: c.name, body: c.body, want: "decode " + c.decode + " | parse " + c.decode})
	}
	cppRunCheck(t, cppRunDefaults, func(string) []cppRunFile { return files })
}

package ir_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	nestedRefsCpp = "testdata/e2e/cpprun/nested_refs.txtar"
	nestedRefsOut = "nr/out"
	nestedRefsExt = ".json"
	nestedHolder  = "h"
)

// nestedRefsFiles are the data files of nested_refs.txtar's lets, each the file the json emit writes.
var nestedRefsFiles = []string{"aa", "bs", "cs", "ss", "ts", "us", "ks", "ls", "ms", "byItem", "over", nestedHolder}

// nestedRefsCase is the holder file with from replaced by to, one position at naming a key its chain lacks, and the key the loader reports.
type nestedRefsCase struct{ name, from, to, at, key string }

// nestedRefsCases: the holder's refs to the end of each chain, named by the key that ends it: an enum's wire (Tone.second is "z2", Tone.fourth "w4", and the good data holds second), a String, a table's id, the id enum of another package's table (axe is an item over lacks), and a union over the enum chain, whose text is read as a wire (CODEGEN.md §5.3, §5.8, §5.9, DECISIONS 387, 456).
var nestedRefsCases = []nestedRefsCase{
	{"good", "", "", "", ""},
	{"enum_one", `"e": "z2"`, `"e": "w4"`, "e", "w4"},
	{"enum_many", "\"first\",\n      \"z2\"\n    ],\n    \"em\"", "\"first\",\n      \"w4\"\n    ],\n    \"em\"", "es[1]", "w4"},
	{"enum_key", `"z2": 1`, `"w4": 1`, "em.w4", "w4"},
	{"string_one", `"s": "p"`, `"s": "zz"`, "s", "zz"},
	{"string_many", "\"p\",\n      \"q\"\n    ],\n    \"sm\"", "\"p\",\n      \"zz\"\n    ],\n    \"sm\"", "ss[1]", "zz"},
	{"string_key", `"p": 1`, `"zz": 1`, "sm.zz", "zz"},
	{"table_one", `"k": "alpha"`, `"k": "zz"`, "k", "zz"},
	{"table_many", "\"alpha\",\n      \"beta\"\n    ],\n    \"km\"", "\"alpha\",\n      \"zz\"\n    ],\n    \"km\"", "ks[1]", "zz"},
	{"table_key", `"alpha": 1`, `"zz": 1`, "km.zz", "zz"},
	{"union_one", `"u": "z2"`, `"u": "w4"`, "u", "w4"},
	{"union_name", `"u": "z2"`, `"u": "second"`, "u", "second"},
	{"union_many", "\"none\",\n      \"z2\"\n    ],\n    \"f\"", "\"none\",\n      \"w4\"\n    ],\n    \"f\"", "us[2]", "w4"},
	{"foreign_one", `"f": "sword"`, `"f": "axe"`, "f", "axe"},
	{"foreign_many", "\"shield\",\n      \"sword\"\n    ],\n    \"fm\"", "\"shield\",\n      \"axe\"\n    ],\n    \"fm\"", "fs[1]", "axe"},
	{"foreign_key", `"sword": 1`, `"axe": 1`, "fm.axe", "axe"},
	{"foreign_union_one", `"fu": "sword"`, `"fu": "axe"`, "fu", "axe"},
	{"foreign_union_name", `"fu": "sword"`, `"fu": "zz"`, "fu", "zz"},
	{"foreign_union_many", "\"none\",\n      \"shield\"\n    ]", "\"none\",\n      \"axe\"\n    ]", "fus[1]", "axe"},
}

// want is what the probe prints for c: the loader's error, which names the file and the position under the value.
func (c nestedRefsCase) want() string {
	if c.at == "" {
		return "ok"
	}
	return "error " + c.name + "/h.json: value." + c.at + ": no entry " + c.key
}

// TestCppNestedRefsRun is CODEGEN.md §5.8, §5.9 and DECISIONS 456 compiled and run: a data loader with @reload resolves a ref whose list is keyed by a ref, three deep, and names a key the chain lacks by the type that keys the chain's end, an enum's wire, a String or a table's id, at a field, a list element and a map key.
func TestCppNestedRefsRun(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("compiles and runs generated code")
	}
	dir := cppRunBuild(t, nestedRefsCpp)
	cases := filepath.Join(dir, cppRunCases)
	var args []string
	var want strings.Builder
	for _, c := range nestedRefsCases {
		nestedRefsWrite(t, dir, filepath.Join(cases, c.name), c)
		args = append(args, c.name)
		want.WriteString(c.name + ": " + c.want() + "\n")
	}
	for _, got := range cppRunOutputs(t, dir, args) {
		if got != want.String() {
			t.Errorf("got:\n%s\nwant:\n%s", got, want.String())
		}
	}
}

// nestedRefsWrite writes the data files of the build in dir to out, the holder's with c applied.
func nestedRefsWrite(t *testing.T, dir, out string, c nestedRefsCase) {
	t.Helper()
	if err := os.MkdirAll(out, e2eDirMode); err != nil {
		t.Fatal(err)
	}
	for _, f := range nestedRefsFiles {
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(nestedRefsOut), f+nestedRefsExt))
		if err != nil {
			t.Fatal(err)
		}
		body := string(raw)
		if f == nestedHolder && c.from != "" {
			if !strings.Contains(body, c.from) {
				t.Fatalf("%s: %q is not in\n%s", c.name, c.from, body)
			}
			body = strings.Replace(body, c.from, c.to, 1)
		}
		if err := os.WriteFile(filepath.Join(out, f+nestedRefsExt), []byte(body), e2eFileMode); err != nil {
			t.Fatal(err)
		}
	}
}

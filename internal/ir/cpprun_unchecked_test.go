package ir_test

import (
	"strings"
	"testing"
)

const (
	cppRunUnchecked = "testdata/e2e/cpprun/unchecked_keys.txtar"
	uncheckedData   = "ud/out/exchanges.json"
	// uncheckedRowA and uncheckedRowB are the rows of that file, every position of row b holding the key a.
	uncheckedRowA = `{"$id": "a", "n": 1, "p": "PENYA", "l": [], "lo": [], "lr": [], "mv": {}, "mu": {}, "mr": {}, "ml": {}, "nl": [], "nr": []}`
	uncheckedRowB = `{"$id": "b", "n": 2, "p": "a", "l": ["a", "PENYA"], "lo": [null, "a"], "lr": ["a", null], "mv": {"k": "a"}, "mu": {"k": "a"}, "mr": {"k": "a"}, "ml": {"k": ["a"]}, "nl": [["a"]], "nr": [["a", null]]}`
)

// uncheckedCase is the file with one position holding zz for the key a, and where a loader that checks it names the error.
type uncheckedCase struct{ name, from, to, at string }

// uncheckedOpen: the positions where a plain ref is a key only (CODEGEN.md §5.8), read unchecked.
var uncheckedOpen = []uncheckedCase{
	{"mv", `"mv": {"k": "a"}`, `"mv": {"k": "zz"}`, ""},
	{"mu", `"mu": {"k": "a"}`, `"mu": {"k": "zz"}`, ""},
	{"mr", `"mr": {"k": "a"}`, `"mr": {"k": "zz"}`, ""},
	{"ml", `"ml": {"k": ["a"]}`, `"ml": {"k": ["zz"]}`, ""},
	{"nl", `"nl": [["a"]]`, `"nl": [["zz"]]`, ""},
	{"nr", `"nr": [["a", null]]`, `"nr": [["zz", null]]`, ""},
}

// uncheckedShut: the positions where a plain ref is resolved, checked (DECISIONS 385, 387).
var uncheckedShut = []uncheckedCase{
	{"p", `"p": "a"`, `"p": "zz"`, "p"},
	{"l", `"l": ["a", "PENYA"]`, `"l": ["a", "zz"]`, "l[1]"},
	{"lo", `"lo": [null, "a"]`, `"lo": [null, "zz"]`, "lo[1]"},
	{"lr", `"lr": ["a", null]`, `"lr": ["zz", null]`, "lr[0]"},
}

// TestCppUncheckedKeysRun is CODEGEN.md §5.8 and DECISIONS 312, 385, 387 compiled and run: a cpp data loader reads a union or an optional ref under a map value or at a second list level unchecked, as a plain ref there is a key only, and checks one at a field's top or one list level deep; the go loader does the same (TestGoUncheckedKeysRun).
func TestCppUncheckedKeysRun(t *testing.T) {
	t.Parallel()
	cppRunCheck(t, cppRunUnchecked, func(dir string) []cppRunFile {
		head := `{"$schema": "` + cppRunSchema(t, dir, uncheckedData) + `", "rows": [`
		files := []cppRunFile{{name: "data:../" + uncheckedData, want: "ok"}}
		for _, c := range append(append([]uncheckedCase{}, uncheckedOpen...), uncheckedShut...) {
			body := strings.Replace(head+uncheckedRowA+", "+uncheckedRowB+"]}", c.from, c.to, 1)
			want := "ok"
			if c.at != "" {
				want = "error " + c.name + ".json: rows[1]." + c.at + ": no entry zz"
			}
			files = append(files, cppRunFile{name: c.name + ".json", body: body, quiet: true}, cppRunFile{name: "data:" + c.name + ".json", want: want})
		}
		return files
	})
}

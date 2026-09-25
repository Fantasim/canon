package cppgen_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dependentCase is one data file of dependentPackage's Event and the driver's line for it.
type dependentCase struct{ name, value, want string }

// dependentCases: each branch, two branches of one storage type apart (emplace by index), an arm
// of two members, a Never arm absent and written, a discriminant through a held record, a list.
var dependentCases = []dependentCase{
	{"ok1", `"active": false, "payload": "hello", "kind3": "a", "multi": "first", "et": {"param": "a"}, "deep": true, "many": ["x", "y"]`,
		"payload=false:hello multi=0:a:first deep=a:true many= false:x false:y"},
	{"ok2", `"active": true, "payload": "despawn", "kind3": "b", "multi": "second", "et": {"param": "c"}, "deep": 1.5, "many": ["spawn"]`,
		"payload=true:despawn multi=1:b:second deep=b:1.500000 many= true:spawn"},
	{"ok3", `"active": true, "payload": "spawn", "kind3": "c", "multi": 7, "et": {"param": "e"}, "deep": 2, "many": []`,
		"payload=true:spawn multi=2:c:7 deep=b:2.000000 many="},
	{"ok4", `"active": false, "payload": "", "kind3": "e", "multi": -3, "et": {"param": "d"}, "deep": 0.25, "many": []`,
		"payload=false: multi=2:c:-3 deep=b:0.250000 many="},
	{"neverwritten", `"active": false, "payload": "p", "kind3": "d", "multi": null, "et": {"param": "a"}, "deep": false, "many": []`,
		"payload=false:p multi=none deep=a:false many="},
	{"badenum", `"active": true, "payload": "nope", "kind3": "a", "multi": "m", "et": {"param": "a"}, "deep": true, "many": []`,
		"error badenum.json: value.payload: unknown value nope"},
	{"badint", `"active": true, "payload": "spawn", "kind3": "c", "multi": 99999999999, "et": {"param": "a"}, "deep": true, "many": []`,
		"error badint.json: value.multi: expected an integer from -2147483648 to 2147483647"},
	{"badbranch", `"active": true, "payload": "spawn", "kind3": "d", "multi": 5, "et": {"param": "a"}, "deep": true, "many": []`,
		"error badbranch.json: value.multi: no branch for this value"},
}

// CODEGEN.md §5.6, §7.6; WIRE.md §5.9: dependent fields read by the loader, the discriminant first; a Never branch takes only none.
func TestDependentValueCompilesAndRuns(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, generate(t, dependentPackage()), "dependent_main.cpp")
	data := filepath.Join(dir, "data")
	var want strings.Builder
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range dependentCases {
		body := `{"$schema": "demo.Event@00000001", "value": {` + c.value + `}}`
		if err := os.WriteFile(filepath.Join(data, c.name+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&want, "%s: %s\n", c.name, c.want)
	}
	for _, out := range buildAndRun(t, dir, []string{"main.cpp", "demo.gen.cpp"}, data) {
		if out != want.String() {
			t.Errorf("got:\n%s\nwant:\n%s", out, want.String())
		}
	}
}

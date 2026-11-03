package ir_test

import (
	"strings"
	"testing"
)

const (
	nestedBranchesCpp    = "testdata/e2e/cpprun/nested_branches.txtar"
	nestedBranchesSchema = "bd/out/rewards.json"
	nestedBranchesData   = ".data"
	nestedBranchesBase   = "ok fire=count:3/5=count:2 w=place:x/20=place:y earth=none/5=none"
)

// nestedBranchesCase is a file of rewards and the verdict both readers give it.
type nestedBranchesCase struct{ name, json, want string }

// nestedBranchesCases: a reward whose event is a ref whose list is keyed by a ref into a list keyed by an enum picks its param's branch by the enum (DECISIONS 350, 456); a Never arm selects no branch, a value of the other branch's type is refused otherwise.
var nestedBranchesCases = []nestedBranchesCase{
	{"good", `{"rewards": [{"ev": "fire", "param": 3, "num": 5, "byNum": 2}, {"ev": "w", "param": "x", "num": 20, "byNum": "y"}, {"ev": "earth", "param": 0, "num": 5, "byNum": 0}]}`, nestedBranchesBase},
	{"never_arm", `{"rewards": [{"ev": "earth", "param": 7, "num": 5, "byNum": 1}]}`, "error no branch"},
	{"wrong_type", `{"rewards": [{"ev": "fire", "param": "x", "num": 5, "byNum": 1}]}`, "error other"},
	{"int_wrong_type", `{"rewards": [{"ev": "fire", "param": 3, "num": 20, "byNum": 4}]}`, "error other"},
}

// TestCppNestedBranchesRun is CODEGEN.md §5.6 and DECISIONS 350, 456 compiled and run in the data and the types mode: a dependent type decided through a ref whose list is keyed by a ref into a list keyed by an enum has a branch table keyed by that enum, which the loader and the decoder read, so both give the verdict of the same file.
func TestCppNestedBranchesRun(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("compiles and runs generated code")
	}
	dir := cppRunBuild(t, nestedBranchesCpp)
	schema := cppRunSchema(t, dir, nestedBranchesSchema)
	var files []cppRunFile
	var want strings.Builder
	for _, c := range nestedBranchesCases {
		name := c.name + nestedRefsExt
		wrapped := `{"$schema": "` + schema + `", "value": ` + c.json + "}\n"
		files = append(files, cppRunFile{name: name, body: c.json + "\n"}, cppRunFile{name: name + nestedBranchesData, body: wrapped, quiet: true})
		want.WriteString("types " + c.want + "\ndata " + c.want + "\n")
	}
	args := cppRunWrite(t, dir, files)
	for _, got := range cppRunOutputs(t, dir, args) {
		if got != want.String() {
			t.Errorf("got:\n%s\nwant:\n%s", got, want.String())
		}
	}
}

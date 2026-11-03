package ir_test

import (
	"strings"
	"testing"
)

const (
	foreignBranchesCpp    = "testdata/e2e/cpprun/foreign_branches.txtar"
	foreignBranchesSchema = "fd/out/rewards.json"
	foreignBranchesBase   = "ok sword=count:3/shield=place:y shield=place:x/sword=count:2 ghost=none/ghost=none"
)

// foreignBranchesCases: a reward whose event is a ref held as the id enum of another package's table, alone or through a keyed list keyed by one, picks its param's branch by the id; a Never arm selects no branch, an id the enum lacks or a value of the other branch's type is refused otherwise (DECISIONS 350, 456).
var foreignBranchesCases = []nestedBranchesCase{
	{"good", `{"rewards": [{"ev": "sword", "param": 3, "ch": "shield", "p2": "y"}, {"ev": "shield", "param": "x", "ch": "sword", "p2": 2}, {"ev": "ghost", "param": 0, "ch": "ghost", "p2": 0}]}`, foreignBranchesBase},
	{"never_arm", `{"rewards": [{"ev": "ghost", "param": 7, "ch": "sword", "p2": 1}]}`, "error no branch"},
	{"chain_never_arm", `{"rewards": [{"ev": "sword", "param": 1, "ch": "ghost", "p2": 7}]}`, "error no branch"},
	{"wrong_type", `{"rewards": [{"ev": "sword", "param": "x", "ch": "sword", "p2": 1}]}`, "error other"},
	{"chain_wrong_type", `{"rewards": [{"ev": "sword", "param": 1, "ch": "shield", "p2": 4}]}`, "error other"},
	{"unknown_id", `{"rewards": [{"ev": "axe", "param": 1, "ch": "sword", "p2": 1}]}`, "error other"},
}

// TestCppForeignBranchesRun is CODEGEN.md §5.3, §5.6 and DECISIONS 350, 456 compiled and run in the data and the types mode: a dependent type decided through a ref held as the id enum of another package's baked table, at one level and down a chain, has a branch table the loader and the decoder read, so both give the verdict of the same file.
func TestCppForeignBranchesRun(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("compiles and runs generated code")
	}
	dir := cppRunBuild(t, foreignBranchesCpp)
	schema := cppRunSchema(t, dir, foreignBranchesSchema)
	var files []cppRunFile
	var want strings.Builder
	for _, c := range foreignBranchesCases {
		name := c.name + nestedRefsExt
		wrapped := `{"$schema": "` + schema + `", "value": ` + c.json + "}\n"
		files = append(files, cppRunFile{name: name, body: c.json + "\n"}, cppRunFile{name: name + nestedBranchesData, body: wrapped, quiet: true})
		want.WriteString("types " + c.want + "\ndata " + c.want + "\n")
	}
	for _, got := range cppRunOutputs(t, dir, cppRunWrite(t, dir, files)) {
		if got != want.String() {
			t.Errorf("got:\n%s\nwant:\n%s", got, want.String())
		}
	}
}

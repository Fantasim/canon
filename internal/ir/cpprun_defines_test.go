package ir_test

import "testing"

const (
	cppRunDefines = "testdata/e2e/cpprun/defines.txtar"
	// definesShown is what the probe prints: every macro still has its value after the generated code.
	definesShown  = "macros 1 2 9 5 legacy 1 0\n"
	definesLegacy = "-DLEGACY"
)

// TestCppDefinesCompileAndRun is CODEGEN.md §7.9, DECISIONS 389-391 compiled and run: with lib's defines header, then with a hand-written header of the same macros, included before the generated headers and sources of lib (a table id, a constant, a renamed member, a member of an enum without @cpp(defines:) spelled like a macro) and of app, the TU compiles under -Werror and the macros keep their values.
func TestCppDefinesCompileAndRun(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("compiles and runs generated code")
	}
	dir := cppRunBuild(t, cppRunDefines)
	cppRunWrite(t, dir, nil)
	for name, extra := range map[string][]string{"defines header": nil, "legacy header": {definesLegacy}} { //canon:unordered each build alone
		for _, got := range cppRunSourcesOutputs(t, dir, []string{cppRunProbe}, extra, nil) {
			if got != definesShown {
				t.Errorf("%s: got %q, want %q", name, got, definesShown)
			}
		}
	}
}

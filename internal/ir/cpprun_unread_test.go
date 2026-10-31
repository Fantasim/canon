package ir_test

import "testing"

const (
	cppRunUnread  = "testdata/e2e/cpprun/defines_unread.txtar"
	cppRunReverse = "probe_reverse.cpp"
	// unreadShown is what both probes print: every macro of lib keeps its value after the generated code.
	unreadShown = "macros 0 1 9\n"
)

// TestCppDefinesUnreadCompileAndRun is CODEGEN.md §7.9, DECISIONS 403 compiled and run: a package that spells lib's macros and does not read lib renames them with @cpp(name:), one that reads lib is guarded; the project builds with no finding, and one TU that includes every defines header and then every generated header and source, and another that includes them in the reverse order, compile under -Werror and keep the macros' values.
func TestCppDefinesUnreadCompileAndRun(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("compiles and runs generated code")
	}
	dir := cppRunBuild(t, cppRunUnread)
	cppRunWrite(t, dir, nil)
	for _, probe := range []string{cppRunProbe, cppRunReverse} {
		for _, got := range cppRunSourcesOutputs(t, dir, []string{probe}, nil, nil) {
			if got != unreadShown {
				t.Errorf("%s: got %q, want %q", probe, got, unreadShown)
			}
		}
	}
}

package ir_test

import "testing"

const cppRunFileSuffix = "testdata/e2e/cpprun/file_suffix.txtar"

// TestCppFileSuffixCompileAndRun is DECISIONS 407 (CODEGEN.md §5.13) compiled and run: tipFile gives DecodeTipFile under -Werror, it decodes a file and its error names TipFile.
func TestCppFileSuffixCompileAndRun(t *testing.T) {
	t.Parallel()
	files := []cppRunFile{
		{name: "good.json", body: `[{"n": 1}, {"n": 2}]`, want: "n1n2"},
		{name: "bad.json", body: `[{}]`, want: "error TipFile: $[0].n: missing"},
	}
	cppRunCheck(t, cppRunFileSuffix, func(string) []cppRunFile { return files })
}

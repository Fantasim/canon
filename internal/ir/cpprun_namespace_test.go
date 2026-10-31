package ir_test

import "testing"

const cppRunNamespace = "testdata/e2e/cpprun/shared_namespace.txtar"

// TestSharedNamespaceCompilesAndRuns is CODEGEN.md §3.5, §7.1 (DECISIONS 379), a pin: two baked and two data packages emitted into sov::gen build with no finding, compile into one translation unit under -Werror, link and run under every compiler, with both id enums still non-constructible into nlohmann::json (the probe's static_asserts).
func TestSharedNamespaceCompilesAndRuns(t *testing.T) {
	t.Parallel()
	files := []cppRunFile{
		{name: "ids", want: "two bolt 2 9"},
		{name: "lookups", want: "5 2"},
		{name: "translated", want: "42 15"},
		{name: "conformance", want: "0 0"},
		{name: "gamma:../c/out/gammaRows.json", want: "first:3"},
		{name: "delta:../d/out/deltaCells.json", want: "second:4"},
	}
	cppRunCheck(t, cppRunNamespace, func(string) []cppRunFile { return files })
}

package cppgen_test

import "testing"

// triWant is what testdata/main/tri_main.cpp prints: every path keeps the text.
const triWant = "field=1\nshort=1\nlong=1\nshortKey=1\nlongKey=1\nfind=1\nconst=1\nconstShort=1\nconstLong=1\ncell=1\n"

// CODEGEN.md §3.4, DECISIONS 423: a String value, a map key and a constant holding `??=`, `??/` and `???` are written with `\?`, so the header compiles under -Wall -Wextra -Werror (no -Wtrigraphs) and the bytes read back are unchanged.
func TestBakedTrigraphLiteralsCompileAndRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, triPackage()), "tri_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"tri.gen.cpp", "main.cpp"}) {
		if out != triWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, triWant)
		}
	}
}

// triTypesWant is what testdata/main/tri_types_main.cpp prints.
const triTypesWant = "whole: ok count=7 amount=5\ncut: refused\nwire=1\n"

// CODEGEN.md §5.13, DECISIONS 423: a field wire name and a case wire holding trigraphs compile under -Werror and are compared byte for byte.
func TestTypesTrigraphWiresCompileAndRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, triTypesPackage()), "tri_types_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"tri.gen.cpp", "main.cpp"}) {
		if out != triTypesWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, triTypesWant)
		}
	}
}

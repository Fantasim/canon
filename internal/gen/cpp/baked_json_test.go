package cppgen_test

import (
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// CODEGEN.md §5.3, DECISIONS 379: nlohmann/json neither writes nor reads a baked id enum (its underlying value is a position); ToWire and <Id>FromWire compile, and so does a plain enum (the failure is the id's deleted hooks).
func TestIDEnumRefusesNlohmann(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, driver string
		compiles     bool
	}{
		{"ToWire and FromWire", "baked_json_ok_main.cpp", true},
		{"json j = id", "baked_json_to_main.cpp", false},
		{"get<Id>()", "baked_json_from_main.cpp", false},
	}
	compilers, include := cxx.Toolchain(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeTree(t, dir, generate(t, bakedPackage()), c.driver)
			for _, cc := range compilers {
				args := append(append([]string(nil), cxx.Flags...), "-fsyntax-only", "-I", dir, "-I", include, filepath.Join(dir, "main.cpp"))
				out, err := compile(cc, args, dir)
				if c.compiles && err != nil {
					t.Errorf("%s: %v\n%s", cc, err, out)
				}
				if !c.compiles && err == nil {
					t.Errorf("%s: want a compile error, it compiled\n%s", cc, out)
				}
			}
		})
	}
}

package cppgen_test

import (
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// CODEGEN.md §5.2, §5.3, §5.5, §5.6, DECISIONS 379, 402: nlohmann/json neither writes nor reads an id enum, an enum without @codes, a kind or a branch enum (their underlying value is a position); ToWire and the FromWires compile, and so does a @codes enum (the failure is the deleted hooks).
func TestIDEnumRefusesNlohmann(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, driver string
		pkg          func() *ir.Package
		compiles     bool
	}{
		{"ToWire and FromWire", "baked_json_ok_main.cpp", bakedPackage, true},
		{"json j = id", "baked_json_to_main.cpp", bakedPackage, false},
		{"get<Id>()", "baked_json_from_main.cpp", bakedPackage, false},
		{"json j = enum", "baked_json_enum_to_main.cpp", bakedPackage, false},
		{"get<Enum>()", "baked_json_enum_from_main.cpp", bakedPackage, false},
		{"json j = kind", "baked_json_kind_main.cpp", bakedPackage, false},
		{"json j = branch", "baked_json_branch_main.cpp", bakedDependentPackage, false},
	}
	compilers, include := cxx.Toolchain(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeTree(t, dir, generate(t, c.pkg()), c.driver)
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

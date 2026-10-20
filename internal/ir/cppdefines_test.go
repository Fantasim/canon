package ir_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// cppDefinesCase is a package with @cpp(defines:) and the macros its defines header writes, in declaration order.
type cppDefinesCase struct {
	name, src string
	want      []string
}

const definesEmit = "emit cpp { out: \"@features/a\", namespace: \"a\", mode: types }\n"

var cppDefinesCases = []cppDefinesCase{
	{"empty prefix writes every member", `package a

/// Legacy kinds.
enum Kind @cpp(defines: "") {
  /// One.
  KIND_A
  /// Two.
  KIND_B
}

` + definesEmit, []string{"KIND_A", "KIND_B"}},
	{"prefix kept when present", `package a

/// Legacy kinds.
enum Kind @cpp(defines: "K_") {
  /// One.
  K_A
  /// Two.
  B
}

` + definesEmit, []string{"K_A", "K_B"}},
	{"underscore-uppercase shape", `package a

/// Legacy kinds.
enum Kind @cpp(defines: "_") {
  /// One.
  NONE
}

` + definesEmit, []string{"_NONE"}},
	{"double underscore shape", `package a

/// Legacy kinds.
enum Kind @cpp(defines: "__") {
  /// One.
  X
}

` + definesEmit, []string{"__X"}},
	{"constants and enums interleave by position", `package a

/// A limit.
@cpp(defines: "")
const MAX_A = 7

/// Legacy kinds.
enum Kind @cpp(defines: "K_") {
  /// One.
  A
}

/// Another limit.
@cpp(defines: "")
const MAX_B = true

/// A name.
@cpp(defines: "")
const NAME = "x"

/// A constant without defines.
const PLAIN = 1

` + definesEmit, []string{"MAX_A", "K_A", "MAX_B", "NAME"}},
	{"no annotation, no define", `package a

/// Legacy kinds.
enum Kind { A }

` + definesEmit, nil},
}

// CODEGEN.md §7.9, DECISIONS 389, 390: presence decides (`""` writes every member), a define names a legacy macro (the `_`-uppercase and `__` shapes are clean), and constants and enums keep declaration order; none of these is a finding.
func TestCppDefineLines(t *testing.T) {
	for _, c := range cppDefinesCases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			w.add(t, "a/a.canon", []byte(c.src))
			w.calls = w.fixtureCalls
			pkgs := w.build(t)
			if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
				t.Fatalf("must be clean:\n%s", out)
			}
			var got []string
			for _, d := range ir.CppDefineLines(pkgs[0]) {
				got = append(got, d.Name)
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("defines %v, want %v", got, c.want)
			}
		})
	}
}

// CODEGEN.md §7.9, DECISIONS 391: an import carries the macros its defines header writes, sorted, the set an importer's files wrap; the same whether the import is built or only declared.
func TestCppDefinesOfImport(t *testing.T) {
	for name, selected := range map[string][]string{"both built": nil, "import only declared": {"b"}} { //canon:unordered each case alone
		t.Run(name, func(t *testing.T) { importDefines(t, selected...) })
	}
}

// importDefines builds package b importing package a with defines, selecting selected, and checks b's import of a.
func importDefines(t *testing.T, selected ...string) {
	t.Helper()
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(`package a

/// Legacy kinds.
enum Kind @cpp(defines: "K_") {
  /// One.
  B
  /// Two.
  A
}

/// A limit.
@cpp(defines: "")
const MAX = 3

`+definesEmit))
	w.add(t, "b/b.canon", []byte(`package b

import a

/// A holder.
record Holder {
  /// Its kind.
  kind: a.Kind = a.Kind.A
}

emit cpp { out: "@features/b", namespace: "b", mode: types }
`))
	w.calls = w.fixtureCalls
	var holder *ir.Package
	for _, p := range w.build(t, selected...) {
		if p.Name == "b" {
			holder = p
		}
	}
	if holder == nil || len(holder.Imports) != 1 {
		t.Fatalf("package b: %+v", holder)
	}
	if got, want := holder.Imports[0].CppDefines, []string{"K_A", "K_B", "MAX"}; !slices.Equal(got, want) {
		t.Errorf("import defines %v, want %v", got, want)
	}
}

package ir_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// declaredCase is a package and the names its cpp emit declares for enum members and constants.
type declaredCase struct {
	name, src string
	want      []string
}

var declaredCases = []declaredCase{
	{"members and constants sorted by name", `package a

/// Kinds.
enum Kind { B, A }

/// A limit.
const MAX = 3

` + definesEmit, []string{"A", "B", "MAX"}},
	{"renamed members and constants are not listed", `package a

/// Kinds.
enum Kind { A @cpp(name: "Alpha"), B }

/// A limit.
@cpp(name: "Top")
const MAX = 3

` + definesEmit, []string{"B"}},
	{"reserved words get an underscore", `package a

/// Kinds.
enum Kind { new }

` + definesEmit, []string{"new_"}},
	{"no cpp emit, no names", `package a

/// Kinds.
enum Kind { A }

emit json { out: "out/" }
`, nil},
}

// CODEGEN.md 7.9, API.md §5.7: CppDeclaredNames lists the cpp names of enum members and constants that @cpp(name:) does not rename, sorted; a package without a cpp emit has none.
func TestCppDeclaredNames(t *testing.T) {
	for _, c := range declaredCases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			w.add(t, "a/a.canon", []byte(c.src))
			w.calls = w.fixtureCalls
			pkgs := w.build(t)
			if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
				t.Fatalf("must be clean:\n%s", out)
			}
			var got []string
			for _, d := range ir.CppDeclaredNames(pkgs[0]) {
				got = append(got, d.Name)
				if (d.Const == nil) == (d.Member == nil) || d.Member != nil && d.Enum == nil {
					t.Errorf("%s: %+v names neither a member of an enum nor a constant", d.Name, d)
				}
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("declared %v, want %v", got, c.want)
			}
		})
	}
}

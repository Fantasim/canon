package load_test

import (
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/load"
)

// WIRE.md 6.8, API.md §5.7: DefineNames lists the object-like defines once, in file order; function-like macros, comments and strings are left out.
func TestDefineNames(t *testing.T) {
	cases := []struct {
		name, header string
		want         []string
	}{
		{"values", "#define A 1\n#define B \"text\"\n#define C (1 << 2)\n#define D\n", []string{"A", "B", "C", "D"}},
		{"function-like", "#define F(x) x\n#define G (x)\n", []string{"G"}},
		{"duplicates once", "#define A 1\n#define A 1\n#define B 2\n", []string{"A", "B"}},
		{"comments", "/* #define HIDDEN 1 */\n// #define ALSO 1\n#define SHOWN 1 // #define AFTER 2\n", []string{"SHOWN"}},
		{"splice", "#define \\\nSPLIT 1\n#  define\tSPACED\t2\n", []string{"SPLIT", "SPACED"}},
		{"not a define", "#undef A\n#include <a.h>\nint define = 1;\n", nil},
		{"last line without newline", "#define LAST 1", []string{"LAST"}},
	}
	for _, c := range cases {
		if got := load.DefineNames([]byte(c.header)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("WIRE.md §6.8, %s: DefineNames = %v, want %v", c.name, got, c.want)
		}
	}
}

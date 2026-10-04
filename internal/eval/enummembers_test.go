package eval_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
)

// maxProbeBudget bounds minBudget's search.
const maxProbeBudget = 64

// enumHead declares an enum with a retired member, and one with codes, for E.members.
const enumHead = `/// A.
package a

local enum E { a, retired b, c }

local enum K @codes(UInt8) { x = 7, retired y = 1, z = 3 }

`

// STDLIB.md §3, DECISIONS 299: E.members in declaration order, retired included; computing with them is allowed.
func TestEnumMembers(t *testing.T) {
	for _, c := range []struct{ decl, want string }{
		{"local let x: Int = E.members.len()", "3"},
		{"local let x: [String] = E.members.map(m => m.name)", `["a", "b", "c"]`},
		{"local let x: [Bool] = E.members.map(m => m.retired)", "[false, true, false]"},
		{"local let x: [Int] = E.members.map(m => m.index)", "[0, 1, 2]"},
		{"local let x: [String] = E.members.filter(m => not m.retired).map(m => m.name)", `["a", "c"]`},
		{"local let x: [E] = E.members.filter(m => not m.retired)", "[a, c]"},
		{"local let x: [Int] = K.members.map(.code)", "[7, 1, 3]"},
		{"local let x: Bool = E.c.retired", "false"},
		{"local let x: String = E.members.map(m => \"{m.name}={m.index}\").join(\",\")", "a=0,b=1,c=2"},
		{"local const X = E.members.len()\nlocal let x: Int = X", "3"},
	} {
		t.Run(c.decl, func(t *testing.T) {
			b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(enumHead + c.decl + "\n")}), eval.Options{})
			v, ok := b.values[eval.Root{Pkg: "a", Name: "x"}]
			if !ok || v.CanonText() != c.want {
				t.Errorf("got %v %t, want %s\n%s", v, ok, c.want, b.findings(t))
			}
		})
	}
}

// STDLIB.md §3: E.members costs one step per member, so three cost two more than one.
func TestEnumMembersSteps(t *testing.T) {
	three := minBudget(t, "local let x: Int = E.members.len()")
	one := minBudget(t, "local enum O { o }\nlocal let x: Int = O.members.len()")
	if three-one != 2 {
		t.Errorf("E.members over 3 members needs %d steps, over 1 %d: want a difference of 2", three, one)
	}
}

// minBudget is the smallest budget under which x of decl evaluates.
func minBudget(t *testing.T, decl string) int64 {
	t.Helper()
	src := []byte(enumHead + decl + "\n")
	for budget := int64(1); budget < maxProbeBudget; budget++ {
		b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{src}), eval.Options{Budget: budget})
		if _, ok := b.values[eval.Root{Pkg: "a", Name: "x"}]; ok {
			return budget
		}
	}
	t.Fatalf("%s: no budget under %d evaluates it", decl, maxProbeBudget)
	return 0
}

// TYPES.md §4.3, DECISIONS 299: the qualified form pkg.E.members reads another package's enum.
func TestEnumMembersQualified(t *testing.T) {
	a := []byte("/// A.\npackage a\n\n/// E.\nenum E { a, retired b, c }\n")
	b := []byte("/// B.\npackage b\n\nimport a\n\nlocal let x: [String] = a.E.members.map(.name)\n")
	got := runBuild(t, parseFiles(t, []string{"a/a.canon", "b/b.canon"}, [][]byte{a, b}), eval.Options{})
	v, ok := got.values[eval.Root{Pkg: "b", Name: "x"}]
	if want := `["a", "b", "c"]`; !ok || v.CanonText() != want {
		t.Errorf("got %v %t, want %s\n%s", v, ok, want, got.findings(t))
	}
}

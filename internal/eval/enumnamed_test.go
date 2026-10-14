package eval_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
)

// bigEnumSize is the size of the enum whose lookup must cost what a small one's does.
const bigEnumSize = 1000

// STDLIB.md §3, DECISIONS 372: E.named(s) is the member named s, case-sensitively, retired included, else none.
func TestEnumNamed(t *testing.T) {
	for _, c := range []struct{ decl, want string }{
		{`local let x: E? = E.named("c")`, "c"},
		{`local let x: E? = E.named("a")`, "a"},
		{`local let x: E? = E.named("z")`, "none"},
		{`local let x: E? = E.named("")`, "none"},
		{`local let x: E? = E.named("A")`, "none"},
		{`local let x: E? = E.named("a ")`, "none"},
		{`local let x: Bool = E.named("b")!.retired`, "true"},
		{`local let x: Int = E.named("b")?.index ?? -1`, "1"},
		{`local let x: E? = E.named(s: "c")`, "c"},
		{`local let x: K? = K.named("z")`, "z"},
		{`local let x: Int = K.named("z")!.code`, "3"},
		{`local let x: E? = E.named("{E.c.name}")`, "c"},
		{"local const X = E.named(\"a\")\nlocal let x: E? = X", "a"},
		{"local fn f(s: String) -> E? {\n  return E.named(s)\n}\nlocal let x: [E?] = [f(\"a\"), f(\"q\")]", "[a, none]"},
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

// TYPES.md §4.3, DECISIONS 372: the qualified form pkg.E.named reads another package's enum.
func TestEnumNamedQualified(t *testing.T) {
	a := []byte("/// A.\npackage a\n\n/// E.\nenum E { a, retired b, c }\n")
	b := []byte("/// B.\npackage b\n\nimport a\n\nlocal let x: [a.E?] = [a.E.named(\"b\"), a.E.named(\"d\")]\n")
	got := runBuild(t, parseFiles(t, []string{"a/a.canon", "b/b.canon"}, [][]byte{a, b}), eval.Options{})
	v, ok := got.values[eval.Root{Pkg: "b", Name: "x"}]
	if want := "[b, none]"; !ok || v.CanonText() != want {
		t.Errorf("got %v %t, want %s\n%s", v, ok, want, got.findings(t))
	}
}

// EVALUATION.md §12.1, DECISIONS 372: E.named costs 1 plus its argument nodes, whatever the enum's size or a miss.
func TestEnumNamedSteps(t *testing.T) {
	members := make([]string, bigEnumSize)
	for i := range members {
		members[i] = fmt.Sprintf("m%d", i)
	}
	big := "local enum Big { " + strings.Join(members, ", ") + " }\n"
	first := minBudget(t, `local let x: Bool = E.named("a") != none`)
	for _, decl := range []string{
		`local let x: Bool = E.named("c") != none`,
		`local let x: Bool = E.named("q") != none`,
		big + `local let x: Bool = Big.named("m0") != none`,
		big + fmt.Sprintf(`local let x: Bool = Big.named("m%d") != none`, bigEnumSize-1),
		big + `local let x: Bool = Big.named("nope") != none`,
	} {
		if got := minBudget(t, decl); got != first {
			t.Errorf("%s needs %d steps, E.named(\"a\") %d: want the same", decl, got, first)
		}
	}
	if plain := minBudget(t, `local let x: Bool = "a" != ""`); first-plain != 1 {
		t.Errorf("E.named(\"a\") != none needs %d steps, \"a\" != \"\" %d: want the application's 1 more", first, plain)
	}
	alone := minBudget(t, `local let x: Bool = "a" + "" != ""`) - minBudget(t, `local let x: Bool = "a" != ""`)
	if args := minBudget(t, `local let x: Bool = E.named("a" + "") != none`); args-first != alone {
		t.Errorf("E.named(\"a\" + \"\") needs %d steps more than E.named(\"a\"), the argument alone %d: want the same", args-first, alone)
	}
}

// lateEnum is declared after the bounds that reflect on it, so they fold before it is completed.
const lateEnum = "local enum K { x, y, z }\n"

// TYPES.md §4.3, DECISIONS 372: reflection in a bound folded before the enum's declaration sees every member.
func TestEnumReflectionBeforeDeclaration(t *testing.T) {
	for _, c := range []struct{ name, before, after, want string }{
		{"named", `local type T = Int(0..=K.named("z")!.index)` + "\n", `local type U = Int(0..=K.named("z")!.index)` + "\nlocal let a: T = 2\nlocal let x: U = 2", "2"},
		{"members", "local type W = Int(0..=K.members.len())\n", "local let x: W = 3", "3"},
		{"typeName", "local type N = Int(0..=K.typeName.len())\n", "local let x: N = 1", "1"},
		{"typefn", "local type W = Int(0..=F(true).members.len())\nlocal type F(b: Bool) = match b {\n  true => K\n  _ => K\n}\n", "local let x: W = 3", "3"},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := "/// A.\npackage a\n\n" + c.before + lateEnum + c.after + "\n"
			b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(src)}), eval.Options{})
			v, ok := b.values[eval.Root{Pkg: "a", Name: "x"}]
			if f := b.findings(t); !ok || v.CanonText() != c.want || !strings.HasPrefix(f, "0 errors") {
				t.Errorf("got %v %t, want %s and no finding\n%s", v, ok, c.want, f)
			}
		})
	}
}

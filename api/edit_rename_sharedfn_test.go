package canon_test

import (
	"fmt"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// sharedFns are, per level of sharedLevels, a function building the level's instance with a pick
// of the old key, and the calls building first's and second's instance with it.
var sharedFns = []struct{ decl, firstCall, secondCall string }{
	{"fn mk(id: String, oo: [S] keyed by k) -> A {\n  return { id: id, oo: oo, pick: \"x\" }\n}\n\n",
		"mk(\"first\", [{ k: \"x\" }, { k: \"y\" }])", "mk(\"second\", [{ k: \"{k}\" }])"},
	{"fn mk(k: String, inn: [T] keyed by k) -> S {\n  return { k: k, inn: inn, pick: \"q\" }\n}\n\n",
		"mk(\"x\", [{ k: \"q\" }, { k: \"y\" }])", "mk(\"x\", [{ k: \"{k}\" }])"},
}

// sharedRefusal is the reason a row's rename is refused with: `computed` when it is, else none.
func sharedRefusal(refused bool) canon.Reason {
	if refused {
		return canon.ReasonComputed
	}
	return ""
}

// API.md R7, E11, E12, E22 (DECISIONS 483): a ref a function body states is shared by every call:
// a ref of second's entry only where a call resolves it there, so a call for first alone or none
// leaves it unlisted and untouched; a call for second makes the rename refused `computed`.
func TestEditRenameSharedFnRef(t *testing.T) {
	cases := []struct {
		name          string
		first, second bool // each instance is built by a call
	}{
		{name: "called for first", first: true},
		{name: "called for second", second: true},
		{name: "called for both", first: true, second: true},
		{name: "called for neither"},
	}
	for i, l := range sharedLevels {
		fn := sharedFns[i]
		for _, c := range cases {
			t.Run(l.name+" "+c.name, func(t *testing.T) {
				marks := map[string]string{"{def}": "", "{pf}": "", "{ps}": ""}
				with := func(key string) map[string]string {
					src := strings.Replace(l.fill(l.src, "{k}", marks), "let aa", fn.decl+"let aa", 1)
					first, second := l.picked(), withPick(l.second, "{k}")
					if c.first {
						first = fn.firstCall
					}
					if c.second {
						second = fn.secondCall
					}
					src = strings.Replace(strings.Replace(src, l.first, first, 1), l.second, second, 1)
					return map[string]string{srcMain: strings.ReplaceAll(src, "{k}", key)}
				}
				before := with(l.old)
				want := []string{fmt.Sprintf("value %s:%d", srcMain, lineOf(before[srcMain], "  { id: \"second\""))}
				if c.second {
					want = []string{fmt.Sprintf("code %s:%d", srcMain, lineOf(before[srcMain], "  return")),
						fmt.Sprintf("value %s:%d", srcMain, lineOf(before[srcMain], "let aa"))}
				}
				sharedRow{l: l, before: before, after: with("y"), want: strings.Join(want, ", "), refused: sharedRefusal(c.second)}.run(t)
			})
		}
	}
}

// sharedLambdas are, per level of sharedLevels, a let mapping aa's instances, those {filter} keeps,
// through a lambda building a copy of each with a pick of the old key.
var sharedLambdas = []string{
	"let bb: [A] = aa{filter}.map(a => A { ...a, pick: \"x\" })\n",
	"let bb: [S] = aa{filter}.map(a => S { ...a.oo.x, pick: \"q\" })\n",
}

// API.md R7, E11, E12, E22 (DECISIONS 483): a lambda's ref is shared by its evaluations, as a
// function body's: evaluated for first alone or none it is unlisted and untouched; for second
// alone its values are computed refs (E12), and for both it also names two entries: refused.
func TestEditRenameSharedLambdaRef(t *testing.T) {
	cases := []struct{ name, filter string }{
		{"mapped for both", ""},
		{"filtered to first", ".filter(a => a.id == \"first\")"},
		{"filtered to second", ".filter(a => a.id == \"second\")"},
		{"filtered to none", ".filter(a => a.id == \"none\")"},
	}
	for i, l := range sharedLevels {
		for _, c := range cases {
			t.Run(l.name+" "+c.name, func(t *testing.T) {
				marks := map[string]string{"{def}": "", "{pf}": ", pick: \"y\"", "{ps}": ", pick: \"{k}\""}
				lambda := strings.ReplaceAll(sharedLambdas[i], "{filter}", c.filter)
				with := func(key string) map[string]string {
					return map[string]string{srcMain: l.fill(l.src, key, marks) + "\n" + lambda}
				}
				before := with(l.old)
				want := []string{fmt.Sprintf("value %s:%d", srcMain, lineOf(before[srcMain], "  { id: \"second\""))}
				second := c.filter == "" || strings.Contains(c.filter, "second")
				if second {
					bb := lineOf(before[srcMain], "let bb")
					want = append(want, fmt.Sprintf("value %s:%d", srcMain, bb), fmt.Sprintf("code %s:%d", srcMain, bb))
				}
				sharedRow{l: l, before: before, after: with("y"), want: strings.Join(want, ", "), refused: sharedRefusal(second)}.run(t)
			})
		}
	}
}

// API.md R7, E12, E19 (DECISIONS 483): an element of a list default, `[b]`, names each quest's own
// b: a code ref of q1's b, which q1 takes, and the rename refused `computed`, as q2 takes it too.
func TestEditRenameSharedDefaultElement(t *testing.T) {
	src := "package p\n\nrecord Step {\n  next: [ref Step] = [b]\n}\n\nrecord Quest {\n  steps: table Step\n}\n\n" +
		"let quests: table Quest = {\n  q1 { steps: { a {}, b { next: [] } } },\n  q2 { steps: { a {}, b { next: [] } } },\n}\n"
	l := sharedLevel{path: "p:quests.q1.steps.b"}
	want := fmt.Sprintf("code %s:%d", srcMain, lineOf(src, "  next:"))
	files := map[string]string{srcMain: src}
	sharedRow{l: l, before: files, after: files, want: want, refused: canon.ReasonComputed}.run(t)
}

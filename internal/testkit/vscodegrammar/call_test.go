package vscodegrammar

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The scopes a string and its interpolation add around what they hold, and their own marks.
var stringFrame = []string{"string.quoted.double.canon", "meta.interpolation.canon", "meta.embedded.line.canon"}

// snapshotLine is one token of a snapshot: its quoted text, then its scopes.
var snapshotLine = regexp.MustCompile(`^\d+:\d+-\d+ ("(?:[^"\\]|\\.)*") (.*)$`)

// callTokens is the snapshot of src as text and scopes, the string frame left out, and the
// tokens that are only the frame (quotes, braces, a bare name in an interpolation) dropped.
func callTokens(t *testing.T, g *Grammar, src string) []string {
	t.Helper()
	snap, err := g.Tokenize(src)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for line := range strings.Lines(snap) {
		m := snapshotLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			t.Fatalf("unread snapshot line %q", line)
		}
		scopes := slices.DeleteFunc(strings.Fields(m[2]), func(s string) bool {
			return slices.Contains(stringFrame, s) || strings.Contains(s, "interpolation")
		})
		if len(scopes) > 0 {
			out = append(out, m[1]+" "+strings.Join(scopes, " "))
		}
	}
	return out
}

// log-2026-10-05 (F12): a call in an interpolation tokenises as the same call outside a string.
func TestCallInInterpolation(t *testing.T) {
	g, err := Load(grammarPath)
	if err != nil {
		t.Fatal(err)
	}
	outside := callTokens(t, g, "sqlStr(l.name)")
	inside := callTokens(t, g, `"{sqlStr(l.name)}"`)
	want := []string{
		`"sqlStr" entity.name.function.call.canon`, `"(" punctuation.section.parens.begin.canon`,
		`"." punctuation.accessor.canon`, `"name" variable.other.property.canon`, `")" punctuation.section.parens.end.canon`,
	}
	if !slices.Equal(outside, want) {
		t.Errorf("outside a string: %q, want %q", outside, want)
	}
	if !slices.Equal(inside, outside) {
		t.Errorf("in an interpolation: %q, outside: %q", inside, outside)
	}
	for _, call := range []string{"s.replace(a, b)", "f(g(x), (y))", "enumOf(c.values).name"} {
		if in, out := callTokens(t, g, `"{`+call+`}"`), callTokens(t, g, call); !slices.Equal(in, out) {
			t.Errorf("%s in an interpolation: %q, outside: %q", call, in, out)
		}
	}
}

// GRAMMAR.md 5.11 pattern: a case binding a name before `=>` is not a call (log-2026-10-05 F12).
func TestMatchArmIsNotACall(t *testing.T) {
	g, err := Load(grammarPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, arm := range []string{"some(s) => s", "gold( _ ) => 0", "item(i)=>i"} {
		got := callTokens(t, g, arm)
		if len(got) == 0 || got[0] != `"=>" keyword.operator.canon` {
			t.Errorf("%s: %q, want the arrow first and no call", arm, got)
		}
	}
	if got := callTokens(t, g, "f(x) + 1"); len(got) == 0 || !strings.Contains(got[0], "entity.name.function.call.canon") {
		t.Errorf("a call before another operator is still a call: %q", got)
	}
}

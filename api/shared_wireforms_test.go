package canon_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// wireSharedDepth is the depth of the chain field b holds: 2^depth places of one Duration list.
const wireSharedDepth = 12

// wireSharedSource is package s: field a holds the list bad at [1] and [10], field b a chain of
// lists w1…w<depth>, each level holding the level below twice, over w0 = bad; both in seconds,
// bad holding 1500ms, which has no whole number of them.
func wireSharedSource(depth int) string {
	var src strings.Builder
	src.WriteString("/// S.\npackage s\n\n/// Bad.\nlet bad: [Duration] = [1s, 1500ms]\n\n/// Fine.\nlet fine: [Duration] = [1s]\n\n/// W0.\nlet w0: [[Duration]] = [bad]\n")
	typ := "[[Duration]]"
	for i := 1; i <= depth; i++ {
		typ = "[" + typ + "]"
		fmt.Fprintf(&src, "\n/// W%d.\nlet w%d: %s = [w%d, w%d]\n", i, i, typ, i-1, i-1)
	}
	fmt.Fprintf(&src, "\n/// R.\nrecord R {\n  /// A.\n  a: [[Duration]] @json(unit: s)\n\n  /// B.\n  b: %s @json(unit: s)\n}\n", typ)
	wide := slices.Repeat([]string{"fine"}, 11)
	wide[1], wide[10] = "bad", "bad"
	fmt.Fprintf(&src, "\n/// R.\nlet r: R = { a: [%s], b: w%d }\n", strings.Join(wide, ", "), depth)
	return src.String()
}

// WIRE.md §5.1, DECISIONS 511: a shared 1500ms is E8102 once per field, at its first place in walk order.
func TestWireFormsSharedFirstPlace(t *testing.T) {
	p, err := canon.Open("/law", project(map[string]string{
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
		"s/s.canon":     wireSharedSource(wireSharedDepth),
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	res, err := p.Check(context.Background(), "s")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range res.Findings {
		got = append(got, f.Code+" "+f.Path)
	}
	code := string(diag.E8102.Def().Code)
	want := []string{code + " r.a[1][1]", code + " r.b" + strings.Repeat("[0]", wireSharedDepth+1) + "[1]"}
	if !slices.Equal(got, want) {
		t.Errorf("findings %q, want %q", got, want)
	}
}

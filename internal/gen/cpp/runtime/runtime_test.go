package cppruntime

import (
	"slices"
	"testing"
)

// CODEGEN.md §7.9: scan keeps identifiers and skips comments, string and character literals, numbers and what follows an escape.
func TestScan(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, text string
		want       []string
	}{
		{"words", "int x_1 = y;", []string{"int", "x_1", "y"}},
		{"line comment", "a // b c\nd", []string{"a", "d"}},
		{"block comment", "a /* b\nc */ d", []string{"a", "d"}},
		{"unterminated block", "a /* b", []string{"a"}},
		{"string", `a "b \" c" d`, []string{"a", "d"}},
		{"char", `a '\'' b 'c' d`, []string{"a", "b", "d"}},
		{"numbers", "0x1F 1e5 100u z", []string{"z"}},
		{"empty", "", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			found := map[string]bool{}
			scan(c.text, found)
			var got []string
			for w := range found { //canon:unordered sorted below
				got = append(got, w)
			}
			slices.Sort(got)
			if !slices.Equal(got, c.want) {
				t.Errorf("%q: %v, want %v", c.text, got, c.want)
			}
		})
	}
}

// CODEGEN.md §7.9: the identifiers of the runtime headers hold the names a define must not take.
func TestIdentifiers(t *testing.T) {
	t.Parallel()
	ids := Identifiers()
	for _, want := range []string{"json", "FlatMap", "CANON_RUNTIME_RT_V1", "canon", "std"} {
		if !slices.Contains(ids, want) {
			t.Errorf("%s missing from the identifiers", want)
		}
	}
	if !slices.IsSorted(ids) {
		t.Error("identifiers are not sorted")
	}
}

package edit

import (
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
)

// API.md M9: the key is the bytes' hash and whether the file is parsed as a project file; a layer
// or a translation is parsed as any other source.
func TestVerdictKey(t *testing.T) {
	raw := []byte("package a\n")
	source := verdictKey(raw, syntax.FileSource)
	for _, c := range []struct {
		name string
		key  VerdictKey
		same bool
	}{
		{"same bytes, source", verdictKey([]byte("package a\n"), syntax.FileSource), true},
		{"same bytes, layer", verdictKey(raw, syntax.FileLayer), true},
		{"same bytes, project", verdictKey(raw, syntax.FileProject), false},
		{"other bytes", verdictKey([]byte("package b\n"), syntax.FileSource), false},
	} {
		if (c.key == source) != c.same {
			t.Errorf("%s: equal to the source key %v, want %v", c.name, c.key == source, c.same)
		}
	}
}

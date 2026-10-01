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

// API.md M9, DECISIONS 258: a file is judged in its role: project.canon at the root is the project
// file, any other path is a source, in a subdirectory too.
func TestRoleOf(t *testing.T) {
	for _, c := range []struct {
		display string
		want    syntax.FileKind
	}{
		{"project.canon", syntax.FileProject},
		{"sub/project.canon", syntax.FileSource},
		{"d/d.canon", syntax.FileSource},
		{"d/dev.layer.canon", syntax.FileSource},
		{"d/d.fr.canon", syntax.FileSource},
		{"@a/project.canon", syntax.FileSource},
		{"project.canon/d.canon", syntax.FileSource},
	} {
		if got := roleOf(c.display); got != c.want {
			t.Errorf("%s: role %v, want %v", c.display, got, c.want)
		}
	}
}

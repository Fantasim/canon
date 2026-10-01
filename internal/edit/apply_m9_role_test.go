package edit_test

import (
	"crypto/sha256"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

const (
	roleProject = "project acme {\n  canon: \"0.1\"\n  languages: [en, fr]\n}\n"
	roleBase    = "/// P.\npackage d\n\n/// A.\nlet a: Int = 1\n"
	roleLayer   = "package d\nlayer dev\n\namend a {\n  value: 2\n}\n"
	roleLayerLs = "package d\nlayer dev\namend a {\n  value:2\n}\n"
	roleTrans   = "package d\ntranslation fr\n\nGlobal.price \"Prix\"\n"
	roleTransLs = "package d\ntranslation fr\nGlobal.price    \"Prix\"\n"
	roleOpening = "project demo {\n  canon: \"0.1\"\n}\n"
)

// roleFiles is a package holding a source, a layer file and a translation file.
func roleFiles(layer, translation string) mapFS {
	return mapFS{
		"law/project.canon":     file(roleProject),
		"law/d/d.canon":         file(roleBase),
		"law/d/dev.layer.canon": file(layer),
		"law/d/d.fr.canon":      file(translation),
		"law/d/p.canon":         file(roleOpening),
	}
}

// A layer file and a translation file are judged as sources, the role they have in the project:
// canonical text is kept as it is, loose text is printed canonically (API.md M9, DECISIONS 258).
func TestM9LayerAndTranslationInRole(t *testing.T) {
	for _, c := range []struct {
		name, display, raw, want string
	}{
		{"layer canonical", "d/dev.layer.canon", roleLayer, roleLayer},
		{"layer loose", "d/dev.layer.canon", roleLayerLs, roleLayer},
		{"translation canonical", "d/d.fr.canon", roleTrans, roleTrans},
		{"translation loose", "d/d.fr.canon", roleTransLs, roleTrans},
	} {
		s := open(t, roleFiles(roleLayer, roleTrans), []string{"dev"}, "dev", "d")
		memo := &spyVerdicts{}
		for ask := range 2 { // a miss, then a hit when the bytes were kept
			got, err := edit.CanonicalSourceWith(s.snap, memo, c.display, []byte(c.raw))
			if err != nil || string(got) != c.want {
				t.Errorf("%s, ask %d: %q, %v; want %q", c.name, ask, got, err, c.want)
			}
		}
		if _, kept := memo.kept[edit.VerdictKey{Sum: sumOf(c.raw)}]; kept != (c.raw == c.want) {
			t.Errorf("%s: kept as a source %v, want %v", c.name, kept, c.raw == c.want)
		}
	}
}

// A source file whose text opens with `project` is no file of a snapshot, so M9 never calls it a
// fixed point; and a verdict kept for a project.canon's text is not one for a source of the same
// bytes (API.md M9, DECISIONS 258).
func TestM9RoleNeverCrossed(t *testing.T) {
	s := open(t, roleFiles(roleLayer, roleTrans), []string{"dev"}, "dev", "d")
	if got, err := edit.CanonicalSourceWith(s.snap, nil, "d/p.canon", []byte(roleOpening)); err == nil {
		t.Errorf("a source opening with project is a fixed point: %q", got)
	}
	memo := &spyVerdicts{kept: map[edit.VerdictKey]bool{{Sum: sumOf(roleLayerLs), Project: true}: true}}
	got, err := edit.CanonicalSourceWith(s.snap, memo, "d/dev.layer.canon", []byte(roleLayerLs))
	if err != nil || string(got) != roleLayer {
		t.Errorf("a project verdict was taken for a layer file: %q, %v", got, err)
	}
	memo = &spyVerdicts{kept: map[edit.VerdictKey]bool{{Sum: sumOf(roleLayerLs)}: true}}
	got, err = edit.CanonicalSourceWith(s.snap, memo, "d/dev.layer.canon", []byte(roleLayerLs))
	if err != nil || string(got) != roleLayerLs {
		t.Errorf("a source verdict was not taken for a layer file: %q, %v", got, err)
	}
}

// sumOf is the hash a verdict key holds for text.
func sumOf(text string) [sha256.Size]byte { return sha256.Sum256([]byte(text)) }

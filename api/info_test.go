package canon_test

import (
	"context"
	"errors"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// API.md §5.5, DECISIONS 313: Info reads the project's name and doc as written, none giving "".
func TestInfo(t *testing.T) {
	cases := []struct {
		name, project string
		want          canon.ProjectInfo
	}{
		{"doc", "/// The demo law.\n/// Two lines.\nproject demo {\n  canon: \"0.1\"\n}\n", canon.ProjectInfo{Name: "demo", Doc: "The demo law.\nTwo lines."}},
		{"none", "project plain {\n  canon: \"0.1\"\n}\n", canon.ProjectInfo{Name: "plain"}},
	}
	for _, c := range cases {
		p, _ := openLaw(t, map[string]string{"project.canon": c.project})
		got, err := p.Info(context.Background())
		if err != nil || got != c.want {
			t.Errorf("API.md §5.5 %s: Info = %+v, %v, want %+v", c.name, got, err, c.want)
		}
	}
}

// API.md O6: Info after Close fails.
func TestInfoClosed(t *testing.T) {
	p, _ := openLaw(t, map[string]string{"project.canon": "project demo {\n  canon: \"0.1\"\n}\n"})
	_ = p.Close()
	if _, err := p.Info(context.Background()); err == nil {
		t.Error("API.md O6: Info after Close succeeded")
	}
}

// API.md §5.5, X1, DECISIONS 313: Info fails like Packages, with the same error and findings.
func TestInfoFailsLikePackages(t *testing.T) {
	const good = "project demo {\n  canon: \"0.1\"\n}\n"
	cases := []struct{ name, overlay string }{
		{"version", "project demo {\n  canon: \"9.9\"\n}\n"},
		{"nocanon", "project demo {\n}\n"},
		{"syntax", "project {{{\n"},
		{"removed", ""},
	}
	for _, c := range cases {
		p, opts := openLaw(t, map[string]string{"project.canon": good})
		if c.overlay != "" {
			if err := p.SetOverlay("project.canon", []byte(c.overlay)); err != nil {
				t.Fatal(err)
			}
		} else if err := opts.FS.(interface{ Remove(string) error }).Remove("/law/project.canon"); err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		_, want := p.Packages(ctx)
		_, got := p.Info(ctx)
		if want == nil || got == nil || got.Error() != want.Error() {
			t.Errorf("API.md §5.5 %s: Info error %v, Packages error %v", c.name, got, want)
			continue
		}
		var gp, wp *canon.ProjectError
		if errors.As(got, &gp) != errors.As(want, &wp) || (gp != nil && len(gp.Findings) != len(wp.Findings)) ||
			errors.Is(got, canon.ErrNoProject) != errors.Is(want, canon.ErrNoProject) ||
			errors.Is(got, canon.ErrProject) != errors.Is(want, canon.ErrProject) {
			t.Errorf("API.md §5.5 %s: Info %#v, Packages %#v", c.name, got, want)
		}
	}
}

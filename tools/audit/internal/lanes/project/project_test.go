package project

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

func runFixture(t *testing.T) []finding.Finding {
	t.Helper()
	r, err := repo.Open("testdata/fakerepo")
	if err != nil {
		t.Fatalf("repo.Open: %v", err)
	}
	enabled := map[string]bool{ruleRootClutter: true, ruleDeadLink: true}
	fs, skips := lane.Run(&lane.Context{Repo: r, Enabled: enabled}, []lane.Lane{New()})
	if len(skips) != 0 {
		t.Fatalf("unexpected skips: %v", skips)
	}
	return fs
}

func TestProjectLane(t *testing.T) {
	var got []string
	for _, f := range runFixture(t) {
		got = append(got, f.Rule+" "+f.File+" "+f.Detail)
	}
	want := []string{
		"dead-link README.md gone.md",
		"dead-link docs/guide.md missing/x.md",
		"dead-link examples/demo/README.md nope.md",
		"root-clutter stray-dir ",
		"root-clutter stray.png ",
	}
	if !slices.Equal(got, want) {
		t.Errorf("findings:\n got  %q\n want %q", got, want)
	}
}

func TestCleanLinkTarget(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"state.md", "state.md"},
		{"#anchor-only", ""},
		{"https://example.com/x", ""},
		{"mailto:a@b.com", ""},
		{"../decisions/0001.md", "../decisions/0001.md"},
		{"state.md#section", "state.md"},
		{"file.go:42", "file.go"},
		{"  spaced.md  ", "spaced.md"},
		{`README.md "Readme"`, "README.md"},
	}
	for _, c := range cases {
		if got := cleanLinkTarget(c.raw); got != c.want {
			t.Errorf("cleanLinkTarget(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

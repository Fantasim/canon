package load

import (
	"path"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/project"
)

// CODEGEN.md §2.4, DECISIONS 451: MatchesPlanned answers as the walk does for the files a tree holds.
func TestMatchesPlannedAgreesWithWalk(t *testing.T) {
	tree := []string{
		"data/a.json", "data/b.csv", "data/.hidden.json", "data/Upper.JSON", "data/ab.json",
		"data/sub/c.json", "data/sub/deep/d.json", "data/sub/.git/keep.json", "data/.dot/e.json",
		"data/{x}.json", "data/x1.json", "data/xa.json", "data/].json", "data/é.json", "data/日本/x.json",
		"data/.dot/.hid.json", "data/sub/.git/sub/z.json",
	}
	files := map[string]string{}
	for _, name := range tree {
		files[path.Join("/p", name)] = "{}"
	}
	l := testLoader(t, files)
	patterns := []string{
		"*.json", "**/*.json", "**", "sub/**", "sub/**/d.json", "*/*.json", "?.json", "[ab].json",
		"*.{json,csv}", ".*.json", "**/.*.json", "sub/*/keep.json", "*.JSON", "x?.json", "**/**/*.json",
		"[!x]*.json", "[]a].json", "[!]a].json", ".dot/*", ".dot/**", "**/.git/*", "**/.git/**", "**/.git/**/z.json",
		"{a,.hidden}.json", "{.dot,sub}/*.json", "{.dot,sub}/**", "*/x.json", "日本/*", "[é]*.json",
	}
	base := project.Path{Display: "data", Abs: "/p/data"}
	for _, rest := range patterns {
		matches, err := l.globMatches(base, rest, Request{})
		if err != nil {
			t.Fatal(err)
		}
		var walked, planned []string
		for _, m := range matches {
			walked = append(walked, m.Display)
		}
		for _, name := range tree {
			if MatchesPlanned(path.Join("data", rest), name) {
				planned = append(planned, name)
			}
		}
		slices.Sort(walked)
		slices.Sort(planned)
		if !slices.Equal(walked, planned) {
			t.Errorf("pattern %q: walk %v, MatchesPlanned %v", rest, walked, planned)
		}
	}
}

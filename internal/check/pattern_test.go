package check

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// acceptedPatterns are the patterns of testdata/accept/patterns.txtar, one per §11.3 construct.
func acceptedPatterns(f *testing.F) []string {
	data, err := os.ReadFile("testdata/accept/patterns.txtar")
	if err != nil {
		f.Fatal(err)
	}
	var out []string
	for _, m := range regexp.MustCompile(`String\(/(.*)/\)\?`).FindAllStringSubmatch(string(data), -1) {
		out = append(out, strings.ReplaceAll(m[1], `\/`, "/"))
	}
	return out
}

// IMPLEMENTATION-PLAN §7.7: the pattern whitelist never panics and names a construct it read.
func FuzzUnportable(f *testing.F) {
	accepted := acceptedPatterns(f)
	if len(accepted) == 0 {
		f.Fatal("no accepted pattern read")
	}
	for _, p := range accepted {
		if bad := unportable(p); bad != "" {
			f.Errorf("unportable(%q) = %q, want none (EVALUATION.md §11.3)", p, bad)
		}
		f.Add(p)
	}
	for _, seed := range []string{`^[a-z\d-]+$`, `(?i)a`, `[x[:digit:]]`, `\x{263A}`, `[\w-a]`, `^*a`, `a{,3}`, `a{01}`, `a{1,02}`, `[^]a]`, `(?:^)*`, `\`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, re string) {
		if _, err := regexp.Compile(re); err != nil {
			return
		}
		if bad := unportable(re); bad != "" && !strings.Contains(re, bad) {
			t.Errorf("unportable(%q) = %q, not spelled in the pattern", re, bad)
		}
	})
}

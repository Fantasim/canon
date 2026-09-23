package repo

import "testing"

func TestSkipped(t *testing.T) {
	cases := map[string]bool{
		"vendor/x/x.go": true,
		"node_modules/flatted/golang/pkg/flatted/x.go":   true,
		"web/node_modules/flatted/golang/pkg/flatted.go": true,
		"internal/testdata/fixture.go":                   true,
		".claude/worktrees/w/x.go":                       true,
		"examples/pipeline/expected/go/x.gen.go":         true,
		"examples/features/text/expected/out.md":         true,
		"examples/_fixtures/client/x.go":                 true,
		"examples/pipeline/potion.canon":                 false,
		"internal/expected/x.go":                         false,
		"cmd/dist/x.go":                                  true,
		"build/x.go":                                     true,
		"internal/repo/repo.go":                          false,
		"web/src/lib/x.ts":                               false,
		"internal/notvendorat/x.go":                      false,
	}
	for rel, want := range cases {
		if got := Skipped(rel); got != want {
			t.Errorf("Skipped(%q) = %v, want %v", rel, got, want)
		}
	}
}

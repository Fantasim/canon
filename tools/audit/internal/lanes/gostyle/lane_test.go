package gostyle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
	"github.com/fantasim/canonlang/tools/audit/internal/threshold"
)

// runFixture parses every .go file directly under dir and runs the lane through the real
// runner (lane.Run), so Symbol-filling and sorting behave as they do in production.
func runFixture(t *testing.T, dir string) []finding.Finding {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var rel []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), goSuffix) {
			rel = append(rel, e.Name())
		}
	}
	tree, perrs := gosrc.Parse(dir, rel)
	for _, e := range perrs {
		t.Fatal(e)
	}
	ctx := &lane.Context{Repo: &repo.Repo{}, Go: tree, Enabled: enabledAll(), Limits: shippedLimits(t)}
	fs, skips := lane.Run(ctx, []lane.Lane{New()})
	for _, s := range skips {
		t.Logf("skip: %+v", s)
	}
	return fs
}

func enabledAll() map[string]bool {
	m := make(map[string]bool, len(ruleIDs))
	for _, id := range ruleIDs {
		m[id] = true
	}
	return m
}

func countByRule(fs []finding.Finding) map[string]int {
	m := map[string]int{}
	for _, f := range fs {
		m[f.Rule]++
	}
	return m
}

// only filters fs down to one rule, for a precise field-by-field check.
func only(fs []finding.Finding, rule string) []finding.Finding {
	var out []finding.Finding
	for _, f := range fs {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

const goSuffix = ".go"

// shippedLimits is thresholds.tsv as the tool ships it.
func shippedLimits(t *testing.T) threshold.Set {
	t.Helper()
	s, err := threshold.Load(filepath.Join("..", "..", "..", threshold.FileName))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

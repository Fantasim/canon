package lanes

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/rules"
	"github.com/fantasim/canonlang/tools/audit/internal/threshold"
)

func TestEveryRuleHasALane(t *testing.T) {
	produced := map[string]bool{}
	for _, l := range All() {
		for _, id := range l.Rules() {
			if _, ok := rules.Lookup(id); !ok {
				t.Errorf("lane %s declares unknown rule %s", l.Name(), id)
			}
			produced[id] = true
		}
	}
	for _, rl := range rules.All {
		if rl.Tool != rules.Mechanism && !produced[rl.ID] {
			t.Errorf("rule %s is produced by no lane", rl.ID)
		}
	}
}

func TestRulesDocInSync(t *testing.T) {
	data, err := os.ReadFile("../../rules.md")
	if err != nil {
		t.Fatal(err)
	}
	var doc []string
	for _, m := range regexp.MustCompile("(?m)^### `([a-z0-9-]+)`").FindAllStringSubmatch(string(data), -1) {
		doc = append(doc, m[1])
	}
	var book []string
	for _, rl := range rules.All {
		book = append(book, rl.ID)
	}
	if !slices.Equal(doc, book) {
		t.Fatalf("rules.md and the rulebook disagree (order matters)\n doc:  %v\n book: %v", doc, book)
	}
}

// Each rule's line in rules.md is its mode, tool and summary with the shipped thresholds.
func TestRulesDocSummaries(t *testing.T) {
	data, err := os.ReadFile("../../rules.md")
	if err != nil {
		t.Fatal(err)
	}
	limits, err := threshold.Load(filepath.Join("..", "..", threshold.FileName))
	if err != nil {
		t.Fatal(err)
	}
	for _, rl := range rules.All {
		want := "### `" + rl.ID + "`\n" + string(rl.Mode) + " · " + rl.Tool + ": " + rl.Describe(limits) + ".\n"
		if !strings.Contains(string(data), want) {
			t.Errorf("rules.md lacks, for %s:\n%s", rl.ID, want)
		}
	}
}

func TestEverySummaryExpands(t *testing.T) {
	for _, rl := range rules.All {
		if _, err := (threshold.Set{}).Expand(rl.Summary); err != nil {
			t.Errorf("%s: %v", rl.ID, err)
		}
	}
}

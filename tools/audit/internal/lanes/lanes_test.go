package lanes

import (
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/rules"
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

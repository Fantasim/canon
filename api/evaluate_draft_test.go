package canon_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// evalDraft evaluates path with draft applied, failing the test on an error.
func evalDraft(t *testing.T, p *canon.Project, path string, draft ...canon.Op) *canon.EvalResult {
	t.Helper()
	res, err := p.Evaluate(context.Background(), canon.EvalRequest{Path: path, Draft: draft})
	if err != nil {
		t.Fatalf("Evaluate(%s): %v", path, err)
	}
	return res
}

// API.md V13, API.md E17: a draft is applied in memory: the texts, findings and summary are the
// draft's, an entry it adds can be evaluated, and nothing is written, the revision unchanged.
func TestEvaluateDraft(t *testing.T) {
	p, opts := openLaw(t, evalLaw)
	rev := p.Revision()
	before, _ := opts.FS.ReadFile("/law/a/a.canon")
	res := evalDraft(t, p, "a:items.sword", canon.Set("a:items.sword.name", canon.Str("Axe")), canon.Set("a:items.sword.count", canon.Int(2)))
	if res.Title.Value != "Item Axe" || len(res.Findings) != 0 || res.Revision != rev || res.Dropped == nil {
		t.Errorf("API.md V13: %+v", res)
	}
	if res.Summary.Errors != 1 || res.Summary.Packages != 2 {
		t.Errorf("API.md E17: the draft's summary %+v (b's error only)", res.Summary)
	}
	plain := evaluate(t, p, "a:items.sword", "")
	if plain.Title.Value != "Item Sword" || len(plain.Findings) != 1 {
		t.Errorf("API.md V13: the draft leaked: %+v", plain)
	}
	axe := `{"name": "Axe", "shown": true, "count": 2, "goal": "kill", "target": "x", "parts": []}`
	if res := evalDraft(t, p, "a:items.axe", canon.AddEntry("a:items", canon.Key("axe"), canon.FromJSON([]byte(axe)))); res.Title.Value != "Item Axe" {
		t.Errorf("API.md V13: a new entry's title %q", res.Title.Value)
	}
	after, _ := opts.FS.ReadFile("/law/a/a.canon")
	if string(after) != string(before) || p.Revision() != rev {
		t.Error("API.md V13: a draft wrote")
	}
}

// API.md V13, API.md S8, API.md S5: concurrent drafts each see their own; a draft against a
// stale base is refused.
func TestEvaluateDraftsApart(t *testing.T) {
	p, opts := openLaw(t, evalLaw)
	base := p.Revision()
	names := []string{"Axe", "Bow", "Axe", "Bow"}
	titles := make([]string, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Go(func() {
			res, err := p.Evaluate(context.Background(), canon.EvalRequest{Path: "a:items.sword", Draft: []canon.Op{canon.Set("a:items.sword.name", canon.Str(name))}})
			if err == nil {
				titles[i] = res.Title.Value
			}
		})
	}
	wg.Wait()
	for i, name := range names {
		if titles[i] != "Item "+name {
			t.Errorf("API.md S8: draft %s gave %q", name, titles[i])
		}
	}
	writeLaw(t, opts.FS, "c/c.canon", strings.Replace(evalLaw["c/c.canon"], "= 1", "= 2", 1))
	req := canon.EvalRequest{Base: base, Path: "a:items.sword", Draft: []canon.Op{canon.Set("a:items.sword.name", canon.Str("Axe"))}}
	if _, err := p.Evaluate(context.Background(), req); !errors.Is(err, canon.ErrStale) {
		t.Errorf("API.md S5: a draft on a stale base: %v", err)
	}
}

// collidingDrafts are pairs of drafts whose ops write the same JSON yet differ: an Int or a
// Float, a String or a Key, a record or a map; neither reads back from it as it was.
var collidingDrafts = [][2]canon.Op{
	{canon.Set("a:items.sword.count", canon.Int(2)), canon.Set("a:items.sword.count", canon.Float(2))},
	{canon.Set("a:items.sword.name", canon.Str("Axe")), canon.Set("a:items.sword.name", canon.Key("Axe"))},
	{canon.Set("a:items.sword.parts[0]", canon.Obj{}), canon.Set("a:items.sword.parts[0]", canon.Map())},
}

// outcome is what an Evaluate of a:items.sword gives with op as its draft.
func outcome(p *canon.Project, op canon.Op) string {
	res, err := p.Evaluate(context.Background(), canon.EvalRequest{Path: "a:items.sword", Draft: []canon.Op{op}})
	if err != nil {
		return err.Error()
	}
	return res.Title.Value + " " + strings.Repeat("!", len(res.Findings)) + " " + string(rune('0'+res.Summary.Errors))
}

// API.md V13, API.md S8 (log-2026-09-29 M4 U5b-r): drafts share a computation only when each op
// reads back exactly from its JSON form, so drafts that collide there each get their own outcome.
func TestEvaluateDraftsCollide(t *testing.T) {
	p, _ := openLaw(t, evalLaw)
	want := make([][2]string, len(collidingDrafts))
	for i, pair := range collidingDrafts {
		want[i] = [2]string{outcome(p, pair[0]), outcome(p, pair[1])}
		if want[i][0] == want[i][1] {
			t.Errorf("%+v and %+v give the same outcome %q: no collision to test", pair[0], pair[1], want[i][0])
		}
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 3 {
		for i, pair := range collidingDrafts {
			for j, op := range pair {
				wg.Go(func() {
					got := outcome(p, op)
					mu.Lock()
					defer mu.Unlock()
					if got != want[i][j] {
						t.Errorf("API.md S8: draft %+v gave %q, alone %q", op, got, want[i][j])
					}
				})
			}
		}
	}
	wg.Wait()
}

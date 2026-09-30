package canon_test

import (
	"context"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

const hardCheckLaw = `package teamboard

record Column {
  label: String(1..)
  count: Int

  check 10 / count >= 0 else "a ratio is negative"
}

let columns: table Column = {
  flaky { label: "Flaky", count: 0 }
}

record Slot {
  count: Int

  check 10 / count >= 0 else "a ratio is negative"
}

let listed: [Slot] = [{ count: 0 }]
`

// API.md F1: a hard error a check raises on an instance has the instance's canonical path, on a
// cold check and on a second one that replays the first's trace.
func TestFindingPathOfInstanceCheckHardError(t *testing.T) {
	p, err := canon.Open("/law", project(map[string]string{
		"project.canon":            "project a {\n  canon: \"0.1\"\n}\n",
		"teamboard/taxonomy.canon": hardCheckLaw,
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	code := diag.E4102.Def().Code
	for pass := 0; pass < 2; pass++ {
		res, err := p.Check(context.Background(), "teamboard")
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for _, f := range res.Findings {
			if diag.Code(f.Code) == code {
				got[f.Path] = true
			}
		}
		if !got["columns.flaky"] || !got["listed[0]"] || len(got) != 2 {
			t.Fatalf("pass %d: %s paths %v, want columns.flaky and listed[0]", pass, code, got)
		}
	}
}

package ir_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// clubs holds a record Club, whose captain refs the @reload table members, in two values: clubs2 @reload or not.
func clubs(reload string) string {
	return `package club

/// A member.
record Member {
  /// Its age.
  age: Int
}

/// A club.
record Club {
  /// Its name.
  title: String
  /// Its captain.
  captain: ref members
}

/// The members.
@reload
let members: table Member = {
  ann { age: 1 }
}

/// A club.
@reload
let club1: Club = { title: "a", captain: ann }

/// Another club.
` + reload + `let club2: Club = { title: "b", captain: ann }

emit go { out: "@features/club", package: "club", mode: data }
emit cpp { out: "@features/club", namespace: "club", mode: data }
emit json { out: "out/" }
`
}

// TestSeveralHoldersResolve is log-2026-09-24 "ir name plans + support plan" (CODEGEN.md §5.8, §5.11): a ref in a record several values hold resolves only when every holder resolves it into the same target, here all @reload beside the @reload members; a holder that is not @reload leaves it a key, in both plans.
func TestSeveralHoldersResolve(t *testing.T) {
	for _, c := range []struct {
		reload string
		want   bool
	}{{"@reload\n", true}, {"", false}} {
		w := newWorld(t)
		w.add(t, "club/club.canon", []byte(clubs(c.reload)))
		w.add(t, "club.members.json", []byte(`{ "ann": { "age": 1 } }`))
		w.add(t, "club.club1.json", []byte(`{ "title": "a", "captain": "ann" }`))
		w.add(t, "club.club2.json", []byte(`{ "title": "b", "captain": "ann" }`))
		w.calls = w.fixtureCalls
		p := w.build(t)[0]
		if out := w.findings(t); out[:len(noFindings)] != noFindings {
			t.Fatalf("findings:\n%s", out)
		}
		club := p.Types[1].(*ir.Record)
		captain := club.Fields[1]
		gotGo := ir.PlanGoNames(p, p.Emits[0]).Slot(captain).Resolved
		gotCpp := ir.PlanCppNames(p, p.Emits[1]).Resolves(captain.Type, club)
		if gotGo != c.want || gotCpp != c.want || captain.Type.Kind != types.Ref {
			t.Errorf("club2 %q: go resolved %v, cpp %v, want %v", c.reload, gotGo, gotCpp, c.want)
		}
	}
}

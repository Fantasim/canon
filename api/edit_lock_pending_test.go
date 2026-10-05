package canon_test

import (
	"context"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// pendingSrc is a stable table whose owl, complete, the lock does not hold yet; lvl has a default.
const pendingSrc = "package p\n\nrecord M {\n  hp: Int\n  lvl: Int = 1\n}\n\nlet ms: stable table M = {\n  wolf { hp: 1 }\n  owl { hp: 2, lvl: 3 }\n}\n"

// API.md E20, LOCK.md 6.2, E23: a Set or Reset of any field of a pending stable entry records its
// id; the Undo gives the value back and keeps the lock fact.
func TestEditLockPendingEntry(t *testing.T) {
	ctx := context.Background()
	locked := "# canon.lock v1\ntable  p.ms  owl\ntable  p.ms  wolf\n"
	for _, op := range []canon.Op{canon.Set("p:ms.owl.hp", canon.Int(3)), canon.Reset("p:ms.owl.lvl")} {
		p, m := openEdit(t, map[string]string{"p/p.canon": pendingSrc, "p/canon.lock": poisonedLock})
		res, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{op}})
		if err != nil {
			t.Errorf("%+v: %v", op, err)
			continue
		}
		if got := lockOf(t, m, "p/canon.lock"); got != locked {
			t.Errorf("API.md E20, %+v: lock %q, want %q", op, got, locked)
		}
		if len(res.Undo) != 1 || res.Undo[0].Kind == canon.OpRetire {
			t.Errorf("API.md E23, %+v: Undo %+v, want the value back, owl not retired", op, res.Undo)
		}
		if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
			t.Errorf("API.md E22, %+v: the Undo %+v: %v", op, res.Undo, err)
			continue
		}
		if got := read(t, m, "p/p.canon"); got != pendingSrc {
			t.Errorf("API.md E22, %+v: source after the Undo %+v:\n%s", op, res.Undo, got)
		}
		if got := lockOf(t, m, "p/canon.lock"); got != locked {
			t.Errorf("API.md E23, %+v: lock after the Undo %q, want %q", op, got, locked)
		}
	}
}

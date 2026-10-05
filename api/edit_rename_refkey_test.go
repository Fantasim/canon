package canon_test

import (
	"context"
	"errors"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// API.md E3, E11, V1, DECISIONS 316: renaming a keyed-list element whose key field is a `ref C`
// to a key C has no entry for is ErrBadValue, AllowErrors or not, and nothing is written.
func TestEditRenameRefKeyMustExist(t *testing.T) {
	src := refKeysHead + "let spawns: [S] keyed by m = [{ m: wolf, n: 2 }]\n"
	for _, allow := range []bool{false, true} {
		p, m := openEdit(t, srcLaw(src))
		_, err := p.Edit(context.Background(), canon.Edit{AllowErrors: allow, Ops: []canon.Op{canon.Rename("p:spawns[wolf]", canon.Key("cat"))}})
		if !errors.Is(err, canon.ErrBadValue) || read(t, m, srcMain) != src {
			t.Errorf("API.md E3, V1, AllowErrors %v: %v, want ErrBadValue and nothing written", allow, err)
		}
		if _, err := p.Edit(context.Background(), canon.Edit{AllowErrors: allow, DryRun: true, Ops: []canon.Op{canon.Rename("p:spawns[wolf]", canon.Key("bear"))}}); err != nil {
			t.Errorf("API.md E11, AllowErrors %v: rename to an existing entry: %v", allow, err)
		}
	}
}

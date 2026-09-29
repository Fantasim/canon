package build

import (
	"bytes"
	"context"
	"path"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

const (
	identityOutput = "g.gen.go"
	identitySame   = "var sameTable bool = true" // same() precomputed: k["sword"] == sw
)

// TYPES.md §15, EVALUATION.md §5: stage A forces a folded constant again, so what it reads is verified (E3505 kept).
func TestConstVerifiedPerEvaluator(t *testing.T) {
	for _, sel := range [][]string{nil, {"a"}} {
		z := archiveAnalyzer(t, "testdata/incremental/constverify.txtar")
		warm, cold := z.pairSel(t, sel)
		for _, a := range []*Analysis{warm, cold} {
			if counts(a, diag.E3505.Def().Code) == 0 {
				t.Errorf("selection %v: the orphan ref of b.L is not reported: %v", sel, a.Result().List)
			}
		}
	}
}

// TYPES.md §15, EVALUATION.md §2.3: a stage E fold's constant is its own record, so stage A's precomputation holds.
func TestConstIdentityPerEvaluator(t *testing.T) {
	z := archiveAnalyzer(t, "testdata/incremental/constidentity.txtar")
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), BuildOptions{Check: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range res.Outputs {
		if path.Base(o.Path) != identityOutput {
			continue
		}
		if !bytes.Contains(o.Content, []byte(identitySame)) {
			t.Errorf("%s lacks %q:\n%s", o.Path, identitySame, o.Content)
		}
		return
	}
	t.Errorf("no %s among %d outputs", identityOutput, len(res.Outputs))
}

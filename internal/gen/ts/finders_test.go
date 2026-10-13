package tsgen_test

import (
	"errors"
	"testing"

	"golang.org/x/tools/txtar"

	tsgen "github.com/fantasim/canonlang/internal/gen/ts"
	"github.com/fantasim/canonlang/internal/ir"
)

const twoLookups = "/// R.\nrecord R {\n  /// A.\n  a_b: Int @stable\n  /// B.\n  aB: Int @stable\n}\n\n/// Rs.\nlet rs: stable table R = {\n  x { a_b: 1, aB: 2 }\n}\n"

// CODEGEN.md §5.9: FindBy + UpperCamel(f) names one lookup per @stable field; two fields making the same name are refused with a sentinel, never written as a duplicate member.
func TestFindByNameCollisionIsRefused(t *testing.T) {
	w := buildWorld(t, []txtar.File{{Name: "a/a.canon", Data: []byte(collisionPrefix + twoLookups + collisionSuffix)}})
	if _, errs := w.findings(t); errs > 0 {
		t.Skipf("the check refuses it first: %v", errs)
	}
	for _, p := range w.pkgs {
		for _, e := range p.Emits {
			if e.Target != ir.TargetTS {
				continue
			}
			if _, err := tsgen.Generate(ir.CopyOf(w.proj, p, e), e); !errors.Is(err, tsgen.ErrMalformed) {
				t.Errorf("%s: %v, want ErrMalformed", p.Name, err)
			}
		}
	}
}

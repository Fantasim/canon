package ir_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// TestNamePlansNotYetGenerated is CODEGEN.md §3.3–§3.5, §5.6, §5.12, §7.7 (log-2026-09-24 "W2 gen/cpp inputs review", Names): the Go and C++ plans name dependent types, LoadInputs, the input namespace and the input slots before gen/go and gen/cpp write them, so no comparison with a generator holds them yet; each go and data-mode cpp emit's scopes and problems, then stage E's findings, are the golden.
func TestNamePlansNotYetGenerated(t *testing.T) {
	golden.Run(t, "testdata/planpending/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		w := newWorld(t)
		for _, f := range c.Archive.Files {
			if f.Name != planFile {
				w.add(t, f.Name, f.Data)
			}
		}
		w.calls = w.fixtureCalls
		var b strings.Builder
		for _, p := range w.build(t) {
			for _, e := range p.Emits {
				switch {
				case e.Target == ir.TargetGo:
					pl := ir.PlanGoNames(p, e)
					fmt.Fprintf(&b, "== go %s\n%s", p.Name, dumpScopes(ir.GoScopeNames(pl), pl.Problems()))
				case e.Target == ir.TargetCpp && e.Mode == ir.ModeData:
					pl := ir.PlanCppNames(p, e)
					fmt.Fprintf(&b, "== cpp %s\n%s", p.Name, dumpScopes(ir.CppScopeNames(pl), pl.Problems()))
				}
			}
		}
		b.WriteString(w.findings(t))
		return []byte(b.String())
	}, golden.Expected(planFile))
}

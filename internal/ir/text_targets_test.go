package ir_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// TestTextDecodersPerTarget is DECISIONS 363: every types-mode code emit decodes the same readable @text results, under its own target's checks and names (C++ Decode<Fn>File, TypeScript decode<Fn>File and parse<Fn>File), and a data-mode go or cpp emit lists in NoHook the records and cases (V.Case) whose make hook its own plan does not write. The golden is each emit's decoders, or its NoHook, then the findings.
func TestTextDecodersPerTarget(t *testing.T) {
	golden.Run(t, "testdata/textdecoders_targets/*.txtar", func(t *testing.T, c golden.Case) []byte {
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
				b.WriteString("== " + p.Name + " " + e.Out + ": " + targetDecoders(ir.CopyOf(w.proj, p, e), e) + "\n")
			}
		}
		b.WriteString(w.findings(t))
		return []byte(b.String())
	}, golden.Expected(planFile))
}

// targetDecoders are the @text decoders emit e of p writes, by its target's names, or the NoHook of a data emit.
func targetDecoders(p *ir.Package, e *ir.Emit) string {
	var names []string
	switch {
	case e.Mode == ir.ModeData:
		return "nohook [" + strings.Join(e.NoHook, " ") + "]"
	case e.Target == ir.TargetGo:
		pl := ir.PlanGoNames(p, e)
		for _, fn := range pl.TextDecoders() {
			names = append(names, pl.TextDecoder(fn))
		}
	case e.Target == ir.TargetCpp:
		pl := ir.PlanCppNames(p, e)
		for _, fn := range pl.TextDecoders() {
			names = append(names, pl.TextDecoder(fn))
		}
	case e.Target == ir.TargetTS:
		for _, fn := range ir.TextDecoded(p, e) {
			n := ir.TSTextFunctions(fn)
			names = append(names, n.Decode, n.Parse)
		}
		var imports []string
		for _, imp := range ir.TSImports(p) {
			imports = append(imports, imp.Alias)
		}
		return "decoders [" + strings.Join(names, " ") + "] imports [" + strings.Join(imports, " ") + "]"
	}
	return "decoders [" + strings.Join(names, " ") + "]"
}

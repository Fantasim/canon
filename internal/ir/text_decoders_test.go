package ir_test

import (
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// TestTextDecoderReach is DECISIONS 340: each copy of a go types emit, seen as its generator sees it (ir.CopyOf), writes Decode<Fn>File only for a result whose every reached package, enums and ref targets included, has a go emit that copy uses under a go_module root; any other result has no decoder, no finding, adds no import and leaves every generated name as it was. The golden is each copy's decoders and name plan, then the findings; every copy generates.
func TestTextDecoderReach(t *testing.T) {
	golden.Run(t, "testdata/textdecoders/*.txtar", func(t *testing.T, c golden.Case) []byte {
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
				if e.Target != ir.TargetGo || e.Mode != ir.ModeTypes {
					continue
				}
				fakeVectors(p)
				view := ir.CopyOf(w.proj, p, e)
				pl := ir.PlanGoNames(view, e)
				var names []string
				for _, fn := range pl.TextDecoders() {
					names = append(names, pl.TextDecoder(fn))
				}
				b.WriteString("== " + p.Name + " " + e.Out + ": decoders [" + strings.Join(names, " ") + "]\n")
				b.WriteString(dumpPlan(pl))
				if _, err := gogen.Generate(view, e); err != nil {
					t.Errorf("%s %s: %v", p.Name, e.Out, err)
				}
			}
		}
		b.WriteString(w.findings(t))
		return []byte(b.String())
	}, golden.Expected(planFile))
}

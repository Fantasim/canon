package ir_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// TestGenSupportScoped is decision 37 read the other way: stage E reports E8019 exactly where a generator refuses, so each case here, which its generator writes, is clean.
func TestGenSupportScoped(t *testing.T) {
	golden.Run(t, "testdata/gensupport/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		w := newWorld(t)
		for _, f := range c.Archive.Files {
			if f.Name != findingsFile {
				w.add(t, f.Name, f.Data)
			}
		}
		w.calls = w.fixtureCalls
		w.build(t)
		out := w.findings(t)
		if !strings.HasPrefix(out, noFindings) {
			t.Errorf("%s must be clean:\n%s", c.Path, out)
		}
		return []byte(out)
	}, golden.Expected(findingsFile))
}

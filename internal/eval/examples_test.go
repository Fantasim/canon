package eval_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const (
	packagesFile = "packages"
	dumpFile     = "dump.txt"
)

// IMPLEMENTATION-PLAN §6 M1: the example packages evaluate; values and findings are goldens.
func TestExamples(t *testing.T) {
	golden.Run(t, "testdata/examples/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		var dirs, pkgs []string
		for _, f := range c.Archive.Files {
			if f.Name != packagesFile {
				continue
			}
			for _, line := range strings.Split(strings.TrimSpace(string(f.Data)), "\n") {
				dir, pkg, _ := strings.Cut(line, " ")
				dirs, pkgs = append(dirs, dir), append(pkgs, pkg)
			}
		}
		b := runBuild(t, fromExamples(t, dirs...), eval.Options{}, pkgs...)
		return []byte(b.dump() + "\n" + b.findings(t))
	}, golden.Expected(dumpFile))
}

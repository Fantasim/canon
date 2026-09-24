package eval_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"golang.org/x/tools/txtar"
)

const (
	packagesFile = "packages"
	dumpFile     = "dump.txt"
)

// IMPLEMENTATION-PLAN §6 M1: the example packages evaluate; values and findings are goldens.
func TestExamples(t *testing.T) {
	golden.Run(t, "testdata/examples/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		dirs, pkgs := archivePackages(c.Archive)
		b := runBuild(t, fromExamples(t, dirs...), eval.Options{}, pkgs...)
		return []byte(b.dump() + "\n" + b.findings(t))
	}, golden.Expected(dumpFile))
}

// archivePackages are the example directories and packages an archive's packages file names.
func archivePackages(a *txtar.Archive) (dirs, pkgs []string) {
	data := strings.TrimSpace(string(archiveFile(a, packagesFile)))
	if data == "" {
		return nil, nil
	}
	for _, line := range strings.Split(data, "\n") {
		dir, pkg, _ := strings.Cut(line, " ")
		dirs, pkgs = append(dirs, dir), append(pkgs, pkg)
	}
	return dirs, pkgs
}

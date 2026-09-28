package build_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"golang.org/x/tools/txtar"
)

// archived is the file of a that has name, if any.
func archived(a *txtar.Archive, name string) ([]byte, bool) {
	for _, f := range a.Files {
		if f.Name == name {
			return f.Data, true
		}
	}
	return nil, false
}

// conformanceMark is in the name of every conformance file an emit writes (CONFORMANCE.md §7.1).
const conformanceMark = "_conformance.gen."

// contentMark heads a printed file's content; a txtar file marker would split the golden.
const contentMark = "== content of"

// EVALUATION.md §1 phase 7, DECISIONS 37, CONFORMANCE.md §6, §7: check reports what build reports; cases print findings, outputs, conformance files.
func TestConformance(t *testing.T) {
	golden.Run(t, "testdata/conform/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		p, err := build.Open(archiveFS(c.Archive), "/p", build.Options{Layers: fields(c.Archive, layersFile)})
		if err != nil {
			t.Fatal(err)
		}
		checked, checkErr := p.Check(context.Background(), fields(c.Archive, selectFile))
		if _, only := archived(c.Archive, checkOnly); only && checkErr == nil {
			return []byte(render(t, checked.Findings))
		}
		res, buildErr := p.Build(context.Background(), build.BuildOptions{Packages: fields(c.Archive, selectFile)})
		if checkErr != nil || buildErr != nil {
			return []byte(failures(checkErr, buildErr))
		}
		var b strings.Builder
		b.WriteString(render(t, res.Findings))
		if got := render(t, checked.Findings); got != b.String() {
			t.Errorf("check reports\n%s\nbuild reports\n%s", got, b.String())
		}
		for _, o := range res.Outputs {
			fmt.Fprintf(&b, "output %s %d\n", o.Path, o.Status)
		}
		for _, o := range res.Outputs {
			if strings.Contains(o.Path, conformanceMark) {
				fmt.Fprintf(&b, "%s %s\n%s", contentMark, o.Path, o.Content)
			}
		}
		return []byte(b.String())
	}, golden.Expected(buildFile))
}

// failures prints the Go errors of check and build, and whether each is a *build.LoadError or internal.
func failures(checkErr, buildErr error) string {
	var b strings.Builder
	for _, x := range []struct {
		name string
		err  error
	}{{"check", checkErr}, {"build", buildErr}} {
		var le *build.LoadError
		fmt.Fprintf(&b, "%s error (load error %t, internal %t): %v\n", x.name, errors.As(x.err, &le), errors.Is(x.err, build.ErrInternal), x.err)
	}
	return b.String()
}

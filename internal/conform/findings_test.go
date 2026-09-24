package conform_test

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/conform"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const findingsFile = "findings.txt"

// reFindingCode is a rendered finding's code, not a code a message mentions.
var reFindingCode = regexp.MustCompile(`(?:error|warning)\[([EW][0-9]{4})\]`)

// IMPLEMENTATION-PLAN.md §7.2: each case fails only conform's rule; no test call is recorded.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		var files []string
		for _, f := range c.Archive.Files {
			if f.Name != findingsFile {
				files = append(files, f.Name, string(f.Data))
			}
		}
		w := newWorld(t, files...)
		if err := conform.Fill(context.Background(), w.prog, w.pkgs, &reference{w: w}, w.bags); err != nil {
			t.Fatal(err)
		}
		out := w.findings(t)
		code := strings.SplitN(filepath.Base(c.Path), "_", 2)[0]
		if !strings.Contains(out, "["+code+"]") {
			t.Errorf("%s does not produce %s", c.Path, code)
		}
		for _, m := range reFindingCode.FindAllStringSubmatch(out, -1) {
			if m[1] != code {
				t.Errorf("%s also produces %s, not just %s:\n%s", c.Path, m[1], code, out)
			}
		}
		return []byte(out)
	}, golden.Expected(findingsFile))
}

package lock_test

import (
	"bytes"
	"path"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/lock"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"golang.org/x/tools/txtar"
)

const (
	findingsFile = "findings.txt"
	lockName     = "canon.lock"
	commandFile  = "command"
	lockCheck    = "canon lock check"
)

// lockCase is one findings case: its lock, the .canon sources of its package, and whether it
// runs as `canon lock check` (a `command` file saying so).
type lockCase struct {
	fs        *source.FileSet
	lock      *source.File
	sources   []*syntax.File
	lockCheck bool
}

func readCase(t *testing.T, a *txtar.Archive) lockCase {
	t.Helper()
	c := lockCase{fs: &source.FileSet{}}
	for _, f := range a.Files {
		if f.Name == findingsFile {
			continue
		}
		if f.Name == commandFile {
			c.lockCheck = strings.TrimSpace(string(f.Data)) == lockCheck
			continue
		}
		src, err := c.fs.Add(f.Name, "/"+f.Name, f.Data)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case path.Base(f.Name) == lockName:
			c.lock = src
		case path.Ext(f.Name) == ".canon":
			parsed := diag.NewBag(c.fs, "")
			c.sources = append(c.sources, syntax.Parse(src, syntax.FileSource, parsed))
			if len(parsed.Findings()) != 0 {
				t.Fatalf("%s does not parse: %v", f.Name, parsed.Findings())
			}
		}
	}
	if c.lock == nil {
		t.Fatal("no canon.lock in the case")
	}
	return c
}

// IMPLEMENTATION-PLAN.md §7.2, LOCK.md §2.4, §4: each case reads its lock, then compares it.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, gc golden.Case) []byte {
		t.Helper()
		c := readCase(t, gc.Archive)
		pkg := strings.ReplaceAll(path.Dir(c.lock.Path), "/", ".")
		bag := diag.NewBag(c.fs, pkg)
		l, ok := lock.Parse(c.lock.ID, c.lock.Content, pkg, bag)
		if ok && len(c.sources) > 0 {
			s := sourcesOf(t, pkg, c.sources)
			l.Verify(s, bag)
			if c.lockCheck {
				l.Pending(s, source.Span{File: c.lock.ID}, bag)
			}
		}
		var buf bytes.Buffer
		opt := diag.RenderOptions{Summary: bag.Summary(), Golden: true}
		if err := diag.Render(&buf, c.fs, bag.Findings(), opt); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}, golden.Expected(findingsFile))
}

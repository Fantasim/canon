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
	layersFile   = "layers" // `<layer> table|field <name> <file> <text>`: forbidden amendments at text
)

// lockCase is one findings case: its lock, the .canon sources of its package, and whether it
// runs as `canon lock check` (a `command` file saying so).
type lockCase struct {
	fs        *source.FileSet
	lock      *source.File
	sources   []*syntax.File
	lockCheck bool
	ids       map[string]source.FileID
	layers    []lock.LayerAmendment
}

func readCase(t *testing.T, a *txtar.Archive) lockCase {
	t.Helper()
	c := lockCase{fs: &source.FileSet{}, ids: map[string]source.FileID{}}
	var layers []byte
	for _, f := range a.Files {
		if f.Name == findingsFile {
			continue
		}
		if f.Name == commandFile {
			c.lockCheck = strings.TrimSpace(string(f.Data)) == lockCheck
			continue
		}
		if f.Name == layersFile {
			layers = f.Data
			continue
		}
		src, err := c.fs.Add(f.Name, "/"+f.Name, f.Data)
		if err != nil {
			t.Fatal(err)
		}
		c.ids[f.Name] = src.ID
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
	if layers != nil {
		c.layers = c.layerAmendments(t, layers)
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
		for _, a := range c.layers {
			lock.ReportLayer(a, bag)
		}
		var buf bytes.Buffer
		opt := diag.RenderOptions{Summary: bag.Summary(), Golden: true}
		if err := diag.Render(&buf, c.fs, bag.Findings(), opt); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}, golden.Expected(findingsFile))
}

// layerAmendments reads the layers file: a layer, a kind, a name, a file, the amendment's text.
func (c lockCase) layerAmendments(t *testing.T, data []byte) []lock.LayerAmendment {
	t.Helper()
	var out []lock.LayerAmendment
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		f := strings.SplitN(line, " ", 5)
		if len(f) != 5 {
			t.Fatalf("layers line %q", line)
		}
		id, ok := c.ids[f[3]]
		at := strings.Index(string(c.fs.Content(id)), f[4])
		if !ok || at < 0 {
			t.Fatalf("layers line %q: no %q in %s", line, f[4], f[3])
		}
		am := lock.LayerAmendment{Layer: f[0], Span: source.Span{File: id, Start: source.Pos(at), End: source.Pos(at + len(f[4]))}}
		if f[1] == "table" {
			am.Table = f[2]
		} else {
			am.Field = f[2]
		}
		out = append(out, am)
	}
	return out
}

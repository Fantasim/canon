package lock_test

import (
	"bytes"
	"flag"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/lock"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const (
	findingsFile = "findings.txt"
	lockName     = "canon.lock"
)

// IMPLEMENTATION-PLAN.md §7.2: each testdata/findings case prints the findings of its lock.
func TestFindings(t *testing.T) {
	cases, err := golden.Load("testdata/findings/*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(filepath.Base(c.Path), func(t *testing.T) {
			var buf bytes.Buffer
			for _, f := range c.Archive.Files {
				if path.Base(f.Name) != lockName {
					continue
				}
				pkg := strings.ReplaceAll(path.Dir(f.Name), "/", ".")
				files := oneFile{path: f.Name, content: f.Data}
				bag := diag.NewBag(files, pkg)
				lock.Parse(1, f.Data, pkg, bag)
				opt := diag.RenderOptions{Summary: bag.Summary(), Golden: true}
				if err := diag.Render(&buf, files, bag.Findings(), opt); err != nil {
					t.Fatal(err)
				}
			}
			checkFindings(t, c, buf.Bytes())
		})
	}
}

// checkFindings compares got with the case's findings.txt, or rewrites it under the golden
// harness's -update flag.
func checkFindings(t *testing.T, c golden.Case, got []byte) {
	t.Helper()
	if update := flag.Lookup("update"); update != nil && update.Value.String() == "true" {
		files := []txtar.File{}
		for _, f := range c.Archive.Files {
			if f.Name != findingsFile {
				files = append(files, f)
			}
		}
		c.Archive.Files = append(files, txtar.File{Name: findingsFile, Data: got})
		if err := os.WriteFile(c.Path, txtar.Format(c.Archive), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, f := range c.Archive.Files {
		if f.Name == findingsFile {
			if !bytes.Equal(f.Data, got) {
				t.Errorf("%s:\n--- got\n%s--- want\n%s", c.Path, got, f.Data)
			}
			return
		}
	}
	t.Errorf("%s has no %s", c.Path, findingsFile)
}

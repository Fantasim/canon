package check_test

import (
	"context"
	"flag"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// benchDir is a project benchgen wrote (IMPLEMENTATION-PLAN §7.6); the benchmarks skip without it.
var benchDir = flag.String("canon.bench", "", "a directory benchgen wrote, for BenchmarkRecheck and BenchmarkCheck")

// benchProject is the benchmark project, parsed: its project.canon and every source file.
type benchProject struct {
	fs    *source.FileSet
	proj  *project.Project
	files []*syntax.File
}

func loadBench(b *testing.B) *benchProject {
	b.Helper()
	if *benchDir == "" {
		b.Skip("no -canon.bench directory")
	}
	bp := &benchProject{fs: &source.FileSet{}}
	own := diag.NewBag(bp.fs, "")
	data, err := os.ReadFile(filepath.Join(*benchDir, "project.canon"))
	if err != nil {
		b.Fatal(err)
	}
	src, err := bp.fs.Add("project.canon", "/project.canon", data)
	if err != nil {
		b.Fatal(err)
	}
	if bp.proj, err = project.Load(src, own); err != nil {
		b.Fatal(err)
	}
	err = filepath.WalkDir(*benchDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != canonExt || d.Name() == projectFile {
			return err
		}
		rel, _ := filepath.Rel(*benchDir, path)
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f, err := bp.fs.Add(filepath.ToSlash(rel), "/"+filepath.ToSlash(rel), text)
		if err == nil {
			bp.files = append(bp.files, syntax.Parse(f, syntax.FileSource, own))
		}
		return err
	})
	if err != nil {
		b.Fatal(err)
	}
	return bp
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: on the benchmark project, edits of one entry file against a cold Check.
func TestRecheckBenchEqualsCold(t *testing.T) {
	if *benchDir == "" {
		t.Skip("no -canon.bench directory")
	}
	srcs := map[string]string{}
	err := filepath.WalkDir(*benchDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != canonExt || d.Name() == projectFile {
			return err
		}
		rel, _ := filepath.Rel(*benchDir, path)
		text, err := os.ReadFile(path)
		srcs[filepath.ToSlash(rel)] = string(text)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	w := newWorld(t, srcs)
	data, err := os.ReadFile(filepath.Join(*benchDir, projectFile))
	if err != nil {
		t.Fatal(err)
	}
	src, _ := w.fs.Add(projectFile, "/"+projectFile, data)
	if w.proj, err = project.Load(src, diag.NewBag(w.fs, "")); err != nil {
		t.Fatal(err)
	}
	_, s, _ := w.session()
	path := slices.Sorted(maps.Keys(srcs))[len(srcs)/2]
	for _, st := range []step{
		{path, "tier: ", `tier: "x" + `, true},
		{path, `tier: "x" + `, "tier: ", true},
		{path, "cost: ", "cost: 1 + ", true},
		{path, "rarity: ", "rarity: BOGUS\n  x: ", true},
		{path, "rarity: BOGUS\n  x: ", "rarity: ", true},
	} {
		s = w.step(s, st)
	}
}

// IMPLEMENTATION-PLAN §7.6: a cold Check of the benchmark project, for comparison.
func BenchmarkCheck(b *testing.B) {
	bp := loadBench(b)
	for b.Loop() {
		bags := check.Bags{}
		check.Check(context.Background(), bp.proj, bp.files, bags, eval.NewFolder(bags, eval.Options{}))
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: Recheck after one field of one entry file changed.
func BenchmarkRecheck(b *testing.B) {
	bp := loadBench(b)
	bags := check.Bags{}
	_, s := check.CheckSession(context.Background(), bp.proj, bp.files, bags, eval.NewFolder(bags, eval.Options{}))
	target := bp.files[len(bp.files)/2]
	text := string(target.Src.Content)
	i := strings.Index(text, "cost: ")
	if s == nil || i < 0 {
		b.Fatal("no session or no cost field")
	}
	n := 0
	for b.Loop() {
		n++
		edited := text[:i] + "cost: " + strconv.Itoa(n) + "\n" + text[i+strings.Index(text[i:], "\n")+1:]
		src, err := bp.fs.Add(target.Src.Path, target.Src.Abs, []byte(edited))
		if err != nil {
			b.Fatal(err)
		}
		f := syntax.Parse(src, syntax.FileSource, diag.NewBag(bp.fs, ""))
		bags := check.Bags{}
		_, next, ok := s.Recheck(context.Background(), []*syntax.File{f}, bags, eval.NewFolder(bags, eval.Options{}))
		if !ok {
			b.Fatal("Recheck refused a body-only edit")
		}
		s = next
	}
}

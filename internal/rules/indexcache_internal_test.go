package rules

import (
	"context"
	"maps"
	"reflect"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	cachePkg     = "a"
	cacheKept    = "a/kept.canon"
	cacheEdited  = "a/edited.canon"
	cacheReaders = 8
	cacheChecks  = 5 // the checks of keptSource and editedBefore
)

// The kept file declares record, case, variant-level and package checks.
const keptSource = `package a

record Tile {
  size: Int

  check size > 0 else "empty tile"
  warn big: size < 100 else "big tile"
}

variant Shape {
  dot
  box {
    side: Int

    check side > 0 else "flat box"
  }

  check self.kind != dot else "a dot"
}
`

const editedBefore = `package a

let tiles: [Tile] = [{ size: 1 }]

check tiles.len() > 0 else "no tile"
`

const editedAfter = `package a

let tiles: [Tile] = [{ size: 2 }]
`

// checkedFiles type-checks the files, each parsed unless reused holds its path, into one package.
func checkedFiles(t *testing.T, sources map[string]string, reused map[string]*syntax.File) (*check.Program, map[string]*syntax.File) {
	t.Helper()
	fs := &source.FileSet{}
	bags := check.Bags{cachePkg: diag.NewBag(fs, cachePkg)}
	files := map[string]*syntax.File{}
	var all []*syntax.File
	for _, path := range []string{cacheKept, cacheEdited} {
		f := reused[path]
		if f == nil {
			src, err := fs.Add(path, "/"+path, []byte(sources[path]))
			if err != nil {
				t.Fatal(err)
			}
			f = syntax.Parse(src, syntax.FileSource, bags[cachePkg])
		}
		files[path], all = f, append(all, f)
	}
	proj := project.New("demo", project.Version{Minor: 1})
	return check.Check(context.Background(), proj, all, bags, eval.NewFolder(bags, eval.Options{})), files
}

// sameIndex fails unless a and b hold the same program facts, check files, broken types and
// variant-level checks.
func sameIndex(t *testing.T, a, b *Index) {
	t.Helper()
	switch {
	case a.info != b.info:
		t.Error("infos differ")
	case !maps.Equal(a.files, b.files):
		t.Error("check files differ")
	case !maps.Equal(a.broken, b.broken):
		t.Error("broken types differ")
	case !reflect.DeepEqual(a.shared, b.shared):
		t.Error("variant-level checks differ")
	}
}

// IMPLEMENTATION-PLAN §7.6: cached indexes equal cold ones across an edit; only the new tree is walked, the old one forgotten.
func TestIndexCacheEqualsCold(t *testing.T) {
	cache := &IndexCache{}
	first, files := checkedFiles(t, map[string]string{cacheKept: keptSource, cacheEdited: editedBefore}, nil)
	ix := cache.Index(first)
	if len(ix.files) != cacheChecks || len(ix.shared) != 1 {
		t.Fatalf("index holds %d checks, %d variants", len(ix.files), len(ix.shared))
	}
	sameIndex(t, ix, NewIndex(first))
	kept := cache.cache().Of(files[cacheKept], func(*syntax.File) []*syntax.CheckDecl { return nil })
	if len(kept) == 0 {
		t.Fatal("the kept file's checks were not cached")
	}
	second, next := checkedFiles(t, map[string]string{cacheEdited: editedAfter}, map[string]*syntax.File{cacheKept: files[cacheKept]})
	sameIndex(t, cache.Index(second), NewIndex(second))
	rescanned := false
	cache.cache().Of(next[cacheKept], func(f *syntax.File) []*syntax.CheckDecl { rescanned = true; return checksIn(f) })
	gone := false
	cache.cache().Of(files[cacheEdited], func(*syntax.File) []*syntax.CheckDecl { gone = true; return nil })
	if rescanned || !gone {
		t.Errorf("kept file walked again %t, replaced tree forgotten %t; want false, true", rescanned, gone)
	}
}

// IMPLEMENTATION-PLAN §7.6: one cache serves concurrent index builds (run with -race).
func TestIndexCacheConcurrent(t *testing.T) {
	prog, _ := checkedFiles(t, map[string]string{cacheKept: keptSource, cacheEdited: editedBefore}, nil)
	cold := NewIndex(prog)
	cache := &IndexCache{}
	out := make([]*Index, cacheReaders)
	var wg sync.WaitGroup
	for i := range out {
		wg.Go(func() { out[i] = cache.Index(prog) })
	}
	wg.Wait()
	for _, ix := range out {
		sameIndex(t, ix, cold)
	}
}

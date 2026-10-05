package verify

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
)

// The kept file declares what indexing records: an enum, a variant, written refinements and
// containers on fields and lets, an asset root.
const keptSource = `package a

enum Color { red, blue }

variant Shape {
  dot
  box { side: Int(1..) }
}

record Tile {
  label: String(1..)
  tags: [String] = []
  icon: asset("icons", ext: [png])?
}

let tiles: [Tile] = [{ label: "one" }]
`

const editedBefore = `package a

let sizes: {String: Int(0..)} = { "s": 1 }
`

const editedAfter = `package a

let sizes: {String: Int(0..9)} = { "s": 1 }
let colors: [Color] = [red]
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

// sameIndex fails unless a and b index the same written types, declarations, asset directories
// and declared types.
func sameIndex(t *testing.T, a, b *Index) {
	t.Helper()
	switch {
	case !maps.Equal(a.src.types, b.src.types):
		t.Error("written types differ")
	case !maps.Equal(a.src.decls, b.src.decls):
		t.Error("declarations differ")
	case !maps.Equal(a.src.assetDirs, b.src.assetDirs):
		t.Error("asset directories differ")
	case !reflect.DeepEqual(a.declared, b.declared):
		t.Error("declared types differ")
	}
}

// IMPLEMENTATION-PLAN §7.6: cached indexes equal cold ones across an edit; only the new tree is scanned, the old one forgotten.
func TestIndexCacheEqualsCold(t *testing.T) {
	cache := &IndexCache{}
	first, files := checkedFiles(t, map[string]string{cacheKept: keptSource, cacheEdited: editedBefore}, nil)
	ix := cache.Index(first)
	if len(ix.src.types) == 0 || len(ix.src.decls) != len(files) || len(ix.src.assetDirs) != 1 {
		t.Fatalf("index holds %d types, %d declarations, %d asset roots", len(ix.src.types), len(ix.src.decls), len(ix.src.assetDirs))
	}
	sameIndex(t, ix, NewIndex(first))
	for name, f := range files {
		if !cached(cache, f) {
			t.Errorf("%s was not cached", name)
		}
	}
	kept := cache.cache().Of(files[cacheKept], func(*syntax.File) *fileSyntax { return nil })
	second, next := checkedFiles(t, map[string]string{cacheEdited: editedAfter}, map[string]*syntax.File{cacheKept: files[cacheKept]})
	sameIndex(t, cache.Index(second), NewIndex(second))
	if cache.cache().Of(next[cacheKept], func(*syntax.File) *fileSyntax { return nil }) != kept {
		t.Error("the kept file was scanned again")
	}
	if cached(cache, files[cacheEdited]) {
		t.Error("the replaced tree was kept")
	}
}

// cached reports f already scanned in c; a file c lacked is now held as an empty scan.
func cached(c *IndexCache, f *syntax.File) bool {
	hit := true
	c.cache().Of(f, func(*syntax.File) *fileSyntax { hit = false; return nil })
	return hit
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

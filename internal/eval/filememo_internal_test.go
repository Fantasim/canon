package eval

import (
	"context"
	"maps"
	"reflect"
	"runtime"
	"testing"
	"time"
	"weak"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

const (
	indexedText = `package a

/// Positive.
type Pos = Int where it > 0

/// A reward.
variant Reward {
  gold { amount: Pos = 2 }
  nothing

  check sane: true else "never"
}

/// An item.
record Item {
  /// N.
  n: Int = 1
  /// R.
  r: Reward = Reward.nothing

  check small: n < 10 else "big"
}

fn twice(n: Int) -> Int {
  return n + n
}

/// Items.
let items: table Item = {}

entry items.one { n: 3 }

test "twice" {
  expect twice(2) == 4
}
`
	indexedDecls  = 7                // Pos's predicate, Reward, sane, Item, small, twice and the test
	indexedChecks = 2                // sane and small
	fileGCWait    = 5 * time.Second  // the bound on the wait for a cleanup; only a leak pays it
	fileGCPause   = time.Millisecond // the wait between two collections
)

// parseIndexed parses indexedText into a file of its own, in a program of its own.
func parseIndexed(t *testing.T) (*syntax.File, *check.Program) {
	t.Helper()
	set := &source.FileSet{}
	src, err := set.Add("a/a.canon", "/a/a.canon", []byte(indexedText))
	if err != nil {
		t.Fatal(err)
	}
	bags := check.Bags{}
	f := syntax.Parse(src, syntax.FileSource, diag.NewBag(set, "a"))
	prog := check.Check(context.Background(), project.New("demo", project.Version{Minor: 1}), []*syntax.File{f}, bags, NewFolder(bags, Options{}))
	return f, prog
}

// held is the number of files c remembers.
func (c *fileMemo[T]) held() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.files)
}

// IMPLEMENTATION-PLAN §7.6: the per-file memo answers as a cold walk, walking each file once.
func TestFileMemoWalksAFileOnce(t *testing.T) {
	walks := 0
	c := &fileMemo[fileIndex]{derive: func(f *syntax.File) fileIndex { walks++; return indexFile(f) }}
	f, _ := parseIndexed(t)
	first := c.of(f)
	if !reflect.DeepEqual(first, indexFile(f)) {
		t.Fatalf("the memo's index differs from a cold one")
	}
	if len(first.checks) != indexedChecks || len(first.decls) != indexedDecls {
		t.Fatalf("indexed %d declarations and %d checks, want %d and %d", len(first.decls), len(first.checks), indexedDecls, indexedChecks)
	}
	if again := c.of(f); walks != 1 || &again.decls[0] != &first.decls[0] {
		t.Errorf("the second ask walked the file again (%d walks)", walks)
	}
	g, _ := parseIndexed(t)
	if other := c.of(g); walks != 2 || other.decls[0] == first.decls[0] {
		t.Errorf("a file parsed anew took the nodes of another (%d walks)", walks)
	}
	if !reflect.DeepEqual(refinedFiles.of(f), typesIn(f)) {
		t.Errorf("the memo's written types differ from a cold walk's")
	}
}

// IMPLEMENTATION-PLAN §7.6: the per-file memo holds a file weakly; its entry goes with the file.
func TestFileMemoForgetsACollectedFile(t *testing.T) {
	c := &fileMemo[fileIndex]{derive: indexFile}
	keep, _ := parseIndexed(t)
	defer runtime.KeepAlive(keep)
	c.of(keep)
	base := c.held()
	func() {
		f, _ := parseIndexed(t)
		c.of(f)
	}()
	if c.held() != base+1 {
		t.Fatalf("holds %d files, want %d", c.held(), base+1)
	}
	for deadline := time.Now().Add(fileGCWait); time.Now().Before(deadline); time.Sleep(fileGCPause) {
		runtime.GC()
		if c.held() == base {
			return
		}
	}
	t.Errorf("holds %d files after %v of collections, want %d", c.held(), fileGCWait, base)
}

// IMPLEMENTATION-PLAN §7.6 (≡ cold): an evaluator without a memo locates code as a cold walk does.
func TestFileMemoEvaluatorsAgree(t *testing.T) {
	f, prog := parseIndexed(t)
	memoed := New(prog, nil, check.Bags{}, Options{})
	memoed.UseMemo(NewMemo(), 1)
	memoed.walkFiles()
	host := New(prog, nil, check.Bags{}, Options{})
	host.walkFiles()
	cold := coldIndex(f)
	for name, x := range map[string]*index{"memo": memoed.index, "host": host.index} {
		if !maps.Equal(x.file, cold.file) || !maps.Equal(x.owner, cold.owner) {
			t.Errorf("%s: file or owner differs from a cold walk's", name)
		}
	}
	rt := itemType(prog)
	if rt == nil || host.fileOf(rt.Decl) != f || !host.fieldSite(rt.Fields[0], rt).has || host.fieldSite(rt.Fields[0], rt) != memoed.fieldSite(rt.Fields[0], rt) {
		t.Errorf("the host's field site differs from the first evaluator's")
	}
}

// IMPLEMENTATION-PLAN §7.6: an evaluator, with or without a memo, walks files through the per-file memos.
func TestFileMemoEvaluatorsReadThrough(t *testing.T) {
	for _, epoch := range []uint64{0, 1} {
		f, prog := parseIndexed(t)
		ev := New(prog, nil, check.Bags{}, Options{})
		if epoch > 0 {
			ev.UseMemo(NewMemo(), epoch)
		}
		if indexedFiles.has(f) || refinedFiles.has(f) {
			t.Fatalf("epoch %d: a file parsed anew is already held", epoch)
		}
		ev.walkFiles()
		if !indexedFiles.has(f) {
			t.Errorf("epoch %d: walkFiles did not index the file through the per-file memo", epoch)
		}
		x := refinedIn(f, prog)
		if x == nil || !ev.index.written.of(x).has || !refinedFiles.has(f) {
			t.Errorf("epoch %d: the written types were not read through the per-file memo", epoch)
		}
		runtime.KeepAlive(f)
	}
}

// refinedIn is the first refinement f writes, nil for none.
func refinedIn(f *syntax.File, prog *check.Program) *types.Refined {
	for _, ta := range typesIn(f) {
		if x, ok := prog.Info.TypeExprs[ta.t].(*types.Refined); ok {
			return x
		}
	}
	return nil
}

// has reports that c holds f.
func (c *fileMemo[T]) has(f *syntax.File) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.files[weak.Make(f)]
	return ok
}

// coldIndex is the file and owner indexes walked from f alone, without any memo.
func coldIndex(f *syntax.File) *index {
	x := emptyIndex()
	fi := indexFile(f)
	for _, n := range fi.decls {
		x.file[n] = f
	}
	for i, c := range fi.checks {
		x.owner[c] = fi.owners[i]
	}
	return x
}

// itemType is indexedText's record type, nil when the program lacks it.
func itemType(prog *check.Program) *types.RecordType {
	for _, obj := range prog.Packages[0].Decls {
		if rt, ok := obj.Type().(*types.RecordType); ok && obj.Name() == rt.Name {
			return rt
		}
	}
	return nil
}

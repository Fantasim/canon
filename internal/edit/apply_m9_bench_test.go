package edit

import (
	"flag"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

var m9Project = flag.String("edit.m9bench", "", "a benchgen project whose monster.canon BenchmarkM9Rewrite edits")

const (
	monsterDisplay = "monster/monster.canon"
	monsterRow     = 100 // the table row whose level the benchmark sets
)

// mapVerdicts is a memo of fixed points with no bound, for the benchmark.
type mapVerdicts struct {
	mu   sync.Mutex
	kept map[VerdictKey]bool
}

func (m *mapVerdicts) Fixed(k VerdictKey) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.kept[k]
}

func (m *mapVerdicts) Keep(k VerdictKey) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kept[k] = true
}

// BenchmarkM9Rewrite is M9 and Rewrite (API.md M9) of one value of monster.canon on a fresh tree,
// as each Edit's, with no memo of verdicts and with one; the file is -edit.m9bench's benchgen project.
func BenchmarkM9Rewrite(b *testing.B) {
	if *m9Project == "" {
		b.Skip("guarded by -edit.m9bench")
	}
	raw, err := os.ReadFile(filepath.Join(*m9Project, monsterDisplay))
	if err != nil {
		b.Fatal(err)
	}
	for _, run := range []struct {
		name string
		memo Verdicts
	}{{"no memo", nil}, {"memo", &mapVerdicts{kept: map[VerdictKey]bool{}}}} {
		b.Run(run.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				b.StopTimer()
				f := benchTree(b, raw)
				snap := &Snapshot{pkgs: []*check.Package{{Files: []*syntax.File{f}}}}
				a := &applier{snap: snap, env: Env{Verdicts: run.memo}}
				b.StartTimer()
				if _, err := a.canonical(monsterDisplay, raw, false); err != nil {
					b.Fatal(err)
				}
				if _, err := format.Rewrite(f, setLevel(f)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// benchTree is raw parsed afresh as a source.
func benchTree(b *testing.B, raw []byte) *syntax.File {
	b.Helper()
	var fs source.FileSet
	src, err := fs.Add(monsterDisplay, monsterDisplay, raw)
	if err != nil {
		b.Fatal(err)
	}
	return syntax.Parse(src, syntax.FileSource, diag.NewBag(&fs, ""))
}

// setLevel is the change that sets the level of table row monsterRow to 77.
func setLevel(f *syntax.File) []format.Change {
	row := f.Decls[1].(*syntax.LetDecl).Value.(*syntax.BraceLit).Items[monsterRow].(*syntax.EntryItem)
	level := row.Value.Items[1].(*syntax.FieldItem).Value
	return []format.Change{{Kind: format.Replace, Node: level, Text: "77"}}
}

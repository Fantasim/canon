package edit

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"
	"weak"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// A package holding one large data table, as a converted one is (meta/handoff
// 2026-10-08-sovereign-port-followups.md item 4): one let of rows of scalars, a row per field line.
const (
	rowsDir     = "/big"
	rowsPkg     = "rows"
	rowsDisplay = "rows/rows.canon"
	rowsProject = "project big {\n  canon: \"0.1\"\n}\n"
	rowsTypes   = `/// A large table.
package rows

/// A row of a converted table.
record Row {
  /// Model file stem.
  name: String
  /// Its id.
  number: Int(0..=1_000_000)
  /// Attached part name.
  part: String
  /// Flies.
  fly: Bool
  /// Pickable.
  pick: Bool
  /// Render scale.
  scale: Float
  /// Transparent.
  trans: Bool
  /// Casts a shadow.
  shadow: Bool
  /// Render flag.
  renderFlag: Bool
}
`
	rowsHead    = "package rows\n\n/// The rows.\nlet rows: [Row] = [\n"
	rowText     = "{ name: \"m%d\", number: %d, part: \"\", fly: false, pick: false, scale: 1.0, trans: false, shadow: true, renderFlag: true }"
	rowLines    = 11 // what the formatter lays a row out on
	rowsFixed   = 4  // the lines of rows.canon around its rows
	rowsOpsSeen = 6  // the rows of the small tables the tests edit
	rowsJSON    = "rows/rows.json"
	rowsLoaded  = "package rows\n\n/// The rows.\nlet rows: [Row] = load(\"rows.json\")\n"
	rowJSON     = `{"name": "m%d", "number": %d, "part": "", "fly": false, "pick": false, "scale": 1.0, "trans": false, "shadow": true, "renderFlag": true}`
	rowsList    = "rows:rows"
	rowAt       = "rows:rows[%d]"
	rowsScale   = "rows:rows[%d].scale"
	rowsGCWait  = 5 * time.Second
	rowsGCPause = 10 * time.Millisecond
	samplePause = 5 * time.Millisecond
	megabyte    = 1 << 20
)

// rowsFS is an in-memory project tree, its names absolute.
type rowsFS fstest.MapFS

func (m rowsFS) ReadFile(name string) ([]byte, error) {
	return fstest.MapFS(m).ReadFile(strings.TrimPrefix(name, "/"))
}

func (m rowsFS) Stat(name string) (fs.FileInfo, error) {
	return fstest.MapFS(m).Stat(strings.TrimPrefix(name, "/"))
}

func (m rowsFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fstest.MapFS(m).ReadDir(strings.TrimPrefix(name, "/"))
}

// rowsTree is the project of the large table with n rows, rows.canon in canonical layout.
func rowsTree(tb testing.TB, n int) rowsFS {
	tb.Helper()
	var b strings.Builder
	b.WriteString(rowsHead)
	for i := range n {
		fmt.Fprintf(&b, rowText, i, i)
		b.WriteString(",\n")
	}
	b.WriteString("]\n")
	data, err := formatted(rowsDisplay, syntax.FileSource, []byte(b.String()))
	if err != nil {
		tb.Fatal(err)
	}
	dir := strings.TrimPrefix(rowsDir, "/") + "/"
	return rowsFS{
		dir + "project.canon":    {Data: []byte(rowsProject)},
		dir + "rows/types.canon": {Data: []byte(rowsTypes)},
		dir + rowsDisplay:        {Data: data},
	}
}

// rowsJSONTree is the project of the table with n rows loaded from a JSON source.
func rowsJSONTree(n int) rowsFS {
	rows := make([]string, n)
	for i := range rows {
		rows[i] = "  " + fmt.Sprintf(rowJSON, i, i)
	}
	dir := strings.TrimPrefix(rowsDir, "/") + "/"
	return rowsFS{
		dir + "project.canon":    {Data: []byte(rowsProject)},
		dir + "rows/types.canon": {Data: []byte(rowsTypes)},
		dir + rowsDisplay:        {Data: []byte(rowsLoaded)},
		dir + rowsJSON:           {Data: []byte("[\n" + strings.Join(rows, ",\n") + "\n]\n")},
	}
}

// rowsEnv is the edit environment of tree and the analysis of its package, as an edit's scope.
func rowsEnv(tb testing.TB, tree rowsFS) (Env, *Snapshot) {
	tb.Helper()
	p, err := build.Open(tree, rowsDir, build.Options{})
	if err != nil {
		tb.Fatal(err)
	}
	a, err := p.AnalyzeOnly(context.Background(), []string{rowsPkg})
	if err != nil {
		tb.Fatal(err)
	}
	if n := a.Result().Summary.Errors; n > 0 {
		tb.Fatalf("%d errors: %v", n, a.Result().List)
	}
	return Env{Project: p, Host: build.EditHost}, NewSnapshot(a)
}

// setScales are n Sets of a scale, one per row, spread over rows rows.
func setScales(n, rows int) []Operation {
	ops := make([]Operation, n)
	for i := range ops {
		ops[i] = Operation{Kind: OpSet, Path: fmt.Sprintf(rowsScale, i*(rows/n)), Value: Source("2.5")}
	}
	return ops
}

// collected counts the states of states the collector has not reclaimed, after collecting
// until none is left or rowsGCWait passed.
func collected(states []weak.Pointer[check.Package]) int {
	held := len(states)
	for deadline := time.Now().Add(rowsGCWait); held > 0 && time.Now().Before(deadline); time.Sleep(rowsGCPause) {
		runtime.GC()
		held = 0
		for _, w := range states {
			if w.Value() != nil {
				held++
			}
		}
	}
	return held
}

// API.md E1 (handoff 2026-10-08 item 4): each operation is applied to the state the ones before
// it left, analyzed again; an edit holds no state but its base and the current one, so its memory
// does not grow with its operations, in a .canon or a JSON source, whatever the operations.
func TestOperationsLetGoOfTheirAnalyses(t *testing.T) {
	row := func(n int) Lit { return Source(fmt.Sprintf(rowText, n, n)) }
	scale := func(i int, v string) Operation {
		return Operation{Kind: OpSet, Path: fmt.Sprintf(rowsScale, i), Value: Source(v)}
	}
	mixed := []Operation{
		{Kind: OpSet, Path: fmt.Sprintf(rowAt, 0), Value: row(rowsOpsSeen)},
		{Kind: OpAdd, Path: rowsList, Value: row(rowsOpsSeen + 1)},
		{Kind: OpRemove, Path: fmt.Sprintf(rowAt, 1)},
		{Kind: OpMove, Path: fmt.Sprintf(rowAt, 0), Index: 2},
		scale(2, "4.5"),
		{Kind: OpSet, Path: fmt.Sprintf(rowAt, 3), Value: row(rowsOpsSeen + 2)},
		scale(0, "5.5"),
	}
	loaded := []Operation{
		scale(0, "2.5"), scale(3, "3.5"),
		{Kind: OpSet, Path: fmt.Sprintf(rowAt, 1), Value: row(rowsOpsSeen)},
		{Kind: OpRemove, Path: fmt.Sprintf(rowAt, 2)},
		scale(1, "4.5"), scale(4, "5.5"), scale(0, "6.5"),
	}
	for _, c := range []struct {
		name string
		tree rowsFS
		ops  []Operation
	}{
		{"canon, Sets of a field", rowsTree(t, rowsOpsSeen), setScales(rowsOpsSeen, rowsOpsSeen)},
		{"canon, mixed", rowsTree(t, rowsOpsSeen), mixed},
		{"json", rowsJSONTree(rowsOpsSeen), loaded},
	} {
		t.Run(c.name, func(t *testing.T) {
			env, base := rowsEnv(t, c.tree)
			if held, all := heldStates(t, env, base, c.ops); held > 0 {
				t.Errorf("the edit holds %d of the %d states its operations left behind", held, all)
			}
		})
	}
}

// heldStates applies ops one by one as Apply does, then counts the states they were resolved
// against, but the base's own and the current one, that the collector cannot reclaim.
func heldStates(t *testing.T, env Env, base *Snapshot, ops []Operation) (held, all int) {
	t.Helper()
	a := newApplier(context.Background(), env, base)
	a.multi = true
	var states []weak.Pointer[check.Package]
	for _, op := range ops {
		if err := a.operation(op); err != nil {
			t.Fatalf("%v %s: %v", op.Kind, op.Path, err)
		}
		states = append(states, weak.Make(a.snap.byPath[rowsPkg]))
	}
	between := states[1 : len(states)-1]
	held = collected(between)
	runtime.KeepAlive(a)
	runtime.KeepAlive(base)
	return held, len(between)
}

// API.md M5, M9: after each operation on a list, the text is known as a fixed point, the verdict
// the next operation's tree takes without a whole-file judgement, and a fresh judgement agrees.
func TestRewrittenTextStaysAFixedPoint(t *testing.T) {
	env, base := rowsEnv(t, rowsTree(t, rowsOpsSeen))
	row := Source(fmt.Sprintf(rowText, rowsOpsSeen, rowsOpsSeen))
	a := newApplier(context.Background(), env, base)
	a.multi = true
	for _, op := range []Operation{
		{Kind: OpSet, Path: fmt.Sprintf(rowsScale, 1), Value: Source("3.5")},
		{Kind: OpAdd, Path: "rows:rows", Value: row},
		{Kind: OpInsert, Path: "rows:rows", Index: 1, Value: row},
		{Kind: OpMove, Path: "rows:rows[0]", Index: 2},
		{Kind: OpRemove, Path: "rows:rows[3]"},
		{Kind: OpSet, Path: fmt.Sprintf(rowsScale, 0), Value: Source("1.0")},
	} {
		if err := a.operation(op); err != nil {
			t.Fatalf("%v %s: %v", op.Kind, op.Path, err)
		}
		s := a.files[rowsDisplay]
		if !s.fixed {
			t.Errorf("%v %s: the text written is not known as a fixed point", op.Kind, op.Path)
		}
		if fresh := judged(t, s.cur); fresh != s.fixed {
			t.Errorf("%v %s: known fixed %v, judged %v", op.Kind, op.Path, s.fixed, fresh)
		}
	}
}

// judged is the formatter's verdict on text, parsed afresh: no verdict given to another tree.
func judged(t *testing.T, text []byte) bool {
	t.Helper()
	var set source.FileSet
	src, err := set.Add(rowsDisplay, rowsDisplay, text)
	if err != nil {
		t.Fatal(err)
	}
	fixed, err := format.Canonical(syntax.Parse(src, syntax.FileSource, diag.NewBag(&set, "")), syntax.FileSource)
	return err == nil && fixed
}

// BenchmarkLargeTableEdit is a 36-Set edit of a table of 20K to 170K lines (handoff 2026-10-08
// item 4): its time, and its peak heap in use and the bytes it allocates per line of the table.
func BenchmarkLargeTableEdit(b *testing.B) {
	kept := recordWrites
	recordWrites = false // production Apply keeps no write steps
	defer func() { recordWrites = kept }()
	const ops = 36
	for _, lines := range []int{20_000, 80_000, 170_000} {
		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) {
			rows := (lines - rowsFixed) / rowLines
			tree := rowsTree(b, rows)
			written := bytes.Count(tree[strings.TrimPrefix(rowsDir, "/")+"/"+rowsDisplay].Data, []byte("\n"))
			env, base := rowsEnv(b, tree)
			req := Request{Ops: setScales(ops, rows)}
			var peak uint64
			var total runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&total)
			before := total.TotalAlloc
			for b.Loop() {
				stop := sampleHeap(&peak)
				p, err := Apply(context.Background(), env, base, req)
				stop()
				if err != nil {
					b.Fatal(err)
				}
				if len(p.NotCanonical) > 0 || len(p.Undo) != ops {
					b.Fatalf("not canonical %v, undo %v", p.NotCanonical, p.Undo)
				}
			}
			runtime.ReadMemStats(&total)
			b.ReportMetric(float64(peak)/megabyte, "peak-MB")
			b.ReportMetric(float64(total.TotalAlloc-before)/float64(b.N)/float64(written), "B/line")
			b.ReportMetric(float64(written), "lines")
		})
	}
}

// sampleHeap records into peak the most heap in use it sees until stop is called.
func sampleHeap(peak *uint64) (stop func()) {
	var done atomic.Bool
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		var ms runtime.MemStats
		for !done.Load() {
			runtime.ReadMemStats(&ms)
			*peak = max(*peak, ms.HeapInuse)
			time.Sleep(samplePause)
		}
	}()
	return func() {
		done.Store(true)
		<-finished
	}
}

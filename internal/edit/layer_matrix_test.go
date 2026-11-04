package edit_test

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/fantasim/canonlang/internal/edit"
)

const (
	matrixFile  = "matrix.txt"
	matrixCase0 = "testdata/layermatrix/rederived.txtar"
	matrixPerm  = 0o644
	matrixRoot  = "law/"
	matrixPkg   = "a"
	matrixLayer = "dev"
	matrixLine  = "\n"
	matrixPort  = "a:server.port"
	matrixPortN = "a:cfg.server.port"
)

// matrixView is how a matrix case edits: the layers active and the edit layer.
type matrixView struct {
	name   string
	layers []string
	edit   string
}

var (
	matrixViews = []matrixView{{"A", []string{matrixLayer}, matrixLayer}, {"B", nil, matrixLayer}, {"C", []string{matrixLayer}, ""}}
	matrixPaths = []string{"a:server.url", "a:server.host", matrixPort, "a:cfg.server.url", "a:cfg.server.host", matrixPortN, "a:cfg.tag"}
	matrixKinds = []edit.Op{edit.OpSet, edit.OpReset, edit.OpRemove}
	matrixNames = map[edit.Op]string{edit.OpSet: "set", edit.OpReset: "reset", edit.OpRemove: "remove"}
)

// API.md W10, W11, E22: every verdict, write and Undo of the reviewer's matrix over a layer that
// amends a value a default re-derives from, as testdata/layermatrix states it (-update rewrites it).
func TestLayerVerdictMatrix(t *testing.T) {
	ar, err := txtar.ParseFile(matrixCase0)
	if err != nil {
		t.Fatal(err)
	}
	fsys := mapFS{}
	kept := ar.Files[:0:0]
	for _, f := range ar.Files {
		if f.Name != matrixFile {
			fsys[matrixRoot+f.Name] = file(string(f.Data))
			kept = append(kept, f)
		}
	}
	var sb strings.Builder
	for _, mc := range matrixCases() {
		fmt.Fprintf(&sb, "[%s %s %s] %s", mc.view.name, mc.op.Path, matrixNames[mc.op.Kind], matrixOutcome(t, fsys, mc.view, mc.op))
	}
	ar.Files = append(kept, txtar.File{Name: matrixFile, Data: []byte(sb.String())})
	got := txtar.Format(ar)
	if *update {
		if err := os.WriteFile(matrixCase0, got, matrixPerm); err != nil {
			t.Fatal(err)
		}
		return
	}
	if want, _ := os.ReadFile(matrixCase0); !bytes.Equal(got, want) {
		t.Errorf("%s differs from what Apply does (run with -update and read the diff):\n%s", matrixCase0, sb.String())
	}
}

// matrixCase is one operation in one view.
type matrixCase struct {
	view matrixView
	op   edit.Operation
}

// matrixCases are the views, then the paths, then Set, Reset and Remove: a port is set to 7, any other value to "x".
func matrixCases() []matrixCase {
	var out []matrixCase
	for _, v := range matrixViews {
		for _, p := range matrixPaths {
			for _, k := range matrixKinds {
				out = append(out, matrixCase{view: v, op: matrixOp(k, p)})
			}
		}
	}
	return out
}

// matrixOp is op kind k at p, a Set's value 7 for a port and "x" for any other.
func matrixOp(k edit.Op, p string) edit.Operation {
	op := edit.Operation{Kind: k, Path: p}
	switch {
	case k != edit.OpSet:
	case p == matrixPort || p == matrixPortN:
		op.Value = edit.Source("7")
	default:
		op.Value = edit.Source(`"x"`)
	}
	return op
}

// matrixOutcome is op's verdict in v: its refusal, or the files it changes and whether its Undo restores them.
func matrixOutcome(t *testing.T, fsys mapFS, v matrixView, op edit.Operation) string {
	t.Helper()
	plan, err := matrixApply(t, fsys, v, []edit.Operation{op})
	if err != nil {
		return "refused: " + err.Error() + matrixLine
	}
	var sb strings.Builder
	sb.WriteString("applied" + matrixLine)
	after := written(fsys, plan)
	sb.WriteString(matrixDiff(fsys, after))
	undo, err := matrixApply(t, after, v, plan.Undo)
	if err != nil {
		return sb.String() + "  undo refused: " + err.Error() + matrixLine
	}
	back := written(after, undo)
	if d := matrixDiff(fsys, back); d != "" {
		return sb.String() + "  undo differs" + matrixLine + d
	}
	return sb.String() + "  undo restores" + matrixLine
}

// matrixApply applies ops in view v.
func matrixApply(t *testing.T, fsys mapFS, v matrixView, ops []edit.Operation) (*edit.Plan, error) {
	t.Helper()
	s := open(t, fsys, v.layers, v.edit, matrixPkg)
	return edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
}

// matrixDiff is each file of want that got changes, by path, its changed lines in one hunk.
func matrixDiff(want, got mapFS) string {
	var sb strings.Builder
	for _, name := range slices.Sorted(maps.Keys(got)) {
		before := ""
		if f := want[name]; f != nil {
			before = string(f.Data)
		}
		if after := string(got[name].Data); after != before {
			sb.WriteString("  " + strings.TrimPrefix(name, matrixRoot) + matrixLine + matrixHunk(before, after))
		}
	}
	return sb.String()
}

// matrixHunk is the lines between the common head and tail of a and b, removed then added.
func matrixHunk(a, b string) string {
	x, y := strings.SplitAfter(a, matrixLine), strings.SplitAfter(b, matrixLine)
	i := 0
	for i < len(x) && i < len(y) && x[i] == y[i] {
		i++
	}
	j := 0
	for j < len(x)-i && j < len(y)-i && x[len(x)-1-j] == y[len(y)-1-j] {
		j++
	}
	var sb strings.Builder
	for _, l := range x[i : len(x)-j] {
		sb.WriteString("    - " + l)
	}
	for _, l := range y[i : len(y)-j] {
		sb.WriteString("    + " + l)
	}
	return sb.String()
}

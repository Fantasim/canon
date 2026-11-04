package edit_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// csvLine is a record whose table and list fields a layer's line gives by a load of CSV, and of
// JSON beside them: the rows live in the data file, outside the layer file.
var csvLine = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/a/a.canon": file("/// A.\npackage a\n\n/// A row.\nrecord E {\n  /// A value.\n  v: Int = 0\n}\n\n" +
		"/// A holder.\nrecord R {\n  /// Rows.\n  sub: table E = { one { v: 1 } }\n  /// Rows.\n  lst: [E] = []\n}\n\n/// The holder.\nlet r: R = { }\n"),
	"law/a/ct.layer.canon": file("package a\nlayer ct\n\namend r {\n  sub: load.csv(\"t.csv\", header: true)\n}\n"),
	"law/a/cl.layer.canon": file("package a\nlayer cl\n\namend r {\n  lst: load.csv(\"l.csv\", header: true)\n}\n"),
	"law/a/jl.layer.canon": file("package a\nlayer jl\n\namend r {\n  lst: load(\"l.json\")\n}\n"),
	"law/a/t.csv":          file("$id,v\ntwo,2\n"),
	"law/a/l.csv":          file("v\n2\n"),
	"law/a/l.json":         file("[{\"v\": 2}]\n"),
}

// API.md W11a, V1, E21 (DECISIONS 411): a layer line giving a table or a list by load.csv holds no
// literal, like any load: an AddEntry or Add at it is `layer`; Set and Reset of the amended path
// itself stay `computed`. The JSON list line beside is the parity witness.
func TestW11aLoadCSVLineParity(t *testing.T) {
	cases := []struct {
		layer, path string
		add         edit.Operation
	}{
		{"ct", "a:r.sub", edit.Operation{Kind: edit.OpAddEntry, Path: "a:r.sub", Key: edit.Key("y"), Value: edit.Obj{"v": edit.Int(3)}}},
		{"ct", "a:r.sub", edit.Operation{Kind: edit.OpAddEntry, Path: "a:r.sub", Key: edit.Key("two"), Value: edit.Obj{"v": edit.Int(3)}}},
		{"cl", "a:r.lst", edit.Operation{Kind: edit.OpAdd, Path: "a:r.lst", Value: edit.Obj{"v": edit.Int(3)}}},
		{"jl", "a:r.lst", edit.Operation{Kind: edit.OpAdd, Path: "a:r.lst", Value: edit.Obj{"v": edit.Int(3)}}},
	}
	for _, c := range cases {
		for _, layers := range [][]string{{c.layer}, nil} {
			checkCSVLine(t, layers, c.layer, c.path, c.add)
		}
	}
}

// checkCSVLine checks add, a Set and a Reset at the load line's path, the layer active or not.
func checkCSVLine(t *testing.T, layers []string, layer, path string, add edit.Operation) {
	s := open(t, csvLine, layers, layer, "a")
	f := fixture{Snapshot: s.snap, files: s.a.Files()}
	ops := []edit.Operation{
		add,
		{Kind: edit.OpSet, Path: path, Value: edit.Source("[]")},
		{Kind: edit.OpReset, Path: path},
	}
	for _, op := range ops {
		want := edit.ReasonLayer
		if op.Kind == edit.OpSet || op.Kind == edit.OpReset {
			want = edit.ReasonComputed
		}
		t.Run(fmt.Sprintf("%s/layers=%s/%s%s", layer, strings.Join(layers, ","), parityOpNames[op.Kind], op.Key), func(t *testing.T) {
			checkVerdict(t, s, resolve(t, f, op.Path), op, want)
		})
	}
}

// API.md W11a, V1 (DECISIONS 411): with the layer active, every path below a load.csv line gets
// `layer` from Editable and Apply, a table's rows by key and a list's by index.
func TestW11aBelowLoadCSVParity(t *testing.T) {
	cases := []struct{ layer, row string }{{"ct", "a:r.sub[two]"}, {"cl", "a:r.lst[0]"}, {"jl", "a:r.lst[0]"}}
	for _, c := range cases {
		s := open(t, csvLine, []string{c.layer}, c.layer, "a")
		f := fixture{Snapshot: s.snap, files: s.a.Files()}
		for _, path := range []string{c.row, c.row + ".v"} {
			for _, op := range parityOps(path, edit.Int(6)) {
				t.Run(fmt.Sprintf("%s/%s%s", c.layer, parityOpNames[op.Kind], path), func(t *testing.T) {
					checkVerdict(t, s, resolve(t, f, path), op, edit.ReasonLayer)
				})
			}
		}
	}
}

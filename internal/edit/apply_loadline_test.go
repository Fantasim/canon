package edit_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// loadLine is a record whose table field a layer's line gives by a load: the table's rows live in
// the JSON file, outside the layer file.
var loadLine = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/a/a.canon": file("/// A.\npackage a\n\n/// A row.\nrecord E {\n  /// A value.\n  v: Int = 0\n}\n\n" +
		"/// A group.\nrecord G {\n  /// Rows.\n  sub: table E = { one { v: 1 } }\n}\n\n/// A holder.\nrecord R {\n  /// Rows.\n  sub: table E = { one { v: 1 } }\n  /// Group.\n  grp: G = { }\n  /// Weights.\n  m: {String: Int} = {}\n}\n\n/// The holder.\nlet r: R = { }\n\n/// Makes rows.\nfn mk() -> table E {\n  return { one { v: 1 } }\n}\n"),
	"law/a/mk.layer.canon":   file("package a\nlayer mk\n\namend r {\n  sub: mk()\n}\n"),
	"law/a/mp.layer.canon":   file("package a\nlayer mp\n\namend r {\n  m: load(\"m.json\")\n}\n"),
	"law/a/dir.layer.canon":  file("package a\nlayer dir\n\namend r {\n  sub: load.dir(\"d/*.json\")\n}\n"),
	"law/a/m.json":           file("{\"k\": 1}\n"),
	"law/a/d/q.json":         file("{\"v\": 1}\n"),
	"law/a/dev.layer.canon":  file("package a\nlayer dev\n\namend r {\n  sub: load(\"s.json\")\n}\n"),
	"law/a/nest.layer.canon": file("package a\nlayer nest\n\namend r {\n  grp: { sub: load(\"s.json\") }\n}\n"),
	"law/a/s.json":           file("{\"two\": {\"v\": 2}}\n"),
}

// API.md E21, 7.2, W11a (DECISIONS 305, 411): a layer line giving a table by a load holds no literal;
// Editable and Apply agree whatever the key of an AddEntry: `layer`, `computed` for a Set or Reset.
func TestE21LoadLineParity(t *testing.T) {
	for _, layers := range [][]string{{"dev"}, nil} {
		s := open(t, loadLine, layers, "dev", "a")
		f := fixture{Snapshot: s.snap, files: s.a.Files()}
		for _, key := range []string{"x", "true", "for", "two"} {
			op := edit.Operation{Kind: edit.OpAddEntry, Path: "a:r.sub", Key: edit.Key(key), Value: edit.Obj{"v": edit.Int(3)}}
			t.Run(fmt.Sprintf("layers=%s/%s", strings.Join(layers, ","), key), func(t *testing.T) {
				checkVerdict(t, s, resolve(t, f, op.Path), op, edit.ReasonLayer)
			})
		}
		ops := []edit.Operation{
			{Kind: edit.OpSet, Path: "a:r.sub", Value: edit.Source("{ }")},
			{Kind: edit.OpReset, Path: "a:r.sub"},
		}
		if layers != nil { // the JSON rows are in the session only while the layer is active
			ops = append(ops,
				edit.Operation{Kind: edit.OpSet, Path: "a:r.sub[two].v", Value: edit.Int(5)},
				edit.Operation{Kind: edit.OpRemove, Path: "a:r.sub[two]"},
				edit.Operation{Kind: edit.OpAdd, Path: "a:r.sub[two]", Value: edit.Int(1)})
		}
		for _, op := range ops {
			want := edit.ReasonLayer
			if op.Path == "a:r.sub" {
				want = edit.ReasonComputed
			}
			t.Run(fmt.Sprintf("layers=%s/%s%s", strings.Join(layers, ","), parityOpNames[op.Kind], op.Path), func(t *testing.T) {
				checkVerdict(t, s, resolve(t, f, op.Path), op, want)
			})
		}
	}
}

// API.md E21, W11a: the same when the load sits inside the literal of a line naming an ancestor.
func TestE21LoadInLiteralParity(t *testing.T) {
	s := open(t, loadLine, []string{"nest"}, "nest", "a")
	f := fixture{Snapshot: s.snap, files: s.a.Files()}
	for _, key := range []string{"x", "true", "for"} {
		op := edit.Operation{Kind: edit.OpAddEntry, Path: "a:r.grp.sub", Key: edit.Key(key), Value: edit.Obj{"v": edit.Int(3)}}
		t.Run(key, func(t *testing.T) { checkVerdict(t, s, resolve(t, f, op.Path), op, edit.ReasonLayer) })
	}
}

// API.md E21, 7.2, W11, W11a (DECISIONS 273, 411): an AddEntry at a layer line whose right-hand
// side is no literal is judged by the line before the key: `layer` for JSON the line loads (a
// file, a map, a directory), `computed` for an expression no Source states back, active or not.
func TestE21LoadFormsParity(t *testing.T) {
	cases := []struct {
		layer, path string
		want        edit.Reason
	}{
		{"mp", "a:r.m", edit.ReasonLayer},
		{"dir", "a:r.sub", edit.ReasonLayer},
		{"mk", "a:r.sub", edit.ReasonComputed},
	}
	for _, c := range cases {
		for _, layers := range [][]string{{c.layer}, nil} {
			checkLoadForm(t, layers, c.layer, c.path, c.want)
		}
	}
}

// checkLoadForm checks the AddEntry of three keys at path under the edit layer, active or not.
func checkLoadForm(t *testing.T, layers []string, layer, path string, want edit.Reason) {
	s := open(t, loadLine, layers, layer, "a")
	f := fixture{Snapshot: s.snap, files: s.a.Files()}
	for _, key := range []string{"y", "true", "for"} {
		op := edit.Operation{Kind: edit.OpAddEntry, Path: path, Key: edit.Key(key), Value: edit.Obj{"v": edit.Int(3)}}
		if layer == "mp" {
			op.Value = edit.Int(3)
		}
		t.Run(fmt.Sprintf("%s/layers=%s/%s", layer, strings.Join(layers, ","), key), func(t *testing.T) {
			checkVerdict(t, s, resolve(t, f, op.Path), op, want)
		})
	}
}

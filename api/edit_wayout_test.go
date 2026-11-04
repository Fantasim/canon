package canon_test

import (
	"context"
	"errors"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// wayoutSrc is a record whose table field a layer's line gives by a load, beside a CSV base.
const wayoutSrc = "/// P.\npackage p\n\n/// A row.\nrecord E {\n  /// V.\n  v: Int = 0\n}\n\n/// R.\nrecord R {\n  /// Rows.\n  sub: table E = {\n    one { v: 1 }\n  }\n}\n\n/// A colour.\nenum Color @codes(UInt8) {\n  red = 1\n  green = 2\n}\n\n/// The holder.\nlet r: R = {}\n\n/// A table.\nlet tb: table E = {\n  one { v: 1 }\n}\n\n/// CSV rows.\nlet bt: table E = load.csv(\"bt.csv\", header: true)\n"

// wayoutLaw is wayoutSrc with the layers dev (a JSON load) and ct (a CSV load) giving sub.
func wayoutLaw() map[string]string {
	return srcLaw(wayoutSrc, "project.canon", editLaw["project.canon"],
		"p/dev.layer.canon", "package p\nlayer dev\n\namend r {\n  sub: load(\"s.json\")\n}\n",
		"p/ct.layer.canon", "package p\nlayer ct\n\namend r {\n  sub: load.csv(\"t.csv\", header: true)\n}\n",
		"p/s.json", "{\"one\": {\"v\": 1}, \"two\": {\"v\": 2}}\n",
		"p/t.csv", "$id,v\none,1\ntwo,2\n",
		"p/bt.csv", "$id,v\ntwo,2\n")
}

// wayoutOpen opens wayoutLaw with layers active and layer the edit layer.
func wayoutOpen(t *testing.T, layers []string, layer string) *canon.Project {
	t.Helper()
	opts := project(wayoutLaw())
	opts.Layers, opts.EditLayer = layers, layer
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// API.md §7.2, DECISIONS 305: a refusal names its way out only where following it makes the op pass.
func TestEditRefusalNamesWayOut(t *testing.T) {
	const (
		load    = "edit the data file directly, or replace the load(…) with a literal"
		data    = "edit the data file directly"
		literal = "replace the expression with a literal"
		noLayer = "make the edit without an edit layer"
	)
	addRow := canon.AddEntry("p:r.sub", canon.Key("y"), canon.Obj{"v": canon.Int(3)})
	cases := []struct {
		name   string
		layers []string
		layer  string
		op     canon.Op
		want   string
	}{
		{"an entry added at a JSON load line", []string{"dev"}, "dev", addRow, "op 0: p:r.sub: value is not editable: layer: " + load},
		{"an entry added at a JSON load line, layer inactive", nil, "dev", addRow, "op 0: p:r.sub: value is not editable: layer: " + load},
		{"an entry added at a CSV load line", []string{"ct"}, "ct", addRow, "op 0: p:r.sub: value is not editable: layer: " + load},
		{"a path below a JSON load line", []string{"dev"}, "dev", canon.Set("p:r.sub[two].v", canon.Int(5)), "op 0: p:r.sub[two].v: value is not editable: layer: " + load},
		{"a path below a JSON load line, layer inactive", nil, "dev", canon.Set("p:r.sub[one].v", canon.Int(5)), "op 0: p:r.sub[one].v: value is not editable: layer: " + load},
		{"a path below a CSV load line", []string{"ct"}, "ct", canon.Set("p:r.sub[two].v", canon.Int(5)), "op 0: p:r.sub[two].v: value is not editable: layer: " + load},
		{"a path below a CSV load line, layer inactive", nil, "ct", canon.Set("p:r.sub[one].v", canon.Int(5)), "op 0: p:r.sub[one].v: value is not editable: layer: " + load},
		{"a Rename no literal line would take, below a JSON load line", []string{"dev"}, "dev", canon.Rename("p:r.sub[two]", canon.Key("z")), "op 0: p:r.sub[two]: value is not editable: layer: " + data},
		{"a Rename no literal line would take, below a JSON load line, layer inactive", nil, "dev", canon.Rename("p:r.sub[one]", canon.Key("z")), "op 0: p:r.sub[one]: value is not editable: layer: " + data},
		{"an Insert at a table entry, which no edit takes", nil, "zz", canon.Insert("p:tb[one]", 0, canon.Obj{"v": canon.Int(3)}), "op 0: p:tb[one]: value is not editable: layer"},
		{"a Set of the load line itself", []string{"dev"}, "dev", canon.Set("p:r.sub", canon.Source("{ }")), "op 0: p:r.sub: value is not editable: computed: " + literal},
		{"a Reset of the CSV load line itself", []string{"ct"}, "ct", canon.Reset("p:r.sub"), "op 0: p:r.sub: value is not editable: computed: " + literal},
		{"a Remove of a base entry the layer does not state", nil, "zz", canon.Remove("p:tb[one]"), "op 0: p:tb[one]: value is not editable: layer: " + noLayer},
		{"a Remove in a CSV base, which the base edit refuses too", nil, "zz", canon.Remove("p:bt[two]"), "op 0: p:bt[two]: value is not editable: layer"},
		{"a RenameName under an edit layer, which has no known way out", nil, "zz", canon.RenameName("p:R", "Q"), "op 0: p:R: value is not editable: layer"},
		{"a Retire of an enum member under an edit layer", nil, "zz", canon.Retire("p:Color.green"), "op 0: p:Color.green: value is not editable: layer: " + noLayer},
	}
	for _, c := range cases {
		p := wayoutOpen(t, c.layers, c.layer)
		_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{c.op}})
		var ne *canon.NotEditableError
		if !errors.As(err, &ne) || err.Error() != c.want {
			t.Errorf("API.md §7.2, DECISIONS 305: %s: %v, want %q", c.name, err, c.want)
		}
	}
}

// API.md §7.2, DECISIONS 305: the same op without the edit layer passes where that way out is printed.
func TestEditWayOutNoLayerPasses(t *testing.T) {
	for _, op := range []canon.Op{canon.Remove("p:tb[one]"), canon.Retire("p:Color.green")} {
		if _, err := wayoutOpen(t, nil, "zz").Edit(context.Background(), canon.Edit{Ops: []canon.Op{op}}); err == nil {
			t.Fatalf("%v: refused under the edit layer, want an error", op)
		}
		if _, err := wayoutOpen(t, nil, "").Edit(context.Background(), canon.Edit{Ops: []canon.Op{op}}); err != nil {
			t.Errorf("API.md §7.2, DECISIONS 305: %v without the edit layer: %v, want it to pass", op, err)
		}
	}
}

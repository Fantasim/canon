package edit_test

import (
	"context"
	"maps"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// The texts the followed rows put in place of a load or an expression: the literal of its value.
const (
	jsonRows = "{ one { v: 1 }, two { v: 2 } }"
	csvRows  = "{ one { v: 5 }, two { v: 2 } }"
	csvLoad  = "load.csv(\"t.csv\", header: true)"
	jsonLoad = "load(\"s.json\")"
)

// followed is a way out followed: the layer file rewritten with from replaced by to, or with to
// "" the same op made without the edit layer.
type followed struct {
	layer, from, to string
}

// followRow is a refused op, the way out its refusal prints, and that way followed.
type followRow struct {
	layer  string
	in     states
	op     edit.Operation
	way    string
	follow followed
}

// noLayer follows wayNoLayer: the same op without the edit layer.
var noLayer = followed{}

// followRows are TestWayOutFollowed's cases: every (op, way) pair TestWayOutRefusals prints an edit way for.
var followRows = []followRow{
	{"dev", inBoth, wayOp(edit.OpSet, "a:r.sub[one].v"), edit.WayLoad, followed{"dev", jsonLoad, jsonRows}},
	{"dev", inBoth, wayOp(edit.OpReset, "a:r.sub[one].v"), edit.WayLoad, followed{"dev", jsonLoad, jsonRows}},
	{"dev", inBoth, wayOp(edit.OpRemove, "a:r.sub[one]"), edit.WayLoad, followed{"dev", jsonLoad, jsonRows}},
	{"dev", inBoth, wayOp(edit.OpAddEntry, "a:r.sub"), edit.WayLoad, followed{"dev", jsonLoad, jsonRows}},
	{"dev", inBoth, setTo("a:r.sub", "{ }"), edit.WayLiteral, followed{"dev", jsonLoad, jsonRows}},
	{"dev", inBoth, wayOp(edit.OpReset, "a:r.sub"), edit.WayLiteral, followed{"dev", jsonLoad, jsonRows}},
	{"ct", inBoth, wayOp(edit.OpSet, "a:r.sub[one].v"), edit.WayLoad, followed{"ct", csvLoad, csvRows}},
	{"ct", inBoth, wayOp(edit.OpReset, "a:r.sub"), edit.WayLiteral, followed{"ct", csvLoad, csvRows}},
	{"ct", inBoth, wayOp(edit.OpAddEntry, "a:r.sub"), edit.WayLoad, followed{"ct", csvLoad, csvRows}},
	{"cl", inBoth, wayOp(edit.OpSet, "a:r.lst[0].v"), edit.WayLoad, followed{"cl", "load.csv(\"l.csv\", header: true)", "[{ v: 2 }]"}},
	{"cl", inBoth, setTo("a:r.lst", "[]"), edit.WayLiteral, followed{"cl", "load.csv(\"l.csv\", header: true)", "[{ v: 2 }]"}},
	{"tx", inBoth, edit.Operation{Kind: edit.OpSet, Path: "a:r.s", Value: edit.Str("x")}, edit.WayLiteral, followed{"tx", "load.text(\"s.txt\")", "\"hi\\n\""}},
	{"tx", inBoth, wayOp(edit.OpReset, "a:r.s"), edit.WayLiteral, followed{"tx", "load.text(\"s.txt\")", "\"hi\\n\""}},
	{"df", inBoth, edit.Operation{Kind: edit.OpAddEntry, Path: "a:r.ds", Key: edit.Key("B"), Value: edit.Obj{"value": edit.Int(2)}}, edit.WayLoad, followed{"df", "load.defines(\"d.h\")", "{ A { value: 1 } }"}},
	{"tl", inBoth, setTo("a:tb[one]", "{ v: 2 }"), edit.WayLiteral, followed{"tl", "load(\"one.json\")", "{ v: 3 }"}},
	{"tl", inBoth, wayOp(edit.OpSet, "a:tb[one].v"), edit.WayLoad, followed{"tl", "load(\"one.json\")", "{ v: 3 }"}},
	{"ta", inActive, wayOp(edit.OpRemove, "a:tb[two]"), edit.WayLiteral, followed{"ta", "load(\"two.json\")", "{ v: 4 }"}},
	{"nj", inBoth, wayOp(edit.OpSet, "a:r.w.sub[one].v"), edit.WayLoad, followed{"nj", jsonLoad, jsonRows}},
	{"nj", inBoth, wayOp(edit.OpRemove, "a:r.w.sub[one]"), edit.WayLoad, followed{"nj", jsonLoad, jsonRows}},
	{"nj", inBoth, wayOp(edit.OpAddEntry, "a:r.w.sub"), edit.WayLoad, followed{"nj", jsonLoad, jsonRows}},
	{"nj", inBoth, setTo("a:r.w.sub", "{ }"), edit.WayLiteral, followed{"nj", jsonLoad, jsonRows}},
	{"nj", inBoth, wayOp(edit.OpReset, "a:r.w.sub"), edit.WayLiteral, followed{"nj", jsonLoad, jsonRows}},
	{"nc", inBoth, wayOp(edit.OpSet, "a:r.w.sub[one].v"), edit.WayLoad, followed{"nc", csvLoad, csvRows}},
	{"nc", inBoth, wayOp(edit.OpRemove, "a:r.w.sub[one]"), edit.WayLoad, followed{"nc", csvLoad, csvRows}},
	{"nc", inBoth, wayOp(edit.OpAddEntry, "a:r.w.sub"), edit.WayLoad, followed{"nc", csvLoad, csvRows}},
	{"nc", inBoth, setTo("a:r.w.sub", "{ }"), edit.WayLiteral, followed{"nc", csvLoad, csvRows}},
	{"nc", inBoth, wayOp(edit.OpReset, "a:r.w.sub"), edit.WayLiteral, followed{"nc", csvLoad, csvRows}},
	{"mk", inBoth, wayOp(edit.OpAddEntry, "a:r.sub"), edit.WayLiteral, followed{"mk", "mk()", "{ one { v: 1 } }"}},
	{"mk", inBoth, wayOp(edit.OpSet, "a:r.sub[one].v"), edit.WayLiteral, followed{"mk", "mk()", "{ one { v: 1 } }"}},
	{"mk", inBoth, wayOp(edit.OpRemove, "a:r.sub[one]"), edit.WayLiteral, followed{"mk", "mk()", "{ one { v: 1 } }"}},
	{"mk", inBoth, setTo("a:r.sub", "{ }"), edit.WayLiteral, followed{"mk", "mk()", "{ one { v: 1 } }"}},
	{"mk", inBoth, wayOp(edit.OpReset, "a:r.sub"), edit.WayLiteral, followed{"mk", "mk()", "{ one { v: 1 } }"}},
	{"fx", inBoth, setTo("a:r.w.e", "{ v: 3 }"), edit.WayLiteral, followed{"fx", "e0", "{ v: 9 }"}},
	{"fx", inBoth, wayOp(edit.OpSet, "a:r.w.e.v"), edit.WayLiteral, followed{"fx", "e0", "{ v: 9 }"}},
	{"fx", inBoth, wayOp(edit.OpReset, "a:r.w.e.v"), edit.WayLiteral, followed{"fx", "e0", "{ v: 9 }"}},
	{"fy", inBoth, wayOp(edit.OpSet, "a:r.w.e.v"), edit.WayLiteral, followed{"fy", "e0", "{ v: 9 }"}},
	{"fy", inBoth, wayOp(edit.OpSet, "a:r.w.sub[one].v"), edit.WayLiteral, followed{"fy", "mk()", "{ one { v: 1 } }"}},
	{"te", inBoth, setTo("a:tb[one]", "{ v: 2 }"), edit.WayLiteral, followed{"te", "me()", "{ v: 5 }"}},
	{"te", inBoth, wayOp(edit.OpSet, "a:tb[one].v"), edit.WayLiteral, followed{"te", "me()", "{ v: 5 }"}},
	{"sx", inBoth, wayOp(edit.OpAddEntry, "a:r.w.sub"), edit.WayLiteral, followed{"sx", "mk2()", "{ one { v: 2 } }"}},
	{"sx", inBoth, setTo("a:r.w.sub", "{ }"), edit.WayLiteral, followed{"sx", "mk2()", "{ one { v: 2 } }"}},
	{"te", inInactive, wayOp(edit.OpRename, "a:tb[one]"), edit.WayNoLayer, noLayer},
	{"te", inInactive, wayOp(edit.OpMove, "a:tb[one]"), edit.WayNoLayer, noLayer},
	{"rg", inBoth, wayOp(edit.OpReset, "a:r.g.n"), edit.WayLiteral, followed{"rg", "mkg()", "{ n: 1 }"}},
	{"dev", inBoth, setTo("a:r.sub[one]", "{ v: 7 }"), edit.WayLoad, followed{"dev", jsonLoad, jsonRows}},
	{"mk", inBoth, setTo("a:r.sub[one]", "{ v: 7 }"), edit.WayLiteral, followed{"mk", "mk()", "{ one { v: 1 } }"}},
	{"nj", inBoth, setTo("a:r.w.sub[one]", "{ v: 7 }"), edit.WayLoad, followed{"nj", jsonLoad, jsonRows}},
	{"zz", inInactive, wayOp(edit.OpRemove, "a:tb[one]"), edit.WayNoLayer, noLayer},
	{"zz", inInactive, wayOp(edit.OpRename, "a:tb[one]"), edit.WayNoLayer, noLayer},
	{"zz", inInactive, wayOp(edit.OpMove, "a:tb[one]"), edit.WayNoLayer, noLayer},
	{"zz", inInactive, wayOp(edit.OpAddEntry, "a:tb"), edit.WayNoLayer, noLayer},
	{"dl", inBoth, wayOp(edit.OpRemove, "a:tb[one]"), edit.WayNoLayer, noLayer},
	{"dl", inInactive, wayOp(edit.OpInsert, "a:r.lst"), edit.WayNoLayer, noLayer},
}

// afterWay is fsys with the way out f followed, and the edit layer the op is then made under.
func afterWay(fsys mapFS, f followed) (mapFS, string) {
	if f.to == "" {
		return fsys, ""
	}
	name := "law/a/" + f.layer + ".layer.canon"
	out := mapFS(maps.Clone(fsys))
	out[name] = file(strings.Replace(string(fsys[name].Data), f.from, f.to, 1))
	return out, f.layer
}

// API.md 7.2, W11, W11a (DECISIONS 305, 462): each row is refused
// with its way out, and once the way is followed the same op passes, Editable and Apply alike.
func TestWayOutFollowed(t *testing.T) {
	for _, c := range followRows {
		for _, layers := range c.in.sessions(c.layer) {
			t.Run(c.layer+"/"+c.op.Path, func(t *testing.T) {
				if got := askWay(t, wayLaw, layers, c.layer, c.op); got.applyWay != c.way || got.editWay != c.way {
					t.Fatalf("layers %v: %d %s: way %q / %q before, want %q", layers, c.op.Kind, c.op.Path, got.editWay, got.applyWay, c.way)
				}
				fsys, layer := afterWay(wayLaw, c.follow)
				if got := askWay(t, fsys, layers, layer, c.op); got.applyReason != edit.ReasonNone || got.editReason != edit.ReasonNone {
					t.Errorf("layers %v: %d %s: after the way out Editable %d, Apply %d, want both to pass", layers, c.op.Kind, c.op.Path, got.editReason, got.applyReason)
				}
			})
		}
	}
}

// DECISIONS 305, 462: every edit way TestWayOutRefusals prints
// has a row of TestWayOutFollowed that follows it, in each state it prints in.
func TestWayOutEveryEditWayFollowed(t *testing.T) {
	type key struct {
		layer, path string
		kind        edit.Op
		layers      int
	}
	seen := map[key]string{}
	for _, c := range followRows {
		for _, layers := range c.in.sessions(c.layer) {
			seen[key{c.layer, c.op.Path, c.op.Kind, len(layers)}] = c.way
		}
	}
	for _, c := range wayRows {
		if c.way == "" || c.way == edit.WayData {
			continue // the data way is an action outside Canon (DECISIONS 462)
		}
		for _, layers := range c.in.sessions(c.layer) {
			if w, ok := seen[key{c.layer, c.op.Path, c.op.Kind, len(layers)}]; !ok || w != c.way {
				t.Errorf("%s, layers %v: %q has no followed row", c.name, layers, c.way)
			}
		}
	}
}

// withheld are refusals that print no literal way out, with that way followed anyway: the same op
// still fails, which is why it is withheld (DECISIONS 462): a Reset whose base value no literal
// states, a Remove of a stable table's entry the base holds.
var withheld = []followRow{
	{"rg", inBoth, wayOp(edit.OpReset, "a:r.g.rg"), "", followed{"rg", "mkg()", "{ n: 1 }"}},
	{"si", inBoth, wayOp(edit.OpRemove, "a:ids[one]"), "", followed{"si", "load(\"one.json\")", "{ v: 3 }"}},
}

// API.md W11, W11a (DECISIONS 305, 462): a refusal withholds the literal way out where following
// it would not make the op pass; its twins that pass print it (followRows).
func TestWayOutWithheld(t *testing.T) {
	for _, c := range withheld {
		for _, layers := range c.in.sessions(c.layer) {
			t.Run(c.layer+"/"+c.op.Path, func(t *testing.T) {
				if got := askWay(t, wayLaw, layers, c.layer, c.op); got.applyReason == edit.ReasonNone || got.applyWay != "" || got.editWay != "" {
					t.Fatalf("layers %v: %d %s: %+v, want a refusal with no way out", layers, c.op.Kind, c.op.Path, got)
				}
				fsys, layer := afterWay(wayLaw, c.follow)
				s := open(t, fsys, layers, layer, "a")
				if _, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}}); err == nil {
					t.Errorf("layers %v: %d %s: passes once the literal is in place: the way out was due", layers, c.op.Kind, c.op.Path)
				}
			})
		}
	}
}

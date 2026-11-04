package edit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// wayLaw is a record whose fields layers give by loads, expressions and literals.
var wayLaw = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/a/a.canon": file("/// A.\npackage a\n\n/// A row.\nrecord E {\n  /// V.\n  v: Int = 0\n}\n\n" +
		"/// A group.\nrecord W {\n  /// Rows.\n  sub: table E = { one { v: 1 } }\n  /// More rows.\n  t2: table E = { one { v: 1 } }\n  /// One.\n  e: E = { v: 1 }\n}\n\n" +
		"/// A span.\nrecord G {\n  /// No literal states it.\n  rg: Range = 0..3\n  /// N.\n  n: Int = 0\n}\n\n" +
		"/// A holder.\nrecord R {\n  /// Rows.\n  sub: table E = { one { v: 1 } }\n  /// List.\n  lst: [E] = [{ v: 1 }]\n  /// Group.\n  w: W = { }\n  /// Text.\n  s: String = \"\"\n  /// Defines.\n  ds: table Define = { }\n  /// A span.\n  g: G = { }\n}\n\n" +
		"/// Makes rows.\nfn mk() -> table E {\n  return { one { v: 1 } }\n}\n\n" +
		"/// Makes other rows.\nfn mk2() -> table E {\n  return { one { v: 2 } }\n}\n\n/// Makes a row.\nfn me() -> E {\n  return { v: 5 }\n}\n\n/// Makes a span.\nfn mkg() -> G {\n  return { n: 1 }\n}\n\n" +
		"/// An E.\nlet e0: E = { v: 9 }\n\n/// The holder.\nlet r: R = { }\n\n/// A table.\nlet tb: table E = { one { v: 1 } }\n\n" +
		"/// CSV rows.\nlet bt: table E = load.csv(\"bt.csv\", header: true)\n\n/// Computed rows.\nlet cb: table E = mk()\n\n" +
		"/// Stable rows.\nlet ids: stable table E = { one { v: 1 } }\n"),
	"law/a/bt.csv":          file("$id,v\ntwo,2\n"),
	"law/a/t.csv":           file("$id,v\none,5\ntwo,2\n"),
	"law/a/l.csv":           file("v\n2\n"),
	"law/a/s.json":          file("{\"one\": {\"v\": 1}, \"two\": {\"v\": 2}}\n"),
	"law/a/one.json":        file("{\"v\": 3}\n"),
	"law/a/two.json":        file("{\"v\": 4}\n"),
	"law/a/s.txt":           file("hi\n"),
	"law/a/d.h":             file("#define A 1\n"),
	"law/a/dev.layer.canon": file("package a\nlayer dev\n\namend r {\n  sub: load(\"s.json\")\n}\n"),
	"law/a/ct.layer.canon":  file("package a\nlayer ct\n\namend r {\n  sub: load.csv(\"t.csv\", header: true)\n}\n"),
	"law/a/cl.layer.canon":  file("package a\nlayer cl\n\namend r {\n  lst: load.csv(\"l.csv\", header: true)\n}\n"),
	"law/a/mk.layer.canon":  file("package a\nlayer mk\n\namend r {\n  sub: mk()\n}\n"),
	"law/a/dl.layer.canon":  file("package a\nlayer dl\n\namend r {\n  lst: [{ v: 2 }]\n}\n"),
	"law/a/tl.layer.canon":  file("package a\nlayer tl\n\namend tb {\n  one: load(\"one.json\")\n}\n"),
	"law/a/ta.layer.canon":  file("package a\nlayer ta\n\namend tb {\n  two: load(\"two.json\")\n}\n"),
	"law/a/nc.layer.canon":  file("package a\nlayer nc\n\namend r {\n  w: { sub: load.csv(\"t.csv\", header: true) }\n}\n"),
	"law/a/nj.layer.canon":  file("package a\nlayer nj\n\namend r {\n  w: { sub: load(\"s.json\") }\n}\n"),
	"law/a/fx.layer.canon":  file("package a\nlayer fx\n\namend r {\n  w: { e: e0 }\n}\n"),
	"law/a/fy.layer.canon":  file("package a\nlayer fy\n\namend r {\n  w: { e: e0, sub: mk() }\n}\n"),
	"law/a/tx.layer.canon":  file("package a\nlayer tx\n\namend r {\n  s: load.text(\"s.txt\")\n}\n"),
	"law/a/df.layer.canon":  file("package a\nlayer df\n\namend r {\n  ds: load.defines(\"d.h\")\n}\n"),
	"law/a/sl.layer.canon":  file("package a\nlayer sl\n\namend r {\n  w: { sub: load(\"s.json\"), t2: { one { v: 1 } }, e: { v: 2 } }\n}\n"),
	"law/a/sx.layer.canon":  file("package a\nlayer sx\n\namend r {\n  w: { sub: mk2(), t2: { one { v: 1 } }, e: { v: 2 } }\n}\n"),
	"law/a/te.layer.canon":  file("package a\nlayer te\n\namend tb {\n  one: me()\n}\n"),
	"law/a/rg.layer.canon":  file("package a\nlayer rg\n\namend r {\n  g: mkg()\n}\n"),
	"law/a/rl.layer.canon":  file("package a\nlayer rl\n\namend r {\n  g: { n: 1 }\n}\n"),
	"law/a/pc.layer.canon":  file("package a\nlayer pc\n\namend r {\n  sub: { one { v: 1 }, two { v: 2 } }\n}\n"),
	"law/a/pl.layer.canon":  file("package a\nlayer pl\n\namend r {\n  sub: {\n    one { v: 1 }\n    two { v: 2 }\n  }\n}\n"),
	"law/a/sm.layer.canon":  file("package a\nlayer sm\n\namend r {\n  w: {\n    sub: mk2()  // a call\n    t2: { one { v: 1 } }\n    e: { v: 2 }\n  }\n}\n"),
	"law/a/sn.layer.canon":  file("package a\nlayer sn\n\namend r {\n  w: {\n    sub: mk2()\n    t2: { one { v: 1 } }\n    e: { v: 2 }\n  }\n}\n"),
	"law/a/sc.layer.canon":  file("package a\nlayer sc\n\namend r {\n  w: {\n    sub: { one { v: 2 } }  // rows\n    t2: { one { v: 1 } }\n    e: { v: 2 }\n  }\n}\n"),
	"law/a/sd.layer.canon":  file("package a\nlayer sd\n\namend r {\n  w: {\n    // a call below\n    sub: mk2()\n    t2: { one { v: 1 } }  /* rows */\n    e: { v: 2 }\n  }\n}\n"),
	"law/a/si.layer.canon":  file("package a\nlayer si\n\namend ids {\n  one: load(\"one.json\")\n}\n"),
}

// wayAnswer is what Editable and Apply say of an op: reasons and ways out.
type wayAnswer struct {
	editReason, applyReason edit.Reason
	editWay, applyWay       string
}

// askWay is op judged by Editable and refused (or not) by Apply under edit layer layer, with layers
// active, in the package its path names.
func askWay(t *testing.T, fsys mapFS, layers []string, layer string, op edit.Operation) wayAnswer {
	t.Helper()
	pkg, _, _ := strings.Cut(op.Path, ":")
	s := open(t, fsys, layers, layer, pkg)
	f := fixture{Snapshot: s.snap, files: s.a.Files()}
	var ans wayAnswer
	if e, err := s.snap.Editable(resolve(t, f, op.Path), op.Kind, layer); err != nil {
		t.Fatalf("Editable(%s): %v", op.Path, err)
	} else if e.Mode == edit.ModeNone {
		ans.editReason, ans.editWay = e.Reason, e.Way()
	}
	_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
	if ne := (*edit.NotEditableError)(nil); errors.As(err, &ne) {
		ans.applyReason, ans.applyWay = ne.Reason, ne.Detail
	} else if err != nil {
		t.Fatalf("Apply(%s): %v", op.Path, err)
	}
	return ans
}

// states are the sessions a row is asked in: the edit layer active, not active, or both.
type states uint8

const (
	inBoth states = iota
	inActive
	inInactive
)

// sessions are the active layers of each state of in.
func (in states) sessions(layer string) [][]string {
	switch in {
	case inActive:
		return [][]string{{layer}}
	case inInactive:
		return [][]string{nil}
	}
	return [][]string{{layer}, nil}
}

// wayOp is an operation of kind k on path, with a key, a value or a case where its kind takes one.
func wayOp(k edit.Op, path string) edit.Operation {
	op := edit.Operation{Kind: k, Path: path}
	switch k {
	case edit.OpSet:
		op.Value = edit.Int(7)
	case edit.OpAdd, edit.OpInsert:
		op.Value = edit.Obj{"v": edit.Int(3)}
	case edit.OpAddEntry:
		op.Key, op.Value = edit.Key("y"), edit.Obj{"v": edit.Int(3)}
	case edit.OpRename:
		op.Key = edit.Key("z")
	case edit.OpSetCase:
		op.Case = "c"
	}
	return op
}

// setTo is a Set of path to the Source text src.
func setTo(path, src string) edit.Operation {
	return edit.Operation{Kind: edit.OpSet, Path: path, Value: edit.Source(src)}
}

// wayRow is an op under an edit layer, the states it is asked in, and the reason and way out
// Editable and Apply both give it there.
type wayRow struct {
	name   string
	layer  string
	in     states
	op     edit.Operation
	reason edit.Reason
	way    string
}

// API.md 7.2, W11, W11a (DECISIONS 273, 305, 411, 462): a refusal names its way out only where
// following it makes the op pass; Editable and Apply agree, and the edit layer active or not, but
// for a way the base verdict itself gives in one state only.
func TestWayOutRefusals(t *testing.T) {
	for _, c := range wayRows {
		for _, layers := range c.in.sessions(c.layer) {
			t.Run(c.name, func(t *testing.T) {
				got := askWay(t, wayLaw, layers, c.layer, c.op)
				if want := (wayAnswer{c.reason, c.reason, c.way, c.way}); got != want {
					t.Errorf("layers %v: %s: Editable %d %q, Apply %d %q, want %d %q", layers, c.name, got.editReason, got.editWay, got.applyReason, got.applyWay, c.reason, c.way)
				}
			})
		}
	}
}

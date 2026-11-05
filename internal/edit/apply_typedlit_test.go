package edit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// typedLitCase is a request on branchFS: the value its root then holds, and a text the Canon file
// it writes then holds ("" for none to look for).
type typedLitCase struct {
	name, root, want, text string
	ops                    []edit.Operation
}

// API.md V1, 8.2, M7, E22 (DECISIONS 171, 475): a record on a branch a bare brace does not read as
// is written typed (`RB { … }`): a Source, Obj, Map, List, Insert or AddEntry applies, checks clean
// and its Undo restores (FromJSON: TestFromJSONSecondBranchCanon; a case: TestTypedCaseRefused).
func TestTypedLiteralSecondBranch(t *testing.T) {
	kb, x1 := edit.Member("b"), edit.Obj{"x": edit.Int(1)}
	for _, c := range []typedLitCase{
		{"Source, top level", "cb", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setTo("cb.t", "RB { x: 1 }")}},
		{"Source, qualified by its package", "cb", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setTo("cb.t", "d.RB { x: 1 }")}},
		{"Source, nested", "ch", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setTo("ch.h", "{ k: b, t: RB { x: 1 } }")}},
		{"Source, driver set before", "cn", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setAt("cn.k", kb), setTo("cn.t", "RB { x: 1 }")}},
		{"Source, list element", "cn", "l: [RB{x: 1}]", "[RB { x: 1 }]", []edit.Operation{setTo("cn", "{ k: b, l: [RB { x: 1 }] }")}},
		{"Source, dependent map value", "cm", "db: RB{x: 1}", "db: RB { x: 1 }", []edit.Operation{setTo("cm.m", "{ db: RB { x: 1 } }")}},
		{"Obj, top level", "cb", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setAt("cb.t", x1)}},
		{"Obj, driver set before", "cn", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setAt("cn.k", kb), setAt("cn.t", x1)}},
		{"Obj, nested", "ch", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setAt("ch.h", edit.Obj{"k": kb, "t": x1})}},
		{"Obj, nested two levels", "ch", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setAt("ch", edit.Obj{"h": edit.Obj{"k": kb, "t": x1}})}},
		{"Obj, holding the branch's default", "cb", "t: RB{x: 5}", "t: RB {}", []edit.Operation{setAt("cb.t", edit.Obj{})}},
		{"List, element", "cn", "l: [RB{x: 1}]", "[RB { x: 1 }]", []edit.Operation{setAt("cn", edit.Obj{"k": kb, "l": edit.List{x1}})}},
		{"Map, dependent map value", "cm", "db: RB{x: 1}", "db: RB { x: 1 }", []edit.Operation{setAt("cm.m", edit.Map{{Key: edit.Key("db"), Value: x1}})}},
		{"Insert, Obj", "cb", "l: [RB{x: 1}]", "[RB { x: 1 }]", []edit.Operation{{Kind: edit.OpInsert, Path: "cb.l", Index: 0, Value: x1}}},
		{"Insert, Source", "cb", "l: [RB{x: 1}]", "[RB { x: 1 }]", []edit.Operation{{Kind: edit.OpInsert, Path: "cb.l", Index: 0, Value: edit.Source("RB { x: 1 }")}}},
		{"AddEntry, Obj", "ce", "db: RB{x: 1}", "db: RB { x: 1 }", []edit.Operation{{Kind: edit.OpAddEntry, Path: "ce.m", Key: edit.Key("db"), Value: x1}}},
		{"AddEntry, Source", "ce", "db: RB{x: 1}", "db: RB { x: 1 }", []edit.Operation{{Kind: edit.OpAddEntry, Path: "ce.m", Key: edit.Key("db"), Value: edit.Source("RB { x: 1 }")}}},
		{"JSON, a Source typed literal", "jh", "t: RB{x: 0}", "", []edit.Operation{setTo("jh.h", "{ k: b, t: RB { x: 0 } }")}},
	} {
		typedLitApplies(t, branchFS(), c)
	}
}

// typedLitApplies applies c to fsys: its root then holds c.want, d.canon c.text, nothing is in
// error, and the Undo restores (E22), twice.
func typedLitApplies(t *testing.T, fsys mapFS, c typedLitCase) {
	t.Helper()
	edited, ok := applied(t, c.name, fsys, c.ops)
	if !ok {
		return
	}
	if got := values(open(t, edited.fsys, nil, "", "d"), false)["d:"+c.root]; !strings.Contains(got, c.want) {
		t.Errorf("%s: %s = %s, want it to hold %s", c.name, c.root, got, c.want)
	}
	if text := string(edited.fsys["law/d/d.canon"].Data); !strings.Contains(text, c.text) {
		t.Errorf("%s: d.canon does not hold %q:\n%s", c.name, c.text, text)
	}
	checkClean(t, c.name, fsys, c.ops)
	undoTwice(t, c.name, fsys, c.ops)
}

// API.md V1, W10, E22; DECISIONS 475: under an active edit layer, a Set on the second branch writes
// its amendment line as a typed literal, and its Undo restores the layer's lines.
func TestTypedLiteralEditLayer(t *testing.T) {
	kb := edit.Member("b")
	view := undoView{[]string{"dev"}, "dev"}
	for _, c := range []struct {
		name string
		ops  []edit.Operation
	}{
		{"Obj", []edit.Operation{setAt("lt.e2.k", kb), setAt("lt.e2.t", edit.Obj{"x": edit.Int(4)})}},
		{"Source", []edit.Operation{setAt("lt.e2.k", kb), setTo("lt.e2.t", "RB { x: 4 }")}},
		{"FromJSON", []edit.Operation{setAt("lt.e2.k", kb), setAt("lt.e2.t", edit.FromJSON(`{"x": 4}`))}},
	} {
		_, fs, ok := applyIn(t, c.name, layerFS(), view, c.ops)
		if !ok {
			continue
		}
		if got := values(open(t, fs, view.layers, "", "d"), false)["d:lt"]; !strings.Contains(got, "e2: LT{k: b, t: RB{x: 4}}") {
			t.Errorf("%s: lt = %s under dev", c.name, got)
		}
		if text := string(fs["law/d/dev.layer.canon"].Data); !strings.Contains(text, "e2.t: RB { x: 4 }") {
			t.Errorf("%s: the layer does not hold a typed line:\n%s", c.name, text)
		}
		undoIn(t, undoSpec{name: c.name, fsys: layerFS(), view: view, ops: c.ops, views: [][]string{{"dev"}, nil}, lines: []string{"law/d/dev.layer.canon"}})
	}
}

// API.md V1, 8.2 (DECISIONS 171, 475): a Source's typed literal names its record's computed branch,
// else it is a ValueError, nothing written (another branch, no branch, another package); so is a
// bare brace on a branch the checker does not read it as, a Set's or an item's alike.
func TestTypedLiteralRefused(t *testing.T) {
	bare := edit.Source("{ x: 1 }")
	refused(t, branchFS(), []namedOps{
		{"another branch", []edit.Operation{setTo("cb.t", "RA { x: 1 }")}},
		{"another branch, nested", []edit.Operation{setTo("ch.h", "{ k: b, t: RA { x: 1 } }")}},
		{"a type that is no branch", []edit.Operation{setTo("cb.t", "HT {}")}},
		{"a type that is no branch, nested", []edit.Operation{setTo("ch.h", "{ k: b, t: OT {} }")}},
		{"another package", []edit.Operation{setTo("cb.t", "e.RB { x: 1 }")}},
		{"a bare brace", []edit.Operation{setTo("cb.t", "{ x: 1 }")}},
		{"a bare brace, nested", []edit.Operation{setTo("ch.h", "{ k: b, t: { x: 1 } }")}},
		{"Insert, a bare brace", []edit.Operation{{Kind: edit.OpInsert, Path: "cb.l", Index: 0, Value: bare}}},
		{"AddEntry, a bare brace", []edit.Operation{{Kind: edit.OpAddEntry, Path: "ce.m", Key: edit.Key("db"), Value: bare}}},
	})
}

// API.md V1, M7 (DECISIONS 378, 475): an edit prints a case bare, which the checker never reads at
// a dependent position: a case there, on any branch, is a ValueError, nothing written, given as
// Obj, Case, Source (`SB.sq { … }` too), FromJSON or an item.
func TestTypedCaseRefused(t *testing.T) {
	kb := edit.Member("b")
	sq := edit.Case{Name: "sq", Fields: edit.Obj{"n": edit.Int(1)}}
	dot := edit.Case{Name: "dot", Fields: edit.Obj{"n": edit.Int(1)}}
	refused(t, branchFS(), []namedOps{
		{"Obj", []edit.Operation{setAt("vn", edit.Obj{"k": kb, "s": sq})}},
		{"Case, driver set before", []edit.Operation{setAt("vn.k", kb), setAt("vn.s", sq)}},
		{"first branch", []edit.Operation{setAt("vn", edit.Obj{"k": edit.Member("a"), "s": dot})}},
		{"Source, typed", []edit.Operation{setTo("vn", "{ k: b, s: SB.sq { n: 1 } }")}},
		{"Source, typed, driver set before", []edit.Operation{setAt("vn.k", kb), setTo("vn.s", "SB.sq { n: 1 }")}},
		{"Source, another variant's case", []edit.Operation{setTo("vn", "{ k: b, s: SA.dot { n: 1 } }")}},
		{"Source, bare", []edit.Operation{setTo("vn", "{ k: b, s: sq { n: 1 } }")}},
		{"FromJSON, nested", []edit.Operation{setAt("vn", edit.FromJSON(`{"k": "b", "s": {"kind": "sq", "n": 1}}`))}},
	})
}

// API.md E22, M7 (DECISIONS 378, 466, 474, 475): an edit removing a case's typed literal would have an
// Undo printed bare, which no operation applies: it is refused with that ValueError, nothing
// written; a write into that literal keeps its name (TestDependentSecondBranchInTypedLiteral).
func TestTypedCaseUndoRefused(t *testing.T) {
	refused(t, branchFS(), []namedOps{
		{"Reset the case", []edit.Operation{reset("vb.s")}},
		{"Set its holder to its default", []edit.Operation{setAt("vb", edit.Obj{})}},
		{"Set the case to none", []edit.Operation{setAt("vb.s", edit.None{})}},
	})
}

// API.md M7 (DECISIONS 171): a record on the branch a bare brace reads as is written bare.
func TestTypedLiteralFirstBranchBare(t *testing.T) {
	ops := []edit.Operation{setAt("cn", edit.Obj{"k": edit.Member("a"), "t": edit.Obj{"x": edit.Int(1)}})}
	edited, ok := applied(t, "first branch", branchFS(), ops)
	if !ok {
		return
	}
	if text := string(edited.fsys["law/d/d.canon"].Data); !strings.Contains(text, "let cn: OO = { k: a, t: { x: 1 } }") || strings.Contains(text, "t: RA {") {
		t.Errorf("first branch: d.canon does not hold a bare brace:\n%s", text)
	}
	checkClean(t, "first branch", branchFS(), ops)
	undoTwice(t, "first branch", branchFS(), ops)
}

// namedOps is a request and its name.
type namedOps struct {
	name string
	ops  []edit.Operation
}

// refused reports each request of cases fsys does not refuse with ErrBadValue and no plan.
func refused(t *testing.T, fsys mapFS, cases []namedOps) {
	t.Helper()
	for _, c := range cases {
		s := open(t, fsys, nil, "", "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: c.ops})
		if !errors.Is(err, edit.ErrBadValue) || plan != nil {
			t.Errorf("%s: planned %v, %v, want ErrBadValue and no plan", c.name, plan != nil, err)
		}
	}
}

// crossFS is branchFS with packages naming d's types through each form of import.
func crossFS() mapFS {
	fs := branchFS()
	fs["law/e/e.canon"] = file("package e\n\nimport d\n\n/// Through d.\nlet ek: d.HT = {}\n")
	fs["law/f/f.canon"] = file("package f\n\nimport d as dd\n\n/// Through an alias.\nlet fk: dd.HT = {}\n")
	fs["law/g/g.canon"] = file("package g\n\nimport d { HT, RB }\n\n/// Through its names.\nlet gk: HT = {}\n")
	fs["law/h/h.canon"] = file("package h\n\nimport d { HT }\n\n/// Without RB.\nlet hk: HT = {}\n")
	return fs
}

// API.md M7, V1, E22; DECISIONS 475: a typed literal is written with the name the file it goes in
// gives its type: bare in its own package and where an import brings the name, qualified by an
// import's alias or last segment else. A file that cannot name the branch refuses the value.
func TestTypedLiteralCrossPackage(t *testing.T) {
	ops := func(root string) []edit.Operation {
		return []edit.Operation{setAt(root+".h", edit.Obj{"k": edit.Member("b"), "t": edit.Obj{"x": edit.Int(1)}})}
	}
	for _, c := range []struct{ pkg, root, file, text string }{
		{"e", "ek", "law/e/e.canon", "t: d.RB { x: 1 }"},
		{"f", "fk", "law/f/f.canon", "t: dd.RB { x: 1 }"},
		{"g", "gk", "law/g/g.canon", "t: RB { x: 1 }"},
	} {
		s := open(t, crossFS(), nil, "", "d", c.pkg)
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops(c.pkg + ":" + c.root)})
		if err != nil {
			t.Errorf("%s: %v", c.pkg, err)
			continue
		}
		fs := written(crossFS(), plan)
		if text := string(fs[c.file].Data); !strings.Contains(text, c.text) {
			t.Errorf("%s: %s does not hold %q:\n%s", c.pkg, c.file, c.text, text)
		}
		after := open(t, fs, nil, "", "d", c.pkg)
		if got := values(after, false)[c.pkg+":"+c.root]; !strings.Contains(got, "t: RB{x: 1}") {
			t.Errorf("%s: %s = %s", c.pkg, c.root, got)
		}
		back, err := edit.Apply(context.Background(), after.env, after.snap, edit.Request{Ops: plan.Undo})
		if err != nil {
			t.Errorf("%s, Undo: %v", c.pkg, err)
			continue
		}
		want := values(open(t, crossFS(), nil, "", "d", c.pkg), false)[c.pkg+":"+c.root]
		if got := values(open(t, written(fs, back), nil, "", "d", c.pkg), false)[c.pkg+":"+c.root]; got != want {
			t.Errorf("%s, Undo: %s = %s, want %s", c.pkg, c.root, got, want)
		}
	}
	s := open(t, crossFS(), nil, "", "d", "h")
	if plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops("h:hk")}); !errors.Is(err, edit.ErrBadValue) || plan != nil {
		t.Errorf("h: planned %v, %v, want ErrBadValue and no plan", plan != nil, err)
	}
}

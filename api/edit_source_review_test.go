package canon_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// srcHead declares M (hp required, lvl defaulted) and base, a name no literal may hold.
const srcHead = "package p\n\nrecord M {\n  hp: Int\n  lvl: Int = 1\n}\n\nlet base: Int = 5\n\n"

// srcMain is p's source file, the one most cases compare.
const srcMain = "p/p.canon"

// srcLaw is package p with text as its source, and more files.
func srcLaw(text string, more ...string) map[string]string {
	law := map[string]string{srcMain: text}
	for i := 0; i+1 < len(more); i += 2 {
		law[more[i]] = more[i+1]
	}
	return law
}

// table is srcHead and ms, a table of M whose entries are body.
func table(kind, body string) string {
	return srcHead + "let ms: " + kind + "table M = {\n" + body + "}\n"
}

// srcCase is an edit on a valueless root: refused with want, or applied, its Undo of the kinds
// undo (nil: unchecked) giving file back.
type srcCase struct {
	name string
	law  map[string]string
	edit canon.Edit
	want error
	undo []canon.Op
	file string
}

func runSource(t *testing.T, rule string, c srcCase) {
	t.Helper()
	ctx := context.Background()
	p, m := openEdit(t, c.law)
	before := read(t, m, c.file)
	res, err := p.Edit(ctx, c.edit)
	if c.want != nil {
		if !errors.Is(err, c.want) || read(t, m, c.file) != before {
			t.Errorf("%s, %s: %v, want %v and nothing written", rule, c.name, err, c.want)
		}
		return
	}
	if err != nil {
		t.Errorf("%s, %s: %v", rule, c.name, err)
		return
	}
	if c.undo != nil && !slices.EqualFunc(res.Undo, c.undo, sameOp) {
		t.Errorf("%s, %s: Undo %+v, want %+v", rule, c.name, res.Undo, c.undo)
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, AllowErrors: true, Ops: res.Undo}); err != nil {
		t.Errorf("API.md E22, %s: the Undo %+v: %v", c.name, res.Undo, err)
	} else if got := read(t, m, c.file); got != before {
		t.Errorf("API.md E22, %s: %s after the Undo:\n%s\nwant\n%s", c.name, c.file, got, before)
	}
}

func allow(ops ...canon.Op) canon.Edit { return canon.Edit{AllowErrors: true, Ops: ops} }

// API.md E19, 8.2 (309 review 1): an op whose inverse would carry text that is no literal (a
// name, an operator) is refused with the root's error, nothing written.
func TestSourceInverseLiteralOnly(t *testing.T) {
	law := srcLaw(table("", "  wolf { hp: 1 }\n  bat { hp: base, lvl: base + 1 }\n  cat {}\n"))
	for _, op := range []canon.Op{canon.Remove("p:ms.bat"), canon.Set("p:ms.bat.hp", canon.Int(2)),
		canon.Set("p:ms.bat", canon.Obj{"hp": canon.Int(2)}), canon.Reset("p:ms.bat.lvl")} {
		runSource(t, "API.md E19, 8.2", srcCase{name: string(op.Kind), law: law, edit: allow(op), want: canon.ErrNoValue, file: srcMain})
	}
}

// API.md E19, W9, E15 (309 review 4, 6): a field of an entry with a spread, and a field that
// drives a dependent field, need the evaluated record: refused with the root's error.
func TestSourceSpreadAndDriver(t *testing.T) {
	spread := srcLaw(srcHead + "let proto: M = { hp: 3, lvl: 5 }\n\nlet ms: table M = {\n  bat { ...proto, lvl: 2 }\n  cat {}\n}\n")
	driver := srcLaw("package p\n\nenum Goal { kill, collect }\n\ntype Target(g: Goal) = match g {\n  kill => String\n  collect => Int\n}\n\n" +
		"record Q {\n  hp: Int\n  goal: Goal\n  aim: Target(goal)?\n}\n\nlet qs: table Q = {\n  wolf { hp: 1, goal: kill, aim: \"x\" }\n  cat { goal: kill }\n}\n")
	for _, c := range []srcCase{
		{name: "W9 Reset", law: spread, edit: allow(canon.Reset("p:ms.bat.lvl"))},
		{name: "W9 Set", law: spread, edit: allow(canon.Set("p:ms.bat.lvl", canon.Int(7)))},
		{name: "E15 Set of a driver", law: driver, edit: allow(canon.Set("p:qs.wolf.goal", canon.Member("collect")))},
	} {
		c.want, c.file = canon.ErrNoValue, srcMain
		runSource(t, "API.md E19, W9, E15", c)
	}
}

// API.md E23, N7, E20 (309 review 2): an AddEntry into a valueless stable table, its id skipped,
// is undone by a Remove; once a request repairs the table and locks the id, its Undo re-breaks
// the table and still retires the id from the sources.
func TestSourceStableAddEntry(t *testing.T) {
	law := srcLaw(table("stable ", "  wolf { hp: 1 }\n  bat {}\n  cat {}\n"), "p/canon.lock", "# canon.lock v1\ntable  p.ms  wolf\n")
	eel := canon.AddEntry("p:ms", canon.Key("eel"), canon.Obj{"hp": canon.Int(1)})
	runSource(t, "API.md E23", srcCase{name: "skipped id", law: law, edit: allow(eel), undo: []canon.Op{canon.Remove("p:ms.eel")}, file: srcMain})
	ctx := context.Background()
	p, m := openEdit(t, law)
	res, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{eel, canon.Set("p:ms.bat.hp", canon.Int(2)), canon.Set("p:ms.cat.hp", canon.Int(3))}})
	if err != nil {
		t.Fatalf("API.md E19: repairing request: %v", err)
	}
	if last := res.Undo[len(res.Undo)-1]; last.Kind != canon.OpRetire || last.Path != "p:ms.eel" {
		t.Errorf("API.md E23: Undo %+v, want it to end with the Retire of eel", res.Undo)
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, AllowErrors: true, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md N7, E22: the Undo %+v: %v", res.Undo, err)
	}
	if got := read(t, m, srcMain); !strings.Contains(got, "retired eel { hp: 1 }") || !strings.Contains(got, "bat {}") {
		t.Errorf("API.md N7, E22: source after the Undo:\n%s", got)
	}
	if _, err := p.Edit(ctx, allow(canon.Op{Kind: canon.OpRetire, Path: "p:ms.cat"})); err != nil || !strings.Contains(read(t, m, srcMain), "retired cat {}") {
		t.Errorf("API.md N7: Retire on the valueless table: %v\n%s", err, read(t, m, srcMain))
	}
}

// API.md N1, N2, E23 (309 review 3): AddEntry into a valueless table places the entry by N1:
// a file from the op's literal under @files, the literal beside entry declarations of its own
// file, where Move works; so every Remove's Undo applies while the table stays broken.
func TestSourceAddEntryPlaced(t *testing.T) {
	files := srcLaw("package p\n\nrecord M {\n  hp: Int\n  lvl: Int = 1\n}\n\n@files(\"m/{lvl}/{id}.canon\")\nlet ms: table M = {}\n",
		"p/m/1/wolf.canon", "package p\n\nentry ms.wolf { hp: 1 }\n", "p/m/1/bat.canon", "package p\n\nentry ms.bat {}\n",
		"p/m/1/cat.canon", "package p\n\nentry ms.cat {}\n")
	decl := srcLaw(table("", "  wolf { hp: 1 }\n  bat {}\n") + "\nentry ms.owl { hp: 2 }\n")
	for _, c := range []srcCase{
		{name: "@files Remove", law: files, edit: allow(canon.Remove("p:ms.wolf")), undo: []canon.Op{canon.AddEntry("p:ms", canon.Key("wolf"), canon.Source(""))}, file: "p/m/1/wolf.canon"},
		{name: "declaration beside the literal", law: decl, edit: allow(canon.Remove("p:ms.wolf")),
			undo: []canon.Op{canon.AddEntry("p:ms", canon.Key("wolf"), canon.Source("")), canon.Move("p:ms.wolf", 0)}, file: srcMain},
	} {
		runSource(t, "API.md N1, E23", c)
	}
	p, m := openEdit(t, files)
	if _, err := p.Edit(context.Background(), allow(canon.AddEntry("p:ms", canon.Key("eel"), canon.Obj{"hp": canon.Int(2), "lvl": canon.Int(3)}))); err != nil {
		t.Fatalf("API.md N2: %v", err)
	}
	if got := read(t, m, "p/m/3/eel.canon"); got != "package p\n\nentry ms.eel { hp: 2, lvl: 3 }\n" {
		t.Errorf("API.md N2, N5: the new entry's file: %q", got)
	}
}

// API.md N1, E23: on a valueless table, the Undo of a Remove of an `entry` declared in the let's
// own file is an AddEntry alone, which N1 places in the literal, so the entry comes back there,
// after the literal's entries, not as a declaration (accepted, log-2026-10-05).
func TestSourceRemoveDeclaredEntry(t *testing.T) {
	ctx := context.Background()
	p, m := openEdit(t, srcLaw(srcHead+"let ms: table M = {\n  wolf { hp: 1 }\n  bat {}\n}\n\nentry ms.owl { hp: 2 }\n"))
	res, err := p.Edit(ctx, allow(canon.Remove("p:ms.owl")))
	if err != nil || !slices.EqualFunc(res.Undo, []canon.Op{canon.AddEntry("p:ms", canon.Key("owl"), canon.Source(""))}, sameOp) {
		t.Fatalf("API.md N1, E23: %+v, %v", res, err)
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, AllowErrors: true, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md E22: the Undo: %v", err)
	}
	if got := read(t, m, srcMain); !strings.Contains(got, "  bat {}\n  owl { hp: 2 }\n}\n") || strings.Contains(got, "entry ms.owl") {
		t.Errorf("API.md N1: source after the Undo:\n%s", got)
	}
}

// API.md E4, E19 (309 review 5): a retired entry takes Set and Reset from the sources; its
// Remove is judged by E4 first.
func TestSourceRetiredEntry(t *testing.T) {
	law := srcLaw(table("stable ", "  wolf { hp: 1 }\n  retired bat { lvl: 2 }\n  cat {}\n"), "p/canon.lock",
		"# canon.lock v1\ntable  p.ms  bat  retired\ntable  p.ms  wolf\n")
	for _, c := range []srcCase{
		{name: "Set", law: law, edit: allow(canon.Set("p:ms.bat.hp", canon.Int(3))), undo: []canon.Op{canon.Set("p:ms.bat", canon.Source(""))}},
		{name: "Reset", law: law, edit: allow(canon.Reset("p:ms.bat.lvl")), undo: []canon.Op{canon.Set("p:ms.bat.lvl", canon.Int(2))}},
		{name: "Remove", law: law, edit: allow(canon.Remove("p:ms.bat")), want: canon.ErrStableKey},
	} {
		c.file = srcMain
		runSource(t, "API.md E4, E19", c)
	}
}

// API.md E19, E23 (309 review 7): any other valueless root takes a Set of the whole root from its
// source, its inverse the old text when that is a literal; every other op on it stays refused.
func TestSourceWholeRoot(t *testing.T) {
	law := srcLaw("package p\n\nrecord C {\n  port: Int\n  name: Int\n}\n\nlet base: Int = 5\n\nlet c: C = { name: 1 }\n\nlet d: C = { name: base }\n")
	whole := canon.Obj{"port": canon.Int(1), "name": canon.Int(1)}
	for _, c := range []srcCase{
		{name: "Set of the root", law: law, edit: allow(canon.Set("p:c", whole)), undo: []canon.Op{canon.Set("p:c", canon.Source(""))}},
		{name: "old text with a name", law: law, edit: allow(canon.Set("p:d", whole)), want: canon.ErrNoValue},
		{name: "Set of a field", law: law, edit: allow(canon.Set("p:c.port", canon.Int(1))), want: canon.ErrNoValue},
	} {
		c.file = srcMain
		runSource(t, "API.md E19, E23", c)
	}
}

// API.md W7, E3, E2, E8, E9, V1 (309 review 8): the source path keeps the rules of the operations
// it writes.
func TestSourceOpRules(t *testing.T) {
	law := srcLaw(table("", "  wolf { hp: 1 }\n  bat { lvl: 2 }\n  cat {}\n"))
	files := srcLaw("package p\n\nrecord M {\n  hp: Int\n}\n\n@files(\"m/{id}.canon\")\nlet ms: table M = {}\n",
		"p/m/wolf.canon", "package p\n\nentry ms.wolf { hp: 1 }\n", "p/m/bat.canon", "package p\n\nentry ms.bat {}\n")
	for _, c := range []srcCase{
		{name: "E3 existing key", law: law, edit: allow(canon.AddEntry("p:ms", canon.Key("cat"), canon.Obj{"hp": canon.Int(1)})), want: canon.ErrKeyExists},
		{name: "E3 not a word", law: law, edit: allow(canon.AddEntry("p:ms", canon.Key("a b"), canon.Obj{"hp": canon.Int(1)})), want: canon.ErrBadValue},
		{name: "E2 Reset of a required field", law: law, edit: allow(canon.Reset("p:ms.bat.hp")), want: canon.ErrBadOp},
		{name: "E9 Move past the end", law: law, edit: allow(canon.Move("p:ms.bat", 7)), want: canon.ErrNoPath},
		{name: "E9 Move in files", law: files, edit: allow(canon.Move("p:ms.bat", 1)), want: canon.ErrNotEditable},
		{name: "V1 wrong type", law: law, edit: allow(canon.Set("p:ms.bat.hp", canon.Str("x"))), want: canon.ErrBadValue},
		{name: "E8 Reset of an absent field", law: law, edit: allow(canon.Reset("p:ms.cat.lvl")), undo: []canon.Op{}},
	} {
		c.file = srcMain
		runSource(t, "API.md "+strings.Fields(c.name)[0], c)
	}
	p, m := openEdit(t, law)
	if _, err := p.Edit(context.Background(), allow(canon.Set("p:ms.bat.hp", canon.Int(3)))); err != nil || !strings.Contains(read(t, m, srcMain), "bat { hp: 3, lvl: 2 }") {
		t.Errorf("API.md W7: %v\n%s", err, read(t, m, srcMain))
	}
}

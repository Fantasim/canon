package canon_test

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// dirRecord is package d up to its let: a record T, whose v is required, w not; dirCall the load.dir
// reading d/ts; dirHead package d, whose table of T is read by it.
const (
	dirRecord = "/// D.\npackage d\n\n/// T.\nrecord T {\n  /// V.\n  v: Int\n  /// W.\n  w: Int = 2\n}\n\n/// Ts.\n"
	dirCall   = "load.dir(\"ts/*.json\")\n"
	dirHead   = dirRecord + "let ts: table T = " + dirCall
)

// dirGood is d/ts/a.json, an entry that decodes; dirSet is an entry file whose v is 5.
const (
	dirGood = "{\n  \"v\": 1\n}\n"
	dirSet  = "{\n  \"v\": 5\n}\n"
)

// dirBroken are the contents of d/ts/b.json that leave p:ts without a value (R6): a file that does
// not parse, one whose v is no Int, one repeating a key (E7104), one missing v (E3302).
var dirBroken = []struct{ name, content string }{
	{"unparsable", "{\n  \"v\": \n"},
	{"wrongtype", "{\n  \"v\": \"x\"\n}\n"},
	{"dupkey", "{\n  \"v\": 2,\n  \"v\": 3\n}\n"},
	{"missing", "{}\n"},
}

// dirLaw is package d with b.json holding broken.
func dirLaw(broken string) map[string]string {
	return map[string]string{"d/d.canon": dirHead, "d/ts/a.json": dirGood, "d/ts/b.json": broken}
}

// sameFiles fails the test unless every file of want is on m as want holds it, lockNone for absent.
func sameFiles(t *testing.T, why string, m *memFS, want map[string]string) {
	t.Helper()
	for _, rel := range slices.Sorted(maps.Keys(want)) {
		if got := lockOf(t, m, rel); got != want[rel] {
			t.Errorf("%s: %s is %q, want %q", why, rel, got, want[rel])
		}
	}
}

// sameUndo fails the test unless undo is want, compared in their JSON form (API.md §8.8).
func sameUndo(t *testing.T, why string, undo, want []canon.Op) {
	t.Helper()
	got, err := json.Marshal(undo)
	exp, err2 := json.Marshal(want)
	if err != nil || err2 != nil || string(got) != string(exp) {
		t.Errorf("%s: Undo %s, want %s (%v, %v)", why, got, exp, err, err2)
	}
}

// undone applies res's Undo with AllowErrors, as the poisoned state it may restore needs (E19).
func undone(t *testing.T, why string, p *canon.Project, res *canon.EditResult) {
	t.Helper()
	if len(res.Undo) == 0 {
		return
	}
	if _, err := p.Edit(context.Background(), canon.Edit{Base: res.Revision, AllowErrors: true, Ops: res.Undo}); err != nil {
		t.Errorf("%s: the Undo: %v", why, err)
	}
}

// API.md E19, N6, E22; DECISIONS 309, 485: a Remove of the broken load.dir entry deletes its file,
// whatever it holds, without AllowErrors, and the table reads again; its Undo adds back an entry
// the file held as a value (V3), and none for bytes that hold no value.
func TestEditPoisonedDirRemoveBroken(t *testing.T) {
	for _, c := range dirBroken {
		p, m := openEdit(t, dirLaw(c.content))
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Remove("d:ts.b")}})
		if err != nil || !res.Applied || len(res.Findings) != 0 {
			t.Errorf("API.md E19, %s: Remove: %+v, %v", c.name, res, err)
			continue
		}
		sameFiles(t, "API.md N6, "+c.name, m, map[string]string{"d/ts/a.json": dirGood, "d/ts/b.json": lockNone})
		readable(t, p, "DECISIONS 309, "+c.name, "d:ts.a.v", "1")
		want := []canon.Op{} // bytes that hold no value give no inverse
		if c.name == "missing" {
			want = []canon.Op{canon.AddEntry("d:ts", canon.Key("b"), canon.Source("{}"))}
		}
		sameUndo(t, "API.md E23, "+c.name, res.Undo, want)
		undone(t, "API.md E22, "+c.name, p, res)
		if c.name == "missing" {
			sameFiles(t, "API.md E22, "+c.name, m, dirLaw(c.content))
		}
	}
}

// dirSiblingOps are the edits of a sibling of the broken entry, and the Undo each gives.
var dirSiblingOps = []struct {
	name string
	op   canon.Op
	undo []canon.Op
}{
	{"Remove a", canon.Remove("d:ts.a"), []canon.Op{canon.AddEntry("d:ts", canon.Key("a"), canon.Source("{ v: 1 }"))}},
	{"Set a.v", canon.Set("d:ts.a.v", canon.Int(5)), []canon.Op{canon.Set("d:ts.a.v", canon.Source("1"))}},
	{"AddEntry c", canon.AddEntry("d:ts", canon.Key("c"), canon.Obj{"v": canon.Int(7)}), []canon.Op{canon.Remove("d:ts.c")}},
}

// API.md E19, E18, E22, E23, N4, N6; DECISIONS 309: an edit of a sibling of the broken entry, or a
// new entry, is refused for the finding the re-check keeps, nothing written, and written with
// AllowErrors; its Undo gives every file back.
func TestEditPoisonedDirSibling(t *testing.T) {
	for _, c := range dirBroken {
		for _, s := range dirSiblingOps {
			why := c.name + ", " + s.name
			p, m := openEdit(t, dirLaw(c.content))
			res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{s.op}})
			if _, ok := isErr[*canon.RejectedError](err, canon.ErrRejected); !ok || res == nil || res.Applied {
				t.Errorf("API.md E19, %s: %+v, %v, want a *RejectedError", why, res, err)
			}
			law := dirLaw(c.content)
			law["d/ts/c.json"] = lockNone
			sameFiles(t, "API.md E19, "+why, m, law)
			res, err = p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{s.op}})
			if err != nil || !res.Applied {
				t.Errorf("API.md E19, %s, AllowErrors: %+v, %v", why, res, err)
				continue
			}
			sameUndo(t, "API.md E23, "+why, res.Undo, s.undo)
			undone(t, "API.md E22, "+why, p, res)
			sameFiles(t, "API.md E22, "+why, m, law)
		}
	}
}

// API.md E19, E21, E23; DECISIONS 309, 485: a Set of the broken entry's field rewrites a file that
// parses, its Undo setting a whole old entry back (none for a value of another type); a file that
// does not parse, a key repeated included, is ErrNoValue, nothing written.
func TestEditPoisonedDirSetBroken(t *testing.T) {
	for _, c := range dirBroken {
		p, m := openEdit(t, dirLaw(c.content))
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("d:ts.b.v", canon.Int(5))}})
		if c.name == "unparsable" || c.name == "dupkey" {
			if _, ok := isErr[*canon.PathError](err, canon.ErrNoValue); !ok || res != nil {
				t.Errorf("DECISIONS 485, %s: %+v, %v, want ErrNoValue", c.name, res, err)
			}
			sameFiles(t, "DECISIONS 485, "+c.name, m, dirLaw(c.content))
			continue
		}
		if err != nil || !res.Applied || len(res.Findings) != 0 {
			t.Errorf("API.md E19, %s: %+v, %v", c.name, res, err)
			continue
		}
		sameFiles(t, "DECISIONS 309, "+c.name, m, dirLaw(dirSet))
		readable(t, p, "DECISIONS 309, "+c.name, "d:ts.b.v", "5")
		want := []canon.Op{} // bytes that hold no value give no inverse
		if c.name == "missing" {
			want = []canon.Op{canon.Set("d:ts.b", canon.Source("{}"))}
		}
		sameUndo(t, "API.md E23, "+c.name, res.Undo, want)
		undone(t, "API.md E22, "+c.name, p, res)
		if c.name == "missing" {
			sameFiles(t, "API.md E22, "+c.name, m, dirLaw(c.content))
		}
	}
}

// API.md E4, E20, E23, E19: on a stable load.dir table without a value, a Remove of an id the lock
// holds is ErrStableKey; a Set of the missing field of a pending entry locks its id, and its Undo
// sets the old entry back, the id staying locked.
func TestEditPoisonedDirStable(t *testing.T) {
	head := "# canon.lock v1\ntable  d.ts  a\n"
	law := dirLaw("{}\n")
	law["d/d.canon"] = dirRecord + "let ts: stable table T = " + dirCall
	law["d/canon.lock"] = head + "table  d.ts  b\n"
	p, _ := openEdit(t, law)
	if _, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Remove("d:ts.b")}}); !errors.Is(err, canon.ErrStableKey) {
		t.Errorf("API.md E4: Remove of a held id: %v, want ErrStableKey", err)
	}
	law["d/canon.lock"] = head
	p, m := openEdit(t, law)
	res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("d:ts.b.v", canon.Int(5))}})
	if err != nil || !res.Applied {
		t.Fatalf("API.md E19: Set: %+v, %v", res, err)
	}
	locked := head + "table  d.ts  b\n"
	if got := lockOf(t, m, "d/canon.lock"); got != locked {
		t.Errorf("API.md E20: lock %q, want %q", got, locked)
	}
	sameUndo(t, "API.md E23", res.Undo, []canon.Op{canon.Set("d:ts.b", canon.Source("{}"))})
	undone(t, "API.md E22", p, res)
	law["d/canon.lock"] = locked
	sameFiles(t, "API.md E22, E23", m, law)
}

// API.md R6, E21; DECISIONS 485: a load of one JSON file that does not parse leaves its table
// without a value, and an edit of an entry in it is that refusal, ErrNoValue; the file is kept.
func TestEditPoisonedLoadFile(t *testing.T) {
	broken := "{\n  \"a\": {\n    \"v\": 1\n  },\n  \"b\": {\n    \"v\": \n"
	law := map[string]string{"d/d.canon": dirRecord + "let ts: table T = load(\"ts.json\")\n", "d/ts.json": broken}
	for _, op := range []canon.Op{canon.Remove("d:ts.b"), canon.Remove("d:ts.a"), canon.Set("d:ts.b.v", canon.Int(5)), canon.AddEntry("d:ts", canon.Key("c"), canon.Obj{"v": canon.Int(7)})} {
		p, m := openEdit(t, law)
		res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{op}})
		if _, ok := isErr[*canon.PathError](err, canon.ErrNoValue); !ok || res != nil {
			t.Errorf("DECISIONS 485, %s %s: %+v, %v, want ErrNoValue", op.Kind, op.Path, res, err)
		}
		sameFiles(t, "DECISIONS 485", m, law)
	}
}

// dirWide is d/ts/a.json with w written.
const dirWide = "{\n  \"v\": 1,\n  \"w\": 3\n}\n"

// API.md E19, E2, E5, E8, E22, E23; DECISIONS 309: a Set of a whole entry and a Reset of a field
// are written from the files too; each Undo sets the old entry or member back.
func TestEditPoisonedDirEntryOps(t *testing.T) {
	cases := []struct {
		name  string
		op    canon.Op
		after string
		undo  []canon.Op
	}{
		{"Set a", canon.Set("d:ts.a", canon.Obj{"v": canon.Int(4)}), "{\n  \"v\": 4\n}\n", []canon.Op{canon.Set("d:ts.a", canon.Source("{ v: 1, w: 3 }"))}},
		{"Reset a.w", canon.Reset("d:ts.a.w"), dirGood, []canon.Op{canon.Set("d:ts.a.w", canon.Source("3"))}},
	}
	for _, c := range cases {
		law := dirLaw(dirBroken[0].content)
		law["d/ts/a.json"] = dirWide
		p, m := openEdit(t, law)
		res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{c.op}})
		if err != nil || !res.Applied {
			t.Errorf("API.md E19, %s: %+v, %v", c.name, res, err)
			continue
		}
		if got := lockOf(t, m, "d/ts/a.json"); got != c.after {
			t.Errorf("API.md E19, %s: a.json %q, want %q", c.name, got, c.after)
		}
		sameUndo(t, "API.md E23, "+c.name, res.Undo, c.undo)
		undone(t, "API.md E22, "+c.name, p, res)
		sameFiles(t, "API.md E22, "+c.name, m, law)
	}
	p, m := openEdit(t, dirLaw(dirBroken[1].content))
	res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("d:ts.b", canon.Obj{"v": canon.Int(5)})}})
	if err != nil || !res.Applied || len(res.Undo) != 0 {
		t.Fatalf("DECISIONS 309: Set of the broken entry: %+v, %v", res, err)
	}
	sameFiles(t, "DECISIONS 309", m, dirLaw(dirSet))
}

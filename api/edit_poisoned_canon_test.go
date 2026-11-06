package canon_test

import (
	"context"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// canonBroken is package d: a table of T whose entry b states v as a string, so the table has no
// value (R6); n is a let an entry may read, which makes its text computed.
const canonBroken = dirRecord + "/// N.\nlet n: Int = 1\n\n/// Ts.\nlet ts: table T = {\n  a { v: n }\n  b { v: \"x\" }\n}\n"

// API.md E19, E22; DECISIONS 273, 309: on a .canon literal table without a value, a Remove or a Set
// of the entry whose old text types as no value applies, and its Undo leaves that inverse out.
func TestEditPoisonedCanonWrongType(t *testing.T) {
	cases := []struct {
		name  string
		op    canon.Op
		after string
	}{
		{"Remove b", canon.Remove("d:ts.b"), "  a { v: n }\n}\n"},
		{"Set b.v", canon.Set("d:ts.b.v", canon.Int(5)), "  a { v: n }\n  b { v: 5 }\n}\n"},
		{"Set b", canon.Set("d:ts.b", canon.Obj{"v": canon.Int(5)}), "  a { v: n }\n  b { v: 5 }\n}\n"},
	}
	for _, c := range cases {
		p, m := openEdit(t, map[string]string{"d/d.canon": canonBroken})
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{c.op}})
		if err != nil || !res.Applied || len(res.Findings) != 0 {
			t.Errorf("DECISIONS 309, %s: %+v, %v", c.name, res, err)
			continue
		}
		want := canonBroken[:len(canonBroken)-len("  a { v: n }\n  b { v: \"x\" }\n}\n")] + c.after
		sameFiles(t, "API.md E19, "+c.name, m, map[string]string{"d/d.canon": want})
		sameUndo(t, "API.md E22, DECISIONS 273, "+c.name, res.Undo, []canon.Op{})
		readable(t, p, "DECISIONS 309, "+c.name, "d:ts.a.v", "1")
	}
}

// API.md E19: an edit whose inverse would carry computed old text stays refused with the root's
// error, ErrNoValue, nothing written.
func TestEditPoisonedCanonComputed(t *testing.T) {
	for _, op := range []canon.Op{canon.Remove("d:ts.a"), canon.Set("d:ts.a.v", canon.Int(5)), canon.Set("d:ts.a", canon.Obj{"v": canon.Int(5)})} {
		p, m := openEdit(t, map[string]string{"d/d.canon": canonBroken})
		res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{op}})
		if _, ok := isErr[*canon.PathError](err, canon.ErrNoValue); !ok || res != nil {
			t.Errorf("API.md E19, %s %s: %+v, %v, want ErrNoValue", op.Kind, op.Path, res, err)
		}
		sameFiles(t, "API.md E19", m, map[string]string{"d/d.canon": canonBroken})
	}
}

// API.md E19, E22; DECISIONS 273, 480: an entry no error finding lies in keeps its value, so when
// Source cannot read its text back (a record typed through an import alias), its Remove is refused
// with the root's error rather than applied with an Undo that loses it.
func TestEditPoisonedCanonUnreadable(t *testing.T) {
	law := map[string]string{
		"e/e.canon": "/// E.\npackage e\n\n/// R.\nrecord R {\n  /// X.\n  x: Int\n}\n",
		"d/d.canon": "/// D.\npackage d\n\nimport e as q\n\n/// T.\nrecord T {\n  /// V.\n  v: Int\n  /// R.\n  r: q.R\n}\n\n/// Ts.\n" +
			"let ts: table T = {\n  a { v: 1, r: q.R { x: 1 } }\n  b { v: \"x\", r: q.R { x: 2 } }\n}\n",
	}
	p, m := openEdit(t, law)
	res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{canon.Remove("d:ts.a")}})
	if _, ok := isErr[*canon.PathError](err, canon.ErrNoValue); !ok || res != nil {
		t.Errorf("API.md E19: Remove a: %+v, %v, want ErrNoValue", res, err)
	}
	sameFiles(t, "API.md E19", m, law)
}

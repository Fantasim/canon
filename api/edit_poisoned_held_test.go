package canon_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// heldLaw is package d with a table of T whose v is refined, entry a written as aText, b as
// bText; package e's record R is imported as q.
func heldLaw(aText, bText string) map[string]string {
	return map[string]string{
		"e/e.canon": "/// E.\npackage e\n\n/// R.\nrecord R {\n  /// X.\n  x: Int\n}\n",
		"d/d.canon": "/// D.\npackage d\n\nimport e as q\n\n/// T.\nrecord T {\n  /// V.\n  v: Int(1..=100)\n  /// R.\n  r: q.R?\n}\n\n/// Ts.\n" +
			"let ts: table T = {\n  a " + aText + "\n  b " + bText + "\n}\n",
	}
}

// heldMapLaw is heldLaw with T holding a map keyed by a union of an enum and a string literal,
// whose keys may encode alike (WIRE.md 5.8).
func heldMapLaw(aText, bText string) map[string]string {
	law := heldLaw(aText, bText)
	law["d/d.canon"] = strings.Replace(law["d/d.canon"], "/// T.\nrecord T {\n",
		"/// Z.\nenum ZzE { default, other }\n\n/// K.\ntype K = ZzE | \"default\"\n\n/// T.\nrecord T {\n  /// M.\n  m: {K: Int} = {}\n", 1)
	return law
}

// API.md E19, E22, E23; EVALUATION.md 7.1; DECISIONS 236, 273, 309: a kept value (soft, keys alike)
// is carried when Source reads it back, else refused with the root's error; a value left none (a
// type mismatch, an unknown field) is dropped from the Undo.
func TestEditPoisonedHeldInError(t *testing.T) {
	broken := "{ v: \"x\" }"
	cases := []struct {
		name string
		law  map[string]string
		path string
		code diag.Code  // the finding the fixture holds in the removed entry
		undo []canon.Op // nil: refused with ErrNoValue
	}{
		{"soft, unreadable", heldLaw("{ v: 200, r: q.R { x: 1 } }", broken), "d:ts.a", diag.E3204.Def().Code, nil},
		{"soft, carried", heldLaw("{ v: 200 }", broken), "d:ts.a", diag.E3204.Def().Code, []canon.Op{canon.AddEntry("d:ts", canon.Key("a"), canon.Source("{ v: 200 }")), canon.Move("d:ts.a", 0)}},
		{"mismatch, dropped", heldLaw("{ v: 1 }", broken), "d:ts.b", diag.E3002.Def().Code, []canon.Op{}},
		{"keys alike, unreadable", heldMapLaw("{ v: 1, r: q.R { x: 1 }, m: { \"default\": 1, default: 2 } }", broken), "d:ts.a", diag.E3317.Def().Code, nil},
		{"unknown field, dropped", heldLaw("{ v: 1, zz: 2 }", broken), "d:ts.a", diag.E3301.Def().Code, []canon.Op{}},
	}
	for _, c := range cases {
		p, m := openEdit(t, c.law)
		f := findingsOf(t, p, "d")
		if !containsCode(f, diag.E3002.Def().Code) || !containsCode(f, c.code) {
			t.Fatalf("%s: the fixture lacks its findings: %+v", c.name, f)
		}
		res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{canon.Remove(c.path)}})
		if c.undo == nil {
			if _, ok := isErr[*canon.PathError](err, canon.ErrNoValue); !ok || res != nil {
				t.Errorf("API.md E19, %s: %+v, %v, want ErrNoValue", c.name, res, err)
			}
			sameFiles(t, "API.md E19, "+c.name, m, c.law)
			continue
		}
		if err != nil || !res.Applied {
			t.Errorf("API.md E19, %s: %+v, %v", c.name, res, err)
			continue
		}
		sameUndo(t, "API.md E23, "+c.name, res.Undo, c.undo)
	}
}

// findingsOf is pkg's check findings.
func findingsOf(t *testing.T, p *canon.Project, pkg string) []canon.Finding {
	t.Helper()
	res, err := p.Check(context.Background(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	return res.Findings
}

// containsCode reports a finding of code among fs.
func containsCode(fs []canon.Finding, code diag.Code) bool {
	for _, f := range fs {
		if f.Code == string(code) {
			return true
		}
	}
	return false
}

// API.md E10, E4, E19, M9; DECISIONS 485: a Retire on a stable load.dir table without a value
// writes `"$retired": true` as the entry object's first key and nothing else.
func TestEditPoisonedDirRetire(t *testing.T) {
	law := dirLaw(dirBroken[1].content)
	law["d/d.canon"] = dirRecord + "let ts: stable table T = " + dirCall
	law["d/ts/a.json"] = dirWide
	law["d/ts/c.json"] = "{}\n"
	law["d/ts/r.json"] = dirRetired
	cases := []struct {
		path, file, after string
	}{
		{"d:ts.a", "d/ts/a.json", "{\n  \"$retired\": true,\n  \"v\": 1,\n  \"w\": 3\n}\n"},
		{"d:ts.c", "d/ts/c.json", "{\n  \"$retired\": true\n}\n"},
		{"d:ts.r", "d/ts/r.json", dirRetired},
	}
	for _, c := range cases {
		p, m := openEdit(t, law)
		res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{canon.Retire(c.path)}})
		if err != nil || res == nil {
			t.Errorf("API.md E10, %s: %+v, %v", c.path, res, err)
			continue
		}
		sameFiles(t, "API.md E10, "+c.path, m, withFile(law, c.file, c.after))
	}
	if _, err := editOne(t, dirLaw(dirBroken[1].content), canon.Retire("d:ts.a")); !errors.Is(err, canon.ErrBadOp) {
		t.Errorf("API.md E2: Retire on a table that is not stable: %v, want ErrBadOp", err)
	}
	if _, err := editOne(t, withFile(law, "d/ts/b.json", dirBroken[0].content), canon.Retire("d:ts.b")); !errors.Is(err, canon.ErrNoValue) {
		t.Errorf("DECISIONS 485: Retire in a file that does not parse: %v, want ErrNoValue", err)
	}
	if _, err := editOne(t, withFile(law, "d/ts/a.json", "{\"v\": 1}\n"), canon.Retire("d:ts.a")); !errors.Is(err, canon.ErrNotCanonical) {
		t.Errorf("API.md M9: Retire in a file not in canonical layout: %v, want ErrNotCanonical", err)
	}
}

// withFile is law with file rel holding content.
func withFile(law map[string]string, rel, content string) map[string]string {
	out := map[string]string{rel: content}
	for k, v := range law { //canon:unordered fills a map
		if k != rel {
			out[k] = v
		}
	}
	return out
}

// editOne applies op with AllowErrors to a project opened with law; a refused op writes nothing.
func editOne(t *testing.T, law map[string]string, op canon.Op) (*canon.EditResult, error) {
	t.Helper()
	p, m := openEdit(t, law)
	res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{op}})
	if err != nil {
		sameFiles(t, "a refused edit", m, law)
	}
	return res, err
}

package canon_test

import (
	"context"
	"errors"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// syntaxHead is package d's first lines: a record and a value whose port an edit sets.
const syntaxHead = "/// D.\npackage d\n\n/// Settings.\nrecord Config {\n  /// A port.\n  port: Int = 8765\n}\n\n/// The settings.\nlet config: Config = {}\n"

// syntaxBad is a declaration that does not parse, without a closing expression.
const syntaxBad = "\n/// Broken.\nlet bad: Int =\n"

// syntaxSet is the edit of d's port.
func syntaxSet() canon.Edit {
	return canon.Edit{Ops: []canon.Op{canon.Set("d:config.port", canon.Int(9000))}}
}

// API.md E21, M9: a file the edit would write that does not parse is a refusal of the layout
// check, a *SyntaxError naming the file with the parser's findings, never an *InternalError;
// nothing is written, with or without AllowErrors or Normalize.
func TestEditWriteIntoBrokenFile(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(canon.Edit) canon.Edit
	}{
		{"plain", func(e canon.Edit) canon.Edit { return e }},
		{"AllowErrors", func(e canon.Edit) canon.Edit { e.AllowErrors = true; return e }},
		{"Normalize", func(e canon.Edit) canon.Edit { e.Normalize = true; return e }},
		{"DryRun", func(e canon.Edit) canon.Edit { e.DryRun = true; return e }},
	} {
		p, m := openEdit(t, map[string]string{"d/d.canon": syntaxHead + syntaxBad})
		before := read(t, m, "d/d.canon")
		res, err := p.Edit(context.Background(), c.edit(syntaxSet()))
		var ie *canon.InternalError
		se, ok := isErr[*canon.SyntaxError](err, canon.ErrSyntax)
		switch {
		case errors.As(err, &ie):
			t.Fatalf("%s: API.md E21: an internal error: %v", c.name, err)
		case !ok || res != nil || len(se.Findings) == 0 || se.Findings[0].File != "d/d.canon":
			t.Fatalf("%s: API.md M9: %+v, %v", c.name, res, err)
		case se.Findings[0].Code != string(diag.E1116.Def().Code):
			t.Errorf("%s: API.md M9: the parser's finding: %+v", c.name, se.Findings[0])
		}
		if read(t, m, "d/d.canon") != before {
			t.Errorf("%s: API.md M9: a refused edit wrote", c.name)
		}
	}
}

// API.md E19, E21: a file of the package that the edit does not write may hold a syntax error: the
// edit is checked against its findings like any other (E18), refused without AllowErrors, with
// the sibling's finding, and written, the sibling untouched, with it.
func TestEditBesideBrokenFile(t *testing.T) {
	files := map[string]string{"d/d.canon": syntaxHead, "d/z.canon": "/// D.\npackage d\n" + syntaxBad}
	p, m := openEdit(t, files)
	res, err := p.Edit(context.Background(), syntaxSet())
	re, ok := isErr[*canon.RejectedError](err, canon.ErrRejected)
	if !ok || res == nil || res.Applied || len(re.Findings) != 1 || re.Findings[0].Code != string(diag.E1116.Def().Code) || re.Findings[0].File != "d/z.canon" {
		t.Fatalf("API.md E19: %+v, %v", res, err)
	}
	if got := read(t, m, "d/d.canon"); got != syntaxHead {
		t.Errorf("API.md E19: a rejected edit wrote:\n%s", got)
	}
	e := syntaxSet()
	e.AllowErrors = true
	res, err = p.Edit(context.Background(), e)
	if err != nil || !res.Applied || len(res.Changes) != 1 || res.Changes[0].Path != "d/d.canon" {
		t.Fatalf("API.md E19, AllowErrors: %+v, %v", res, err)
	}
	if got := lines(read(t, m, "d/d.canon"), "port: 9000"); len(got) != 1 {
		t.Errorf("API.md E19: AllowErrors did not write: %q", got)
	}
	if got := read(t, m, "d/z.canon"); got != "/// D.\npackage d\n"+syntaxBad {
		t.Errorf("API.md E1: the broken sibling changed:\n%s", got)
	}
}

// API.md R6, E21: a JSON source that does not parse leaves its value without one, and an edit of
// it is that refusal (ErrNoValue), never an *InternalError; the file is kept.
func TestEditBrokenJSON(t *testing.T) {
	files := map[string]string{
		"d/d.canon":  "/// D.\npackage d\n\n/// Names.\nlet names: {String: String} = load(\"etc.json\")\n",
		"d/etc.json": "{\n  \"A\": \"Ay\",\n",
	}
	p, m := openEdit(t, files)
	_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("d:names", canon.FromJSON([]byte(`{"A": "Bee"}`)))}})
	_, ok := isErr[*canon.PathError](err, canon.ErrNoValue)
	if !ok {
		t.Fatalf("API.md R6: %v", err)
	}
	if got := read(t, m, "d/etc.json"); got != files["d/etc.json"] {
		t.Errorf("API.md E21: a refused edit wrote:\n%s", got)
	}
}

package canon_test

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

const rulesProject = "project a {\n  canon: \"0.1\"\n}\n"

// oneCheck is the Check of package a of files, whose project.canon is rulesProject.
func oneCheck(t *testing.T, source string) (*canon.CheckResult, *canon.Project) {
	t.Helper()
	p, _ := openLaw(t, map[string]string{"project.canon": rulesProject, "a/a.canon": source})
	res, err := p.Check(context.Background(), "a")
	if err != nil {
		t.Fatal(err)
	}
	return res, p
}

// rendered is the template of the variant of def that has that name, with its arguments given as
// name and text pairs; a variant with no name is the code's only one.
func rendered(t *testing.T, def *diag.Def, variant string, pairs ...string) string {
	t.Helper()
	for _, v := range def.Variants {
		if v.Name == variant {
			return strings.NewReplacer(pairs...).Replace(v.Template)
		}
	}
	t.Fatalf("%s has no variant %q", def.Code, variant)
	return ""
}

// API.md F3: a code with arguments has the template of its registry entry, rendered with them.
func TestFindingMessageRendersArguments(t *testing.T) {
	res, _ := oneCheck(t, "/// A.\npackage a\n\n/// Limit.\nlet limit: Int = \"not an int\"\n")
	def := diag.E3002.Def()
	want := rendered(t, def, "", "{expected}", "Int", "{found}", "String")
	for _, f := range res.Findings {
		if f.Code == string(def.Code) {
			if f.Message != want {
				t.Errorf("message %q, want %q", f.Message, want)
			}
			return
		}
	}
	t.Fatalf("no %s finding: %+v", def.Code, res.Findings)
}

const blockChecks = `/// A.
package a

/// A status.
record Status {
  /// Its label.
  label: String

  check {
    if label.len() > 8 {
      warn(label, "label {label} is long")
    }
  }
}

/// The statuses.
let statuses: table Status = {
  open { label: "Open" }
  verified { label: "Verified and closed" }
}

check {
  for s in statuses {
    if s.label == "Open" {
      fail(s, "status {s.label} is not claimed")
    }
  }
}
`

// API.md F3: the block forms are E5002 and W5002, whose message is the text the check gave.
func TestBlockCheckCodes(t *testing.T) {
	res, _ := oneCheck(t, blockChecks)
	want := map[string]string{
		string(diag.E5002.Def().Code): rendered(t, diag.E5002.Def(), "", "{message}", "status Open is not claimed"),
		string(diag.W5002.Def().Code): rendered(t, diag.W5002.Def(), "", "{message}", "label Verified and closed is long"),
	}
	for _, f := range res.Findings {
		text, ok := want[f.Code]
		if !ok {
			t.Errorf("unexpected finding %s %q", f.Code, f.Message)
			continue
		}
		if f.Message != text || len(f.Related) != 1 || f.Related[0].Note != "check" {
			t.Errorf("%s: message %q, related %+v, want %q", f.Code, f.Message, f.Related, text)
		}
		delete(want, f.Code)
	}
	if len(want) != 0 {
		t.Errorf("missing findings: %v (got %+v)", want, res.Findings)
	}
}

// API.md F12: a related location is `  expected by <file>:<line>`, then ` (<note>)` only when the
// note is not empty; one without a file writes `  expected by (<note>)`.
func TestRelatedTextForms(t *testing.T) {
	at := canon.Span{File: "a/a.canon", Line: 3, Col: 1, EndLine: 3, EndCol: 5}
	related := []canon.Related{
		{Span: canon.Span{File: "a/a.canon", Line: 7, Col: 3, EndLine: 7, EndCol: 9}, Note: "check short"},
		{Span: canon.Span{File: "a/a.canon", Line: 8, Col: 3, EndLine: 8, EndCol: 9}},
		{Note: "no file"},
	}
	f := canon.Finding{
		Severity: canon.SeverityError, Code: string(diag.E5001.Def().Code), Span: at, Package: "a",
		Message: "m", Related: related,
	}
	var buf bytes.Buffer
	if err := canon.WriteFindings(&buf, []canon.Finding{f}, canon.WriteOptions{Summary: canon.Summary{Errors: 1, Packages: 1}, Golden: true}); err != nil {
		t.Fatal(err)
	}
	want := "error[" + f.Code + "]  a/a.canon:3:1\n  m\n" +
		"  expected by a/a.canon:7 (check short)\n" +
		"  expected by a/a.canon:8\n" +
		"  expected by (no file)\n\n" +
		"1 error, 0 warnings in 1 package (…)\n"
	if buf.String() != want {
		t.Errorf("text form:\n%s\nwant:\n%s", buf.String(), want)
	}
}

// API.md T2: a JSON source that is not in the canonical layout comes back in it, key order and
// unknown numbers kept as written.
func TestFormatJSONSourceLayout(t *testing.T) {
	in := "\r\n{\"z\":1.50,  \"a\":{\"y\":[1,2,{}],\"b\":{}},\"e\":[ ],\"s\":\"x\\u0041\"}"
	want := "{\n  \"z\": 1.50,\n  \"a\": {\n    \"y\": [\n      1,\n      2,\n      {}\n    ],\n    \"b\": {}\n  },\n" +
		"  \"e\": [],\n  \"s\": \"xA\"\n}\n"
	got, err := canon.FormatJSONSource([]byte(in))
	if err != nil || string(got) != want {
		t.Errorf("FormatJSONSource:\n%q\nwant:\n%q (%v)", got, want, err)
	}
}

// rootsLaw reads xs through the root data, which Options.Roots can point elsewhere.
var rootsLaw = map[string]string{
	"project.canon": "project a {\n  canon: \"0.1\"\n  roots {\n    data: \"data\"\n  }\n}\n",
	"a/a.canon":     "/// A.\npackage a\n\n/// Xs.\nlet xs: [Int] = load(\"@data/xs.json\")\n",
	"data/xs.json":  "[1, 2]\n",
	"alt/xs.json":   "[7]\n",
}

// API.md O7: Roots change the files read, so what the program computes and the revision (the
// hash of the read set, S3) change with them.
func TestRootsChangeTheProgram(t *testing.T) {
	values := map[string]string{}
	revisions := map[string]canon.Revision{}
	for name, roots := range map[string]map[string]string{"declared": nil, "redirected": {"data": "alt"}} { //canon:unordered each case fills its own entry
		opts := project(rootsLaw)
		opts.Roots = roots
		p, err := canon.Open("/law", opts)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = p.Close() })
		v, err := p.Value(context.Background(), "a:xs")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		values[name], revisions[name] = v.String(), p.Revision()
	}
	if values["declared"] != "[1, 2]" || values["redirected"] != "[7]" || revisions["declared"] == revisions["redirected"] {
		t.Errorf("values %v, revisions %v", values, revisions)
	}
}

const whenFails = `/// A.
package a

/// A box.
record Box {
  /// Its label.
  label: String
  /// Its size.
  n: Int
}

view Box {
  title "{label}"
  n { when: 1 / (n - n) > 0 }
}

/// The boxes.
let boxes: table Box = {
  one { label: "One", n: 3 }
}
`

// API.md V12: a `when` whose evaluation fails counts as true and produces no finding: the result
// holds none, and its summary is the check's.
func TestWhenFailureIsTrueAndSilent(t *testing.T) {
	checked, p := oneCheck(t, whenFails)
	if len(checked.Findings) != 0 {
		t.Fatalf("the fixture has findings: %+v", checked.Findings)
	}
	res := evaluate(t, p, "a:boxes.one", "")
	if shown, ok := res.When["n"]; !ok || !shown {
		t.Errorf("When = %v, want n shown", res.When)
	}
	if len(res.Findings) != 0 || !reflect.DeepEqual(res.Summary, checked.Summary) {
		t.Errorf("findings %+v, summary %+v; the check's summary is %+v", res.Findings, res.Summary, checked.Summary)
	}
}

const deprecatedLaw = `/// A.
package a

/// A gadget.
record Gadget {
  /// Its label.
  label: String
  /// The old size.
  size: Int = 0 @deprecated("use label")
}

/// The gadget.
let gadget: Gadget = { label: "g" }
`

// API.md W6: a @deprecated field is editable: an edit that sets one in a .canon source writes it
// and returns W3301 among its findings.
func TestSetDeprecatedFieldWarns(t *testing.T) {
	p, opts := openLaw(t, map[string]string{"project.canon": rulesProject, "a/a.canon": deprecatedLaw})
	res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("a:gadget.size", canon.Int(5))}})
	if err != nil || res == nil || !res.Applied {
		t.Fatalf("Edit: %+v, %v", res, err)
	}
	written, err := opts.FS.ReadFile("/law/a/a.canon")
	if err != nil || !strings.Contains(string(written), "gadget: Gadget = { label: \"g\", size: 5 }") {
		t.Errorf("the deprecated field is not written (%v):\n%s", err, written)
	}
	code := string(diag.W3301.Def().Code)
	message := rendered(t, diag.W3301.Def(), "reason", "{name}", "size", "{reason}", "use label")
	found := false
	for _, f := range res.Findings {
		found = found || (f.Code == code && f.Severity == canon.SeverityWarning && f.Message == message)
	}
	if !found || res.Summary.Errors != 0 {
		t.Errorf("findings %+v, summary %+v", res.Findings, res.Summary)
	}
}

// API.md V14: an edit written with AllowErrors is evaluated like any other: Edit.Evaluate returns
// the result at the new revision, and an Evaluate call after it reads the same snapshot.
func TestEvaluateAfterAllowErrors(t *testing.T) {
	p, _ := openEdit(t, nil)
	e := canon.Edit{
		Ops:         []canon.Op{canon.Set("a:statuses.open.weight", canon.Int(-1))},
		AllowErrors: true, Evaluate: []string{"a:statuses.open"},
	}
	res, err := p.Edit(context.Background(), e)
	if err != nil || !res.Applied || res.Summary.Errors != 1 {
		t.Fatalf("Edit: %+v, %v", res, err)
	}
	inline := res.Eval["a:statuses.open"]
	if inline == nil || inline.Revision != res.Revision || len(inline.Findings) != 1 {
		t.Fatalf("Edit.Evaluate: %+v", inline)
	}
	after := evaluate(t, p, "a:statuses.open", "")
	if after.Revision != res.Revision || len(after.Findings) != 1 || after.Findings[0].Code != inline.Findings[0].Code {
		t.Errorf("Evaluate after the edit: %+v, want the inline result %+v", after, inline)
	}
}

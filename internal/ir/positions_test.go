package ir_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// markLines reads marks.txt: `pkg.name text` lines.
func (w *world) markLines(data string) {
	w.marks = map[string][]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(data), "\n") {
		key, text, _ := strings.Cut(line, " ")
		w.marks[key] = append(w.marks[key], text)
	}
}

// mark gives the leaves of v named by marks[key] the position mark, keeping their provenance (EVALUATION.md §13.1).
func (w *world) mark(key string, v value.Value) {
	texts := w.marks[key]
	if len(texts) == 0 || v == nil {
		return
	}
	switch x := v.(type) {
	case *value.Int:
		x.P = w.marked(x.P, x.CanonText(), texts)
	case *value.Str:
		x.P = w.marked(x.P, x.CanonText(), texts)
	case *value.Float:
		x.P = w.marked(x.P, x.CanonText(), texts)
	case *value.Record:
		for _, f := range x.Fields {
			w.mark(key, f)
		}
	case *value.List:
		for _, e := range x.Elems {
			w.mark(key, e)
		}
	case *value.Map:
		for i := range x.Keys {
			w.mark(key, x.Keys[i])
			w.mark(key, x.Vals[i])
		}
	case *value.Table:
		for _, e := range x.Entries {
			w.mark(key, e)
		}
	}
}

// marked is p with the mark when text is one of texts; a world with a site gives it that site, the zero span being a value made at no source position.
func (w *world) marked(p *value.Prov, text string, texts []string) *value.Prov {
	for _, t := range texts {
		if t != text {
			continue
		}
		twin := value.Prov{}
		if p != nil {
			twin = *p
		}
		twin.Position = true
		if w.site != nil {
			twin.Span = *w.site
		}
		return &twin
	}
	return p
}

const rowsFile = "a.rows.json"

var w8031Code = string(diag.W8031.Def().Code)

// foldRecord folds a brace literal of a record type whose every field is written and foldable, in declaration order.
func foldRecord(x *syntax.BraceLit, info *check.Info) (value.Value, bool) {
	t := info.Types[x]
	if _, ok := t.Base().(*types.RecordType); !ok {
		return nil, false
	}
	written := map[string]value.Value{}
	for _, it := range x.Items {
		f, isField := it.(*syntax.FieldItem)
		if !isField {
			return nil, false
		}
		v, ok := fold(f.Value, info)
		if !ok {
			return nil, false
		}
		written[f.Name.Name] = v
	}
	r := &value.Record{T: t}
	for _, f := range verify.Fields(t) {
		v, ok := written[f.Name]
		if !ok {
			return nil, false
		}
		r.Fields = append(r.Fields, v)
		r.Set = append(r.Set, true)
	}
	return r, true
}

const w8031Program = `package a

/// The numbers.
let rows: [Int] = []

emit json { out: "out/json/" }
`

// buildPositions builds package a from src with the marks lines and the (name, content) fixture files of extra, and renders its findings.
func buildPositions(t *testing.T, src, marks string, site *source.Span, extra ...string) string {
	t.Helper()
	w := newWorld(t)
	w.site = site
	w.add(t, "a/a.canon", []byte(src))
	for i := 0; i+1 < len(extra); i += 2 {
		w.add(t, extra[i], []byte(extra[i+1]))
	}
	if marks != "" {
		w.markLines(marks)
	}
	w.build(t)
	return w.findings(t)
}

// CODEGEN.md §2.10: an unmarked value, a marked one under a view-only emit and a marked `local let` (a check-only use) are not written to an output. The plain list, `positional` and `at`/`len` rows are unmarked values here: EVALUATION §13.1 carries them in eval, proved by its TestEntryPositions and TestPositionCarried.
func TestW8031Accept(t *testing.T) {
	for _, tc := range []struct{ name, src, marks string }{
		{"unmarked", w8031Program, ""},
		{"local let", strings.Replace(w8031Program, "let rows", "local let rows", 1), "a.rows 7"},
		{"view only", strings.Replace(w8031Program, "emit json { out: \"out/json/\" }", "emit view { out: \"out/view/\" }", 1), "a.rows 7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if out := buildPositions(t, tc.src, tc.marks, nil, rowsFile, "[7]"); strings.Contains(out, w8031Code) {
				t.Fatalf("%s reported:\n%s", w8031Code, out)
			}
		})
	}
}

// CODEGEN.md §2.10: a marked value is one finding, and a thousand marked leaves written at one site are one finding.
func TestW8031OncePerSite(t *testing.T) {
	thousand := "[" + strings.Repeat("7,", 999) + "7]"
	fixedSite := source.Span{File: 1, Start: 0, End: 1} // the project file's first byte: a site every leaf shares
	cases := []struct {
		name string
		rows string
		site *source.Span
		want int
	}{
		{"one", "[7]", nil, 1},
		{"fixed site", thousand, &fixedSite, 1},
		{"no site", thousand, &source.Span{}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := buildPositions(t, w8031Program, "a.rows 7", tc.site, rowsFile, tc.rows)
			if got := strings.Count(out, "warning["+w8031Code+"]"); got != tc.want {
				t.Fatalf("%d findings, want %d:\n%s", got, tc.want, out)
			}
		})
	}
}

const w8031OrderProgram = `package a

/// An item.
record Item {
  /// N.
  n: Int

  /// Its place.
  export fn place(self) -> Int { return 2 }
}

/// Another.
record Other {
  /// K.
  k: Int
}

/// The rows.
record Rows {
  /// An item.
  item: Item
  /// Another.
  other: Other
}

/// The rows.
let rows: Rows = { item: { n: 1 }, other: { k: 5 } }

emit json { out: "out/" }
`

// CODEGEN.md §2.10, precompute.go:69-81: a marked field and a marked method result met in one let share a site; the let's fields are walked before the queued method results, so the finding names the let, not the method.
func TestW8031FieldsBeforeMethodResults(t *testing.T) {
	site := source.Span{File: 1, Start: 0, End: 1}
	out := buildPositions(t, w8031OrderProgram, "a.rows 5\na.place 2", &site, rowsFile, `{ "item": { "n": 1 }, "other": { "k": 5 } }`, "a.place.calls.json", `{ "": 2 }`)
	if got := strings.Count(out, "warning["+w8031Code+"]"); got != 1 {
		t.Fatalf("%d findings, want 1:\n%s", got, out)
	}
	if !strings.Contains(out, "written to a.rows;") {
		t.Fatalf("the finding does not name the let a.rows:\n%s", out)
	}
}

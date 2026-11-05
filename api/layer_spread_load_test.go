package canon_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// The lets of spreadDepthSrc that write b1 and c1 as literals, and their record literals.
const (
	spreadDepthB1    = `let b1: B = { id: "b1", inner: { id: "i", oo: [{ name: "x" }, { name: "y" }] } }`
	spreadDepthC1    = `let c1: C = { id: "c1", b: { id: "b", inner: { id: "i", oo: [{ name: "x" }, { name: "y" }] } } }`
	spreadDepthB1Lit = `{ id: "b1", inner: { id: "i", oo: [{ name: "x" }, { name: "y" }] } }`
	spreadDepthC1Lit = `{ id: "c1", b: { id: "b", inner: { id: "i", oo: [{ name: "x" }, { name: "y" }] } } }`
)

// The JSON documents of b1 and c1, as spreadDepthSrc writes them.
const (
	spreadDepthB1JSON = `{"id": "b1", "inner": {"id": "i", "oo": [{"name": "x"}, {"name": "y"}]}}` + "\n"
	spreadDepthC1JSON = `{"id": "c1", "b": {"id": "b", "inner": {"id": "i", "oo": [{"name": "x"}, {"name": "y"}]}}}` + "\n"
)

// How spreadDepthVariant gives b1 and c1.
const (
	spreadByLoad    = iota // each read by load from its JSON file
	spreadByDir            // each an entry of a load.dir keyed list
	spreadByListLit        // each an entry of a literal keyed list: what spreadByDir decodes
)

// spreadDepthVariant is spreadDepthSrc with b1 and c1 given as how says, each line kept in place,
// and the files it reads.
func spreadDepthVariant(how int) (string, map[string]string) {
	b1, c1 := `let b1: B = load("b1.json")`, `let c1: C = load("c1.json")`
	files := map[string]string{"p/b1.json": spreadDepthB1JSON, "p/c1.json": spreadDepthC1JSON}
	bs, cs := `load.dir("bs/*.json")`, `load.dir("cs/*.json")`
	switch how {
	case spreadByDir:
		files = map[string]string{"p/bs/b1.json": spreadDepthB1JSON, "p/cs/c1.json": spreadDepthC1JSON}
	case spreadByListLit:
		files, bs, cs = nil, "["+spreadDepthB1Lit+"]", "["+spreadDepthC1Lit+"]"
	}
	tail := ""
	if how != spreadByLoad {
		b1, c1 = `let b1: B = bs.b1`, `let c1: C = cs.c1`
		tail = "\nlet bs: [B] keyed by id = " + bs + "\n\nlet cs: [C] keyed by id = " + cs + "\n"
	}
	src := strings.Replace(strings.Replace(spreadDepthSrc, spreadDepthB1, b1, 1), spreadDepthC1, c1, 1)
	return src + tail, files
}

// spreadDepthReads is each value spreadDepthSrc reads through a copy, and what it is when every
// copy holds its source's entries (DECISIONS 79, 478).
var spreadDepthReads = map[string]string{
	"p:s5": "true", "p:s3": "true", "p:sp": "true", "p:sh": "true",
	"p:w5": "0", "p:w3": "0", "p:wp": "0", "p:wh": "0",
	"p:kb5": "[true, true]", "p:kc5": "[true, true]", "p:kpi": "[true, true]", "p:khb": "[true, true]",
}

// errorsOf is the code, path and message of each of p's error findings.
func errorsOf(t *testing.T, p *canon.Project) []string {
	t.Helper()
	res, err := p.Check(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range res.Findings {
		if f.Severity == "error" {
			out = append(out, f.Code+" "+f.Path+" "+f.Message)
		}
	}
	return out
}

// DECISIONS 79, 315, 478: without a layer, every copy of a loaded b1 or c1 holds its entries and
// m's defaulted key x is bound to the decoded instance holding oo: no error, each read as for the
// literals, Graph and Refs theirs.
func TestLoadedCopyKeepsEntries(t *testing.T) {
	for _, c := range []struct {
		name     string
		how, lit int
	}{{"load", spreadByLoad, -1}, {"load.dir", spreadByDir, spreadByListLit}} {
		t.Run(c.name, func(t *testing.T) {
			want := spreadDepthSrc
			if c.lit >= 0 {
				want, _ = spreadDepthVariant(c.lit)
			}
			graph0, refs0 := spreadDepthView(t, openLayered(t, want, "", false))
			src, files := spreadDepthVariant(c.how)
			p := openLayeredWith(t, src, files, "", false)
			if errs := errorsOf(t, p); len(errs) > 0 {
				t.Errorf("EVALUATION.md §3.4, DECISIONS 315: errors\n%s", strings.Join(errs, "\n"))
			}
			graph1, refs1 := spreadDepthView(t, p)
			if !slices.Equal(graph0, graph1) || !slices.Equal(refs0, refs1) {
				t.Errorf("DECISIONS 478, 410: Graph and Refs =\n%s\n%s\nwant, as for the literals\n%s\n%s",
					strings.Join(graph1, "\n"), strings.Join(refs1, "\n"), strings.Join(graph0, "\n"), strings.Join(refs0, "\n"))
			}
			checkValues(t, p, "DECISIONS 79, 478", spreadDepthReads)
		})
	}
}

// DECISIONS 486, 492 over loaded sources: each layer line of TestLayerSpreadCopyAtDepth, over b1
// and c1 decoded from JSON, keeps or re-owns the copy's entries exactly as over the literals.
func TestLayerLoadedCopyAtDepth(t *testing.T) {
	for _, how := range []int{spreadByLoad, spreadByDir} {
		src, files := spreadDepthVariant(how)
		open := spreadDepthOpener(src, files)
		for _, c := range spreadDepthCases {
			name := []string{"load ", "load.dir "}[how] + c.root + " " + c.line
			t.Run(name, func(t *testing.T) { spreadDepthRow(t, c, open) })
		}
	}
}

// nestedDefaultSrc is p: q's default key x, in an element of i's qs, names an entry of the
// enclosing I's oo; i is written by w, a literal or a load of nestedDefaultJSON.
const nestedDefaultSrc = "package p\n\nrecord S {\n  name: String\n}\n\nrecord Q {\n  name: String\n  picks: {ref S: Int} = { x: 2 }\n}\n\n" +
	"record I {\n  oo: [S] keyed by name\n  qs: [Q] keyed by name\n}\n\nlet i: I = w\n\n" +
	"let held: Bool = i.qs.q.picks.keys().contains(i.oo.x)\n\nlet pw: Int = i.qs.q.picks[i.oo.x]\n"

// nestedDefaultJSON is i as a JSON document.
const nestedDefaultJSON = `{"oo": [{"name": "x"}], "qs": [{"name": "q"}]}` + "\n"

// EVALUATION.md §3.4: a nested decoded default's ref binds to the enclosing instance, as a literal's.
func TestLoadedNestedDefaultRefBound(t *testing.T) {
	for _, c := range []struct{ name, w string }{
		{"literal", `{ oo: [{ name: "x" }], qs: [{ name: "q" }] }`}, // regression guard: passes on the base
		{"load", `load("i.json")`},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := strings.Replace(nestedDefaultSrc, "= w\n", "= "+c.w+"\n", 1)
			p := openLayeredWith(t, src, map[string]string{"p/i.json": nestedDefaultJSON}, "", false)
			if errs := errorsOf(t, p); len(errs) > 0 {
				t.Errorf("EVALUATION.md §3.4: errors\n%s", strings.Join(errs, "\n"))
			}
			checkValues(t, p, "EVALUATION.md §3.4", map[string]string{"p:held": "true", "p:pw": "2"})
		})
	}
}

// dirDefaultSrc is p: each element of ii, one per file, has m's default key x, an entry of its oo.
const dirDefaultSrc = "package p\n\nrecord S {\n  name: String\n}\n\n" +
	"record I {\n  id: String\n  oo: [S] keyed by name\n  m: {ref S: Int} = { x: 1 }\n}\n\n" +
	"let ii: [I] keyed by id = load.dir(\"ii/*.json\")\n\n" +
	"let held: [Bool] = [ii.a.m.keys().contains(ii.a.oo.x), ii.b.m.keys().contains(ii.b.oo.x)]\n"

// dirDefaultJSON is the element of ii in file ii/<id>.json, its oo holding x and more.
func dirDefaultJSON(id, more string) string {
	return `{"id": "` + id + `", "oo": [{"name": "x"}, {"name": "` + more + `"}]}` + "\n"
}

// EVALUATION.md §3.4: a load.dir element's default key stays bound when a warm build serves it again.
func TestLoadedDirDefaultRefBoundWarm(t *testing.T) {
	files := map[string]string{"p/ii/a.json": dirDefaultJSON("a", "y"), "p/ii/b.json": dirDefaultJSON("b", "y")}
	p := openLayeredWith(t, dirDefaultSrc, files, "", false)
	// b edited: a is served, b decoded; a file the glob skips, put or edited: a and b are both served
	for _, step := range []struct{ name, file, data string }{
		{"cold", "", ""},
		{"b edited", "p/ii/b.json", dirDefaultJSON("b", "w")},
		{"note put", "p/ii/note.txt", "one\n"},
		{"note edited", "p/ii/note.txt", "two\n"},
	} {
		if step.file != "" {
			if err := p.SetOverlay(context.Background(), step.file, []byte(step.data)); err != nil {
				t.Fatal(err)
			}
		}
		if errs := errorsOf(t, p); len(errs) > 0 {
			t.Errorf("EVALUATION.md §3.4: %s: errors\n%s", step.name, strings.Join(errs, "\n"))
		}
		checkValues(t, p, "EVALUATION.md §3.4 "+step.name, map[string]string{"p:held": "[true, true]"})
	}
}

package canon_test

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// checkCallers is how many checks TestCheckLayerConcurrent runs at once.
const checkCallers = 8

// checkLayerFiles are allLayersOpen's two layers: lw gives qs[a] rewards, lf amends ps[one].
var checkLayerFiles = map[string]string{
	"d/lw.layer.canon": "package d\nlayer lw\n\namend qs {\n  a.rewards: [\"s\"]\n}\n",
	"d/lf.layer.canon": "package d\nlayer lf\n\namend ps {\n  one.v: 2\n}\n",
}

// checkLayersOpen opens package d as src with checkLayerFiles and more files, layers active,
// max findings kept per package (API.md F7), 0 the default.
func checkLayersOpen(t *testing.T, src string, layers []string, max int, more map[string]string) *canon.Project {
	t.Helper()
	files := map[string]string{"project.canon": editLaw["project.canon"], "d/d.canon": src}
	for _, m := range []map[string]string{checkLayerFiles, more} {
		//canon:unordered each file is stored under its own name
		for name, text := range m {
			files[name] = text
		}
	}
	opts := project(files)
	opts.Layers, opts.MaxFindings = layers, max
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// checkBrokenCases are bases a hand edit left breaking a layer: its finding exists only once the
// layer is evaluated.
var checkBrokenCases = []struct {
	name, layer, src string
	code             diag.Code
}{
	{"the driver no longer types the layer's rewards", "lw", strings.Replace(allLayersSrc, "a { items: true }", "a { items: false }", 1), diag.E3802.Def().Code},
	{"the entry the layer amends is gone", "lf", strings.Replace(allLayersSrc, "  one { v: 1 }\n", "", 1), diag.E1905.Def().Code},
}

// mustCheck is p's Check of packages.
func mustCheck(t *testing.T, p *canon.Project, packages ...string) *canon.CheckResult {
	t.Helper()
	res, err := p.Check(context.Background(), packages...)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// DECISIONS 469, API.md F9, F11: Check with no layer active evaluates every declared layer; one a
// base breaks fails the check, its finding located in its layer's file and named by it.
func TestCheckInactiveLayerFails(t *testing.T) {
	for _, c := range checkBrokenCases {
		res := mustCheck(t, checkLayersOpen(t, c.src, nil, 0, nil))
		f, ok := layerFinding(res.Findings, c.code, c.layer)
		if !ok || !res.HasErrors() || res.Summary.Errors != 1 || len(res.Findings) != 1 {
			t.Errorf("%s: %v, summary %+v: want one %s named by layer %s", c.name, res.Findings, res.Summary, c.code, c.layer)
			continue
		}
		var buf bytes.Buffer
		if err := canon.WriteFindings(&buf, []canon.Finding{f}, canon.WriteOptions{Golden: true}); err != nil {
			t.Fatal(err)
		}
		at := "error[" + string(c.code) + "]  d/" + c.layer + ".layer.canon:"
		if text := buf.String(); !strings.HasPrefix(text, at) || !strings.Contains(text, "\n  set by layer "+c.layer+"\n") {
			t.Errorf("%s: %q, want it located in its layer's file and its layer named", c.name, text)
		}
	}
}

// DECISIONS 469, API.md F7: the broken layer active fails the check once: its own finding, never
// repeated by the analysis of an inactive one.
func TestCheckActiveLayerOnce(t *testing.T) {
	for _, c := range checkBrokenCases {
		res := mustCheck(t, checkLayersOpen(t, c.src, []string{c.layer}, 0, nil))
		n := len(slices.DeleteFunc(slices.Clone(res.Findings), func(f canon.Finding) bool { return f.Code != string(c.code) }))
		if n != 1 || res.Summary.Errors != 1 {
			t.Errorf("%s, active: %v, summary %+v: want its %s once", c.name, res.Findings, res.Summary, c.code)
		}
	}
}

// DECISIONS 469: a project whose every layer evaluates passes, whichever layers are active.
func TestCheckEveryLayerClean(t *testing.T) {
	for _, active := range [][]string{nil, {"lw"}, {"lf", "lw"}} {
		if res := mustCheck(t, checkLayersOpen(t, allLayersSrc, active, 0, nil)); len(res.Findings) != 0 || res.Summary.Errors != 0 {
			t.Errorf("active %v: %v, want a clean check", active, res.Findings)
		}
	}
}

// DECISIONS 469, API.md F7: a base error present with and without each layer is reported once,
// named by no layer.
func TestCheckBaseErrorOnce(t *testing.T) {
	const bad = "\n/// A count that breaks its range.\nlet bad: Int(1..) = 0\n"
	res := mustCheck(t, checkLayersOpen(t, allLayersSrc+bad, nil, 0, nil))
	if len(res.Findings) != 1 || res.Findings[0].Layer != "" || res.Summary.Errors != 1 {
		t.Errorf("%v, summary %+v: want the base error once, named by no layer", res.Findings, res.Summary)
	}
}

// DECISIONS 469, 196, API.md R2: a layer an imported package declares is evaluated for the package
// checked, as `canon check e --layer lf` would: the amended value e reads fails, its error reported
// as an imported package's errors are, the summary counting it; e alone is checked.
func TestCheckImportedLayer(t *testing.T) {
	c := checkBrokenCases[1]
	e := map[string]string{"e/e.canon": "/// E.\npackage e\n\nimport d\n\n/// A row's value.\nlet n: Int = d.ps.two.v\n"}
	p := checkLayersOpen(t, c.src, nil, 0, e)
	res := mustCheck(t, p, "e")
	f, ok := layerFinding(res.Findings, c.code, c.layer)
	if !ok || f.Package != "d" || res.Summary.Errors == 0 || res.Summary.Packages != 1 {
		t.Errorf("%v, summary %+v: want d's %s named by layer %s, e alone checked", res.Findings, res.Summary, c.code, c.layer)
	}
	if active := mustCheck(t, checkLayersOpen(t, c.src, []string{c.layer}, 0, e), "e"); active.Summary.Errors != res.Summary.Errors {
		t.Errorf("inactive %v, active %v: want the same errors as the layer active", res.Findings, active.Findings)
	}
}

// DECISIONS 469, API.md F7: the merged findings are cut once per package, the counts and the
// truncation those of the merged list.
func TestCheckLayerCutOnce(t *testing.T) {
	const bad = "\n/// A count that breaks its range.\nlet bad: Int(1..) = 0\n"
	c := checkBrokenCases[1]
	res := mustCheck(t, checkLayersOpen(t, c.src+bad, nil, 1, nil))
	want := []canon.Truncation{{Package: "d", Errors: 1}}
	if len(res.Findings) != 1 || res.Summary.Errors != 2 || !slices.Equal(res.Summary.Truncated, want) {
		t.Errorf("%v, summary %+v: want one kept of the base's and the layer's errors, one counted dropped", res.Findings, res.Summary)
	}
}

// DECISIONS 469, API.md R3a: CheckWith in another language reports the same finding set and counts.
func TestCheckLayerLang(t *testing.T) {
	c := checkBrokenCases[0]
	p := checkLayersOpen(t, c.src, nil, 0, nil)
	res, err := p.CheckWith(context.Background(), canon.CheckRequest{Lang: "fr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := layerFinding(res.Findings, c.code, c.layer); !ok || res.Summary.Errors != 1 {
		t.Errorf("in fr: %v, summary %+v: want the layer's %s once", res.Findings, res.Summary, c.code)
	}
}

// DECISIONS 469, API.md S8, F7: concurrent checks of one snapshot, past a package's cut, each get
// the merged findings, reading the kept analysis's whole lists without lifting its cut.
func TestCheckLayerConcurrent(t *testing.T) {
	const bad = "\n/// A count that breaks its range.\nlet bad: Int(1..) = 0\n"
	c := checkBrokenCases[1]
	p := checkLayersOpen(t, c.src+bad, nil, 1, nil)
	var wg sync.WaitGroup
	results := make([]*canon.CheckResult, checkCallers)
	for i := range results {
		wg.Go(func() {
			res, err := p.Check(context.Background())
			if err != nil {
				t.Error(err)
			}
			results[i] = res
		})
	}
	wg.Wait()
	for _, res := range results {
		if res == nil || len(res.Findings) != 1 || res.Summary.Errors != 2 {
			t.Errorf("%+v: want one kept of the base's and the layer's errors", res)
		}
	}
}

// DECISIONS 464, 469, API.md F11: two inactive layers that raise the same finding located in the
// base each report it, naming its own layer: the layer is blame, and fixing one leaves the other.
func TestCheckTwoLayersSameFinding(t *testing.T) {
	src := "/// D.\npackage d\n\n/// A row.\nrecord P {\n  /// V.\n  v: Int = 0\n}\n\n/// Rows.\nlet ps: table P = {\n  one { v: 1 }\n}\n\n" +
		"/// Their total, small.\nlet total: Int(..5) = ps.one.v + 1\n"
	files := map[string]string{"project.canon": editLaw["project.canon"], "d/d.canon": src}
	for _, l := range []string{"l1", "l2"} {
		files["d/"+l+".layer.canon"] = "package d\nlayer " + l + "\n\namend ps {\n  one.v: 9\n}\n"
	}
	p, err := canon.Open("/law", project(files))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	res := mustCheck(t, p)
	var layers []string
	for _, f := range res.Findings {
		if f.File == "d/d.canon" {
			layers = append(layers, f.Layer)
		}
	}
	if !slices.Equal(layers, []string{"l1", "l2"}) || len(res.Findings) != 2 || res.Summary.Errors != 2 {
		t.Errorf("%v, summary %+v: want the base-located finding once per layer, each naming it", res.Findings, res.Summary)
	}
}

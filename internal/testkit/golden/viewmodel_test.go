package golden

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/testkit/jsonschema"
)

// viewSchemaPath is the view-model schema every view model validates against (VIEWMODEL.md V2).
var viewSchemaPath = filepath.Join("..", "..", "..", "spec", "viewmodel.schema.json")

// acceptedViews are the packages whose view models M3 acceptance 2 names: each must be written
// by the whole-project view build, and each has a view golden (exampleTargets).
var acceptedViews = []string{"pipeline", "resource.events", "resource.farm"}

// viewBreaks are schema-violating edits of a valid view model, each of which the schema must
// refuse (V1: a $schema other than canon-vm/1; V2: strict, no unknown member; 12.1: every
// top-level member but studio always present).
var viewBreaks = []struct {
	name string
	edit func(map[string]any)
}{
	{"unknown $schema", func(m map[string]any) { m["$schema"] = "canon-vm/0" }},
	{"unknown member", func(m map[string]any) { m["unknownMember"] = true }},
	{"missing types", func(m map[string]any) { delete(m, "types") }},
	{"i18n without source", func(m map[string]any) { delete(m["i18n"].(map[string]any), "source") }},
}

// compiledViewSchema is spec/viewmodel.schema.json, compiled once.
var compiledViewSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	b, err := os.ReadFile(viewSchemaPath)
	if err != nil {
		return nil, err
	}
	return jsonschema.Compile(b)
})

// viewSchema is compiledViewSchema, failing t when it does not compile.
func viewSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	s, err := compiledViewSchema()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// checkViewGoldens validates, against the view-model schema, the golden that expected/MANIFEST
// pairs with each view output of res (VIEWMODEL.md V2; M3 acceptance 2).
func checkViewGoldens(t reporter, expected string, res *canon.BuildResult) {
	t.Helper()
	for _, o := range res.Outputs {
		if o.Target != canon.TargetView {
			continue
		}
		golden := filepath.Join(expected, manifestGolden(o.Path))
		b, err := os.ReadFile(golden)
		s, serr := compiledViewSchema()
		if err = errors.Join(err, serr); err != nil {
			t.Errorf("%s: %v", golden, err)
			continue
		}
		for _, v := range s.Validate(b) {
			t.Errorf("%s: %s", golden, v)
		}
	}
}

// VIEWMODEL.md V2: checkViewGoldens fails on a view golden the schema refuses or that is
// missing, passes on a valid one, and ignores the goldens of other targets.
func TestCheckViewGoldensFailsOnInvalid(t *testing.T) {
	valid, err := os.ReadFile(filepath.Join(examplesDir, "pipeline", expectedDir, "potion.view.json"))
	if err != nil {
		t.Fatal(err)
	}
	expected := t.TempDir()
	if err := os.Mkdir(filepath.Join(expected, outDir), dirPerm); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct {
		name    string
		content []byte
	}{{"valid.json", valid}, {"invalid.json", []byte("{}\n")}} {
		if err := os.WriteFile(filepath.Join(expected, outDir, f.name), f.content, filePerm); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		name   string
		output canon.Output
		fails  bool
	}{
		{"valid", canon.Output{Path: "@out/valid.json", Target: canon.TargetView}, false},
		{"invalid", canon.Output{Path: "@out/invalid.json", Target: canon.TargetView}, true},
		{"missing", canon.Output{Path: "@out/missing.json", Target: canon.TargetView}, true},
		{"not a view", canon.Output{Path: "@out/invalid.json", Target: canon.TargetJSON}, false},
	} {
		var got errCounter
		checkViewGoldens(&got, expected, &canon.BuildResult{Outputs: []canon.Output{c.output}})
		if (got > 0) != c.fails {
			t.Errorf("%s: %d findings, want failing %v", c.name, got, c.fails)
		}
	}
}

// VIEWMODEL.md V2, J4, M3 acceptance 2: `canon build` of every example package, view target
// only, writes a view model for every emit view, errors or not, each valid against the schema;
// acceptedViews are among them, and the schema refuses each of viewBreaks applied to one.
func TestExamplesViewModelsValidate(t *testing.T) {
	root, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	proj, roots, p := openExamples(t, root)
	res, err := p.Build(context.Background(), canon.BuildOptions{Targets: []canon.Target{canon.TargetView}})
	if err != nil {
		t.Fatal(err)
	}
	s := viewSchema(t)
	models := map[string][]byte{}
	for _, o := range res.Outputs {
		src, ok := resolveDisplay(o.Path, proj, roots)
		if !ok || o.Target != canon.TargetView {
			t.Fatalf("%s (%s): not a view model under a known root", o.Path, o.Target)
		}
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range s.Validate(b) {
			t.Errorf("%s: %s", o.Path, v)
		}
		models[o.Package] = b
	}
	checkAcceptedViews(t, models)
	checkViewBreaks(t, s, models[acceptedViews[0]])
}

// checkAcceptedViews fails when a package of acceptedViews wrote no view model.
func checkAcceptedViews(t *testing.T, models map[string][]byte) {
	t.Helper()
	for _, pkg := range acceptedViews {
		if models[pkg] == nil {
			written := make([]string, 0, len(models))
			for p := range models {
				written = append(written, p)
			}
			sort.Strings(written)
			t.Errorf("no view model written for %s (written: %v)", pkg, written)
		}
	}
}

// checkViewBreaks fails when the schema accepts model after any edit of viewBreaks: the
// validation above is not vacuous.
func checkViewBreaks(t *testing.T, s *jsonschema.Schema, model []byte) {
	t.Helper()
	if len(s.Validate(model)) != 0 {
		t.Fatal("the unbroken model does not validate")
	}
	for _, c := range viewBreaks {
		var m map[string]any
		dec := json.NewDecoder(bytes.NewReader(model))
		dec.UseNumber()
		if err := dec.Decode(&m); err != nil {
			t.Fatal(err)
		}
		c.edit(m)
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if len(s.Validate(b)) == 0 {
			t.Errorf("%s: the schema accepts it", c.name)
		}
	}
}

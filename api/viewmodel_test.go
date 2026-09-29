package canon_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/api/vm"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
)

// viewPackages are M3 item 5's packages (IMPLEMENTATION-PLAN.md §6), their emit view file and golden.
var viewPackages = []struct{ pkg, file, golden string }{
	{"pipeline", "/examples/pipeline/out/potion.view.json", "/examples/pipeline/expected/potion.view.json"},
	{"resource.farm", "/out/generated/farm.view.json", "/examples/resource/farm/expected/generated/farm.view.json"},
	{"resource.events", "/out/generated/events.view.json", "/examples/resource/events/expected/generated/events.view.json"},
	{"game.items", "/out/generated/items.view.json", ""},
	{"studio", "/out/generated/studio.view.json", ""},
	{"resource.adventurequest", "/out/generated/adventurequest.view.json", ""},
	{"resource.heistia", "/out/generated/heistia.view.json", ""},
	{"sovcommon.time", "/out/generated/time.view.json", ""},
	{"resource.vocab", "/out/generated/vocab.view.json", ""},
}

// openViewExamples is the examples project with the files a build writes kept in memory.
func openViewExamples(t *testing.T) (*canon.Project, *memFS) {
	t.Helper()
	fsys := newMemFS(committedExamples())
	p, err := canon.Open(exampleRoot, canon.Options{FS: fsys, Roots: exampleRoots, Cache: "off"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p, fsys
}

// API.md R9: JSON() is the emit view file and golden, the package built alone or with M3 item 2's.
func TestViewModelEqualsEmitView(t *testing.T) {
	for _, c := range viewPackages {
		sameAsEmitView(t, []string{c.pkg})
	}
	sameAsEmitView(t, []string{"pipeline", "resource.farm", "resource.events"})
}

// sameAsEmitView builds sel's view models, then compares each with its package's ViewModel.
func sameAsEmitView(t *testing.T, sel []string) {
	t.Helper()
	p, fsys := openViewExamples(t)
	res, err := p.Build(context.Background(), canon.BuildOptions{Packages: sel, Targets: []canon.Target{canon.TargetView}})
	if err != nil || res.Check.HasErrors() {
		t.Fatalf("Build %v: %v %+v", sel, err, res)
	}
	compared := 0
	for _, c := range viewPackages {
		want, built := fsys.files[c.file]
		if !built {
			continue
		}
		compared++
		m, err := p.ViewModel(context.Background(), c.pkg)
		if err != nil {
			t.Fatalf("ViewModel(%s): %v", c.pkg, err)
		}
		if !bytes.Equal(m.JSON(), want) {
			t.Errorf("build %v: ViewModel(%s).JSON() differs from %s (%d vs %d bytes)", sel, c.pkg, c.file, len(m.JSON()), len(want))
		}
		if golden := committedExamples()[c.golden]; c.golden != "" && !bytes.Equal(m.JSON(), golden) {
			t.Errorf("ViewModel(%s).JSON() differs from its golden %s (%d vs %d bytes)", c.pkg, c.golden, len(m.JSON()), len(golden))
		}
		if m.Package != c.pkg || m.Revision != p.Revision() {
			t.Errorf("ViewModel(%s): package %q, revision %q, want %q", c.pkg, m.Package, m.Revision, p.Revision())
		}
	}
	if compared != len(sel) {
		t.Errorf("build %v wrote %d view models, want %d: %+v", sel, compared, len(sel), res.Outputs)
	}
}

// API.md R9: a package without emit view (teamboard) has its model too.
func TestViewModelWithoutEmit(t *testing.T) {
	p, _ := openViewExamples(t)
	m, err := p.ViewModel(context.Background(), "teamboard")
	if err != nil {
		t.Fatal(err)
	}
	var doc vm.ViewModel
	if err := m.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc.Schema != "canon-vm/1" || doc.Package != "teamboard" || !bytes.HasSuffix(m.JSON(), []byte("}\n")) {
		t.Errorf("teamboard model: schema %q, package %q", doc.Schema, doc.Package)
	}
}

// API.md R10: decoded into package vm and written again, a model keeps its bytes (J1).
func TestViewModelDecodeRoundTrip(t *testing.T) {
	p, _ := openViewExamples(t)
	for _, c := range viewPackages {
		m, err := p.ViewModel(context.Background(), c.pkg)
		if err != nil {
			t.Fatal(err)
		}
		var doc vm.ViewModel
		if err := m.Decode(&doc); err != nil {
			t.Fatalf("Decode(%s): %v", c.pkg, err)
		}
		again, err := viewgen.Write(&doc)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(again, m.JSON()) {
			t.Errorf("%s: decoded and written again, the model differs", c.pkg)
		}
	}
}

// API.md §5.4, S11, O6: a name of no package, a selector, a cancelled call, a closed project.
func TestViewModelErrors(t *testing.T) {
	p, _ := openViewExamples(t)
	for _, pkg := range []string{"nosuch", "resource...", "./pipeline", "", "pipeline/potion.canon", "Pipeline"} {
		if _, err := p.ViewModel(context.Background(), pkg); !errors.Is(err, canon.ErrUnknownPackage) {
			t.Errorf("ViewModel(%q): %v, want ErrUnknownPackage", pkg, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.ViewModel(ctx, "pipeline"); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled ViewModel: %v, want context.Canceled (S11)", err)
	}
	_ = p.Close()
	if _, err := p.ViewModel(context.Background(), "pipeline"); !errors.Is(err, canon.ErrClosed) {
		t.Errorf("ViewModel after Close: %v, want ErrClosed", err)
	}
}

// API.md R9, VIEWMODEL.md J4: a package with errors has its model, the file its emit view writes.
func TestViewModelWithErrors(t *testing.T) {
	broken := append(tierPackage("a"), []byte("\n/// Broken.\nlet broken: Int = \"one\"\n\nemit view { out: \"@out/a.view.json\" }\n")...)
	fsys := newMemFS(map[string][]byte{"/law/project.canon": []byte(buildTestProject), "/law/a/a.canon": broken})
	p := openTierProject(t, fsys)
	res, err := p.Build(context.Background(), canon.BuildOptions{Packages: []string{"a"}, Targets: []canon.Target{canon.TargetView}})
	if err != nil || !res.Check.HasErrors() {
		t.Fatalf("Build: %v, want errors", err)
	}
	m, err := p.ViewModel(context.Background(), "a")
	if err != nil {
		t.Fatal(err)
	}
	if written := fsys.files["/law/out/a.view.json"]; !bytes.Equal(m.JSON(), written) {
		t.Errorf("ViewModel(a) differs from the file emit view wrote (%d vs %d bytes)", len(m.JSON()), len(written))
	}
	var doc vm.ViewModel
	if err := m.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	errs := 0
	for _, f := range doc.Findings {
		if f.Severity == string(canon.SeverityError) {
			errs++
		}
	}
	if errs == 0 || len(doc.Types) == 0 {
		t.Errorf("model of a broken package: %d errors, %d types", errs, len(doc.Types))
	}
}

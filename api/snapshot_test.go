package canon_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"runtime"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// loadLaw is a package that loads a JSON file beside it, and one that does not.
var loadLaw = map[string]string{
	"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
	"a/a.canon":     "/// A.\npackage a\n\n/// Xs.\nlet xs: [Int] = load(\"a.json\")\n",
	"a/a.json":      "[1, 2]\n",
	"b/b.canon":     "/// B.\npackage b\n\n/// N.\nconst N = 1\n",
}

func openLaw(t *testing.T, files map[string]string) (*canon.Project, canon.Options) {
	t.Helper()
	opts := project(files)
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p, opts
}

// API.md S1, S3: the revision covers the files a load read; changing one gives a new revision,
// and a Check returns the revision Revision then gives.
func TestRevisionCoversLoads(t *testing.T) {
	p, opts := openLaw(t, loadLaw)
	res, err := p.Check(context.Background())
	if err != nil || res.HasErrors() || res.Revision != p.Revision() {
		t.Fatalf("Check: %v, %+v, revision %s", err, res, p.Revision())
	}
	if err := opts.FS.WriteFile("/law/a/a.json", []byte("[3]\n")); err != nil {
		t.Fatal(err)
	}
	if p.Revision() == res.Revision {
		t.Error("a loaded file changed and the revision did not")
	}
}

// API.md §3.4, S3: overlays are read and revised, never written; outside the project, ErrBadPath.
func TestOverlays(t *testing.T) {
	p, opts := openLaw(t, loadLaw)
	ctx := context.Background()
	before := p.Revision()
	if err := p.SetOverlay("b/b.canon", []byte("/// B.\npackage b\n\nconst = 1\n")); err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(ctx, "b")
	if err != nil || !res.HasErrors() || p.Revision() == before {
		t.Errorf("Check with an overlay: %v, %+v", err, res)
	}
	if disk, _ := opts.FS.ReadFile("/law/b/b.canon"); string(disk) != loadLaw["b/b.canon"] {
		t.Error("an overlay was written")
	}
	if err := p.ClearOverlay("/law/b/b.canon"); err != nil {
		t.Fatal(err)
	}
	if res, err := p.Check(ctx, "b"); err != nil || res.HasErrors() || p.Revision() != before {
		t.Errorf("Check after ClearOverlay: %v, %+v", err, res)
	}
	var perr *canon.PathError
	if err := p.SetOverlay("../x.canon", nil); !errors.Is(err, canon.ErrBadPath) || !errors.As(err, &perr) || perr.Path != "../x.canon" {
		t.Errorf("an overlay outside the project: %v", err)
	}
	_ = p.Close()
	if err := p.SetOverlay("b/b.canon", nil); !errors.Is(err, canon.ErrClosed) {
		t.Errorf("SetOverlay after Close: %v (O6)", err)
	}
}

// API.md §2.2, §3.4 (log M4 B12-r): on Windows '\' and a drive's case key one overlay.
func TestOverlayBackslash(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip(`'\' is a name character outside Windows`)
	}
	p, _ := openLaw(t, loadLaw)
	before := p.Revision()
	if err := p.SetOverlay(`b\b.canon`, []byte("/// B.\npackage b\n\nconst = 1\n")); err != nil {
		t.Fatal(err)
	}
	if res, err := p.Check(context.Background(), "b"); err != nil || !res.HasErrors() {
		t.Errorf("Check with a backslash overlay: %v, %+v", err, res)
	}
	if err := p.ClearOverlay(`\law\b\b.canon`); err != nil || p.Revision() != before {
		t.Errorf("ClearOverlay: %v, revision %s, want %s", err, p.Revision(), before)
	}
	lower := strings.ToLower(p.Root()[:1]) + p.Root()[1:] + "/b/b.canon" // "d:/law/b/b.canon"
	if err := p.SetOverlay(lower, []byte("/// B.\npackage b\n\nconst = 1\n")); err != nil {
		t.Fatal(err)
	}
	if err := p.ClearOverlay("b/b.canon"); err != nil || p.Revision() != before {
		t.Errorf("ClearOverlay after a lower-case drive: %v, revision %s, want %s", err, p.Revision(), before)
	}
}

// API.md R4: a Value holds its snapshot across revisions: an overlay and a file written after it
// leave it as it was, while a new Value sees each change.
func TestValueAcrossRevisions(t *testing.T) {
	p, opts := openLaw(t, loadLaw)
	held := mustValue(t, p, "b:N")
	if err := p.SetOverlay("b/b.canon", []byte("/// B.\npackage b\n\n/// N.\nconst N = 2\n")); err != nil {
		t.Fatal(err)
	}
	if v := mustValue(t, p, "b:N"); v.Text != "2" || held.Text != "1" {
		t.Errorf("after an overlay: new %s, held %s", v.Text, held.Text)
	}
	if err := opts.FS.WriteFile("/law/a/a.json", []byte("[5]\n")); err != nil {
		t.Fatal(err)
	}
	xs := mustValue(t, p, "a:xs")
	if xs.Text != "[5]" || held.Text != "1" || len(held.Children()) != 0 {
		t.Errorf("after a write: xs %s, held %s", xs.Text, held.Text)
	}
}

// API.md O7: Workers, Cache, Logger and Lang change nothing a check computes; Layers do.
func TestOptionsDoNotChangeResults(t *testing.T) {
	files := map[string]string{
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
		"a/a.canon": "/// A.\npackage a\n\n/// Cfg.\nrecord Cfg {\n  /// N.\n  n: Int\n}\n\n/// Config.\nlet cfg: Cfg = { n: 1 }\n\n" +
			"/// Bad.\nlet bad: Int = \"s\"\n",
		"a/dev.layer.canon": "package a\nlayer dev\n\namend cfg {\n  n: 2\n}\n",
	}
	run := func(mod func(*canon.Options)) (*canon.CheckResult, string) {
		opts := project(files)
		mod(&opts)
		p, err := canon.Open("/law", opts)
		if err != nil {
			t.Fatal(err)
		}
		defer p.Close()
		res, err := p.Check(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		res.Duration = 0
		return res, mustValue(t, p, "a:cfg.n").Text
	}
	base, n := run(func(*canon.Options) {})
	for i, mod := range []func(*canon.Options){
		func(o *canon.Options) { o.Workers = 1 },
		func(o *canon.Options) { o.Cache = "off" },
		func(o *canon.Options) { o.Logger = slog.New(slog.NewTextHandler(io.Discard, nil)) },
		func(o *canon.Options) { o.Lang = "fr" },
	} {
		if got, gotN := run(mod); !reflect.DeepEqual(got, base) || gotN != n {
			t.Errorf("option %d changed the check: %+v, %s; want %+v, %s", i, got, gotN, base, n)
		}
	}
	if _, layered := run(func(o *canon.Options) { o.Layers = []string{"dev"} }); layered != "2" || n != "1" || !base.HasErrors() {
		t.Errorf("with the layer n = %s, without %s", layered, n)
	}
}

// API.md S11: every read returns ctx.Err() when its ctx is done.
func TestCancelledReads(t *testing.T) {
	p, _ := openLaw(t, loadLaw)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, errCheck := p.Check(ctx)
	_, errPkgs := p.Packages(ctx)
	_, errValue := p.Value(ctx, "b:N")
	_, errBuild := p.Build(ctx, canon.BuildOptions{Check: true})
	_, errWrite := p.Build(ctx, canon.BuildOptions{})
	_, errTest := p.Test(ctx, canon.TestOptions{})
	for i, err := range []error{errCheck, errPkgs, errValue, errBuild, errWrite, errTest} {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("call %d: %v", i, err)
		}
	}
}

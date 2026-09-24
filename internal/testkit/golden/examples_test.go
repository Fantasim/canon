package golden

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

const (
	examplesDir  = "../../../examples"
	manifestFile = "MANIFEST"
	findingsFile = "findings.txt"
)

// exampleSkip lists examples whose build cannot run yet: pipeline is illustrative until GEN-01 regenerates it (M2, DOCTRINE.md §4).
var exampleSkip = map[string]bool{"pipeline": true}

// exampleExtra are extra selectors an example's Build needs beyond its own package (M1
// acceptance item 3: sovcommon.ui and sovcommon.roles must also be emitted, not just imported).
var exampleExtra = map[string][]string{"teamboard": {"sovcommon..."}}

// exampleWriteRoots are the roots examples/project.canon declares that only a build writes to;
// resource and client are redirected to the fixtures instead, read-only
// (examples/_fixtures/README.md).
var exampleWriteRoots = []string{"source", "services", "sovcommon", "web", "parity", "generated"}

// TestExamples rebuilds every example with expected/MANIFEST (exampleSkip aside), comparing every listed file with its golden (-update rewrites it) and failing on an unlisted write (IMPLEMENTATION-PLAN.md §7.1, DECISIONS 201).
func TestExamples(t *testing.T) {
	root, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	manifests, err := filepath.Glob(filepath.Join(root, "*", "expected", manifestFile))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(manifests)
	if len(manifests) == 0 {
		t.Fatal("no example has expected/MANIFEST")
	}
	for _, m := range manifests {
		expected := filepath.Dir(m)
		name := filepath.Base(filepath.Dir(expected))
		if exampleSkip[name] {
			continue
		}
		t.Run(name, func(t *testing.T) { runExample(t, root, name, expected) })
	}
}

// runExample copies examples/ into a temporary project, builds name (and exampleExtra[name])
// into it, and compares findings.txt and every file expected/MANIFEST lists.
func runExample(t *testing.T, root, name, expected string) {
	t.Helper()
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "proj")
	if err := os.CopyFS(proj, os.DirFS(root)); err != nil {
		t.Fatal(err)
	}
	roots := exampleRoots(proj, tmp)
	p, err := canon.Open(filepath.ToSlash(proj), canon.Options{Roots: roots, Cache: "off"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	// findings.txt is `canon check <example>` alone (API.md R2); Build's own selection also
	// carries exampleExtra[name], the sibling packages M1 acceptance item 3 also emits.
	checked, err := p.Check(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	compareFindings(t, expected, checked)
	selectors := append([]string{name}, exampleExtra[name]...)
	res, err := p.Build(context.Background(), canon.BuildOptions{
		Packages: selectors, Targets: []canon.Target{canon.TargetGo, canon.TargetJSON},
	})
	if err != nil {
		t.Fatal(err)
	}
	listed := compareManifest(t, expected, proj, roots)
	checkNothingUnlisted(t, res, listed)
}

// exampleRoots redirects every root outside the project into tmp, and resource/client into the
// copy's own fixtures (examples/_fixtures/README.md); pipeline_go and features are not
// redirected, since they are inside the project already.
func exampleRoots(proj, tmp string) map[string]string {
	roots := map[string]string{
		"resource": filepath.Join(proj, "_fixtures", "resource"),
		"client":   filepath.Join(proj, "_fixtures", "client"),
	}
	for _, r := range exampleWriteRoots {
		roots[r] = filepath.Join(tmp, "out", r)
	}
	return roots
}

// compareFindings compares (or, under -update, rewrites) expected/findings.txt: `canon check`'s text form, sorted per API.md F2, the duration replaced by "(…)" (IMPLEMENTATION-PLAN.md §7.2).
func compareFindings(t *testing.T, expected string, check *canon.CheckResult) {
	t.Helper()
	var buf bytes.Buffer
	opt := canon.WriteOptions{Summary: check.Summary, Duration: check.Duration, Golden: true}
	if err := canon.WriteFindings(&buf, check.Findings, opt); err != nil {
		t.Fatal(err)
	}
	compareOrUpdate(t, filepath.Join(expected, findingsFile), buf.Bytes())
}

// compareManifest resolves and compares (or, under -update, rewrites) every file
// expected/MANIFEST lists, and returns the display paths it names.
func compareManifest(t *testing.T, expected, proj string, roots map[string]string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(expected, manifestFile))
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		display, golden, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("MANIFEST: malformed line %q", line)
		}
		listed[display] = true
		src, ok := resolveDisplay(display, proj, roots)
		if !ok {
			t.Fatalf("MANIFEST: %s: unknown root", display)
		}
		got, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("MANIFEST: %s: %v", display, err)
		}
		compareOrUpdate(t, filepath.Join(expected, golden), got)
	}
	return listed
}

// resolveDisplay is the file a display path (WIRE.md §2.3) names in this rebuild.
func resolveDisplay(display, proj string, roots map[string]string) (string, bool) {
	rest, ok := strings.CutPrefix(display, "@")
	if !ok {
		return filepath.Join(proj, filepath.FromSlash(display)), true
	}
	root, sub, _ := strings.Cut(rest, "/")
	dir, ok := roots[root]
	if !ok {
		return "", false
	}
	return filepath.Join(dir, filepath.FromSlash(sub)), true
}

// reporter is the subset of *testing.T checkNothingUnlisted needs, so a test can capture its findings without failing (DECISIONS 201).
type reporter interface {
	Helper()
	Errorf(format string, args ...any)
}

// checkNothingUnlisted fails when the build wrote an output, or appended a lock, that
// expected/MANIFEST does not list (DECISIONS 201).
func checkNothingUnlisted(t reporter, res *canon.BuildResult, listed map[string]bool) {
	t.Helper()
	for _, o := range res.Outputs {
		if o.Status != canon.OutputUnchanged && !listed[o.Path] {
			t.Errorf("MANIFEST does not list the output %s (%s)", o.Path, o.Status)
		}
	}
	for _, l := range res.Lock {
		if !listed[l.File] {
			t.Errorf("MANIFEST does not list the lock %s", l.File)
		}
	}
}

// errCounter is a reporter that counts its findings instead of failing the test.
type errCounter int

func (c *errCounter) Helper() {}

func (c *errCounter) Errorf(string, ...any) { *c++ }

// DECISIONS 201: an output or a lock the build wrote but expected/MANIFEST does not list fails
// the check; one already listed does not.
func TestCheckNothingUnlistedFailsOnExtra(t *testing.T) {
	unlistedOutput := &canon.BuildResult{Outputs: []canon.Output{{Path: "@out/new.json", Status: canon.OutputWritten}}}
	unlistedLock := &canon.BuildResult{Lock: []canon.LockChange{{Package: "a", File: "a/canon.lock"}}}
	listed := map[string]bool{"@out/new.json": true, "a/canon.lock": true}
	for _, c := range []struct {
		name   string
		res    *canon.BuildResult
		listed map[string]bool
		want   errCounter
	}{
		{"unlisted output", unlistedOutput, map[string]bool{}, 1},
		{"unlisted lock", unlistedLock, map[string]bool{}, 1},
		{"listed output", unlistedOutput, listed, 0},
		{"listed lock", unlistedLock, listed, 0},
	} {
		var got errCounter
		checkNothingUnlisted(&got, c.res, c.listed)
		if got != c.want {
			t.Errorf("%s: %d findings, want %d", c.name, got, c.want)
		}
	}
}

// compareOrUpdate compares got with path's content, or, under -update, writes it.
func compareOrUpdate(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, filePerm); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s: %v\n--- want\n%s\n--- got\n%s", path, errDiffers, want, got)
	}
}

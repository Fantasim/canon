package golden

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// errDuplicateGolden is buildManifest's error when two display paths would share one golden
// name: one would silently overwrite the other's file.
var errDuplicateGolden = errors.New("two display paths share one golden name")

const (
	examplesDir  = "../../../examples"
	manifestFile = "MANIFEST"
	findingsFile = "findings.txt"
	// expectedDir and fixturesDir are discoverManifests' special directory names.
	expectedDir = "expected"
	fixturesDir = "_fixtures"
)

// defaultTargets are the targets an example without its own row in exampleTargets is built for.
var defaultTargets = []canon.Target{canon.TargetGo, canon.TargetJSON}

// exampleTargets overrides defaultTargets for the examples with a MANIFEST whose cpp emit gen/cpp writes: pipeline and features.dependent, both in data mode.
var exampleTargets = map[string][]canon.Target{
	"pipeline":           {canon.TargetGo, canon.TargetCpp, canon.TargetJSON},
	"features.dependent": {canon.TargetGo, canon.TargetCpp, canon.TargetJSON},
}

// targetsFor is exampleTargets[name], or defaultTargets without a row.
func targetsFor(name string) []canon.Target {
	if t, ok := exampleTargets[name]; ok {
		return t
	}
	return defaultTargets
}

// exampleExtra are extra selectors an example's Build needs beyond its own package (M1
// acceptance item 3: sovcommon.ui and sovcommon.roles must also be emitted, not just imported).
var exampleExtra = map[string][]string{"teamboard": {"sovcommon..."}}

// outDir is the directory name an emit's own out: option always writes under: copyProject skips
// it so a fixture's own out/, if ever checked in by mistake, never looks "unchanged" on the
// harness's first build.
const outDir = "out"

// exampleWriteRoots are the roots examples/project.canon declares that only a build writes to;
// resource and client are redirected to the fixtures instead, read-only
// (examples/_fixtures/README.md).
var exampleWriteRoots = []string{"source", "services", "sovcommon", "web", "parity", "generated"}

// TestExamples rebuilds every example with expected/MANIFEST, comparing every listed file with its golden (-update rewrites it) and failing on an unlisted write or a listed-but-unwritten path (DECISIONS 201).
func TestExamples(t *testing.T) {
	root, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	manifests, err := discoverManifests(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) == 0 {
		t.Fatal("no example has expected/MANIFEST")
	}
	for _, m := range manifests {
		expected := filepath.Dir(m)
		name := manifestName(root, expected)
		t.Run(name, func(t *testing.T) { runExample(t, root, name, expected) })
	}
}

// discoverManifests is every expected/MANIFEST under root, sorted, at any depth, skipping fixturesDir (IMPLEMENTATION-PLAN §7.1).
func discoverManifests(root string) ([]string, error) {
	var manifests []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == fixturesDir {
			return fs.SkipDir
		}
		if !d.IsDir() && d.Name() == manifestFile && filepath.Base(filepath.Dir(path)) == expectedDir {
			manifests = append(manifests, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(manifests)
	return manifests, nil
}

// manifestName is the package name TestExamples runs an example's subtest under: expected's
// parent directory, relative to root, with "/" turned into "." (balance/parity → balance.parity).
func manifestName(root, expected string) string {
	rel, err := filepath.Rel(root, filepath.Dir(expected))
	if err != nil {
		return filepath.Base(filepath.Dir(expected))
	}
	return strings.ReplaceAll(filepath.ToSlash(rel), "/", ".")
}

// runExample copies examples/ into a temporary project, builds name (and exampleExtra[name])
// into it, and compares findings.txt and every file expected/MANIFEST lists.
func runExample(t *testing.T, root, name, expected string) {
	t.Helper()
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "proj")
	if err := copyProject(proj, root); err != nil {
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
		Packages: selectors, Targets: targetsFor(name),
	})
	if err != nil {
		t.Fatal(err)
	}
	listed := compareManifest(t, expected, proj, roots, res)
	checkNothingUnlisted(t, res, listed)
	checkNothingUnwritten(t, res, listed)
}

// copyProject copies root into proj, skipping every out/ directory: a fixture's own out/, if
// ever checked in by mistake, would let a first build report its matching outputs "unchanged"
// and hide a MANIFEST entry checkNothingUnlisted or checkNothingUnwritten should have caught.
func copyProject(proj, root string) error {
	return fs.WalkDir(os.DirFS(root), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == outDir {
				return fs.SkipDir
			}
			return os.MkdirAll(filepath.Join(proj, p), dirPerm)
		}
		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(proj, p), b, filePerm)
	})
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

// compareManifest resolves and compares every file expected/MANIFEST lists; under -update it
// first rewrites MANIFEST itself from res's outputs and locks (DECISIONS 201: a golden, never
// typed by hand), then rewrites each file it names. It returns the display paths it names.
func compareManifest(t *testing.T, expected, proj string, roots map[string]string, res *canon.BuildResult) map[string]bool {
	t.Helper()
	manifest := filepath.Join(expected, manifestFile)
	if *update {
		content, err := buildManifest(res)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifest, content, filePerm); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(manifest)
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

// buildManifest is expected/MANIFEST's content for a build's result (IMPLEMENTATION-PLAN §7.1): one line per output, then per lock, "<display path written> <golden path relative to expected/>". Two display paths that would share one golden name are an error: one would silently overwrite the other's file.
func buildManifest(res *canon.BuildResult) ([]byte, error) {
	var b strings.Builder
	seen := map[string]string{}
	add := func(display string) error {
		golden := manifestGolden(display)
		if prev, ok := seen[golden]; ok {
			return fmt.Errorf("%w: %s and %s", errDuplicateGolden, prev, display)
		}
		seen[golden] = display
		fmt.Fprintf(&b, "%s %s\n", display, golden)
		return nil
	}
	for _, o := range res.Outputs {
		if err := add(o.Path); err != nil {
			return nil, err
		}
	}
	for _, l := range res.Lock {
		if err := add(l.File); err != nil {
			return nil, err
		}
	}
	return []byte(b.String()), nil
}

// manifestGolden is the golden name MANIFEST pairs with a display path: an outside root's "@"
// dropped, or a project-relative path's package directory and, right after it, an "out/" segment
// dropped (a lock, written beside the package's source, has neither to drop).
func manifestGolden(display string) string {
	if rest, ok := strings.CutPrefix(display, "@"); ok {
		return rest
	}
	_, rest, ok := strings.Cut(display, "/")
	if !ok {
		return display
	}
	if trimmed, ok := strings.CutPrefix(rest, "out/"); ok {
		return trimmed
	}
	return rest
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

// reporter is the subset of *testing.T checkNothingUnlisted and checkNothingUnwritten need, so a test can capture their findings without failing (DECISIONS 201).
type reporter interface {
	Helper()
	Errorf(format string, args ...any)
}

// checkNothingUnlisted fails when the build reported an output, of any status, or appended a
// lock, that expected/MANIFEST does not list (DECISIONS 201).
func checkNothingUnlisted(t reporter, res *canon.BuildResult, listed map[string]bool) {
	t.Helper()
	for _, o := range res.Outputs {
		if !listed[o.Path] {
			t.Errorf("MANIFEST does not list the output %s (%s)", o.Path, o.Status)
		}
	}
	for _, l := range res.Lock {
		if !listed[l.File] {
			t.Errorf("MANIFEST does not list the lock %s", l.File)
		}
	}
}

// checkNothingUnwritten fails when expected/MANIFEST lists a path that is neither an output nor
// a lock the build reported: a stale entry left over from an emit the build no longer runs (the
// reverse of checkNothingUnlisted; DECISIONS 201).
func checkNothingUnwritten(t reporter, res *canon.BuildResult, listed map[string]bool) {
	t.Helper()
	written := map[string]bool{}
	for _, o := range res.Outputs {
		written[o.Path] = true
	}
	for _, l := range res.Lock {
		written[l.File] = true
	}
	paths := make([]string, 0, len(listed))
	for path := range listed {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if !written[path] {
			t.Errorf("MANIFEST lists %s, which the build did not report", path)
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

// DECISIONS 201: a path expected/MANIFEST lists that the build reported neither as an output
// nor as a lock fails the check; one it did report does not.
func TestCheckNothingUnwrittenFailsOnMissing(t *testing.T) {
	res := &canon.BuildResult{
		Outputs: []canon.Output{{Path: "@out/kept.json", Status: canon.OutputWritten}},
		Lock:    []canon.LockChange{{Package: "a", File: "a/canon.lock"}},
	}
	for _, c := range []struct {
		name   string
		listed map[string]bool
		want   errCounter
	}{
		{"stale output entry", map[string]bool{"@out/kept.json": true, "@out/stale.json": true, "a/canon.lock": true}, 1},
		{"stale lock entry", map[string]bool{"@out/kept.json": true, "a/canon.lock": true, "b/canon.lock": true}, 1},
		{"every entry written", map[string]bool{"@out/kept.json": true, "a/canon.lock": true}, 0},
	} {
		var got errCounter
		checkNothingUnwritten(&got, res, c.listed)
		if got != c.want {
			t.Errorf("%s: %d findings, want %d", c.name, got, c.want)
		}
	}
}

// buildManifest fails when two outputs would share one golden name (a display path that
// manifestGolden reduces to the same string as another's).
func TestBuildManifestFailsOnDuplicateGolden(t *testing.T) {
	res := &canon.BuildResult{Outputs: []canon.Output{
		{Path: "a/out/x.json", Status: canon.OutputWritten},
		{Path: "b/out/x.json", Status: canon.OutputWritten},
	}}
	if _, err := buildManifest(res); !errors.Is(err, errDuplicateGolden) {
		t.Errorf("buildManifest: %v, want %v", err, errDuplicateGolden)
	}
}

// discoverManifests finds a nested package's expected/MANIFEST, not just a top-level one's,
// skips one under fixturesDir, and manifestName turns its path back into a dotted name:
// balance/parity/expected/MANIFEST names the package "balance.parity".
func TestDiscoverManifestsNamesNestedPackages(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{
		"pipeline/expected",
		"balance/parity/expected",
		fixturesDir + "/resource/expected",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), dirPerm); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{"pipeline/expected", "balance/parity/expected"} {
		if err := os.WriteFile(filepath.Join(root, dir, manifestFile), nil, filePerm); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, fixturesDir, "resource", "expected", manifestFile), nil, filePerm); err != nil {
		t.Fatal(err)
	}
	manifests, err := discoverManifests(root)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(manifests))
	for i, m := range manifests {
		names[i] = manifestName(root, filepath.Dir(m))
	}
	if want := []string{"balance.parity", "pipeline"}; !slices.Equal(names, want) {
		t.Errorf("names: %v, want %v", names, want)
	}
}

// compareOrUpdate compares got with path's content, or, under -update, writes it, creating
// path's directory first: a new example's golden (balance.parity's expected/parity/, e.g.) has
// none yet, unlike every directory an already-generated golden writes into again.
func compareOrUpdate(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
			t.Fatal(err)
		}
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

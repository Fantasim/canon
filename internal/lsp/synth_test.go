package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shape sizes a synthetic project: packages, data files per package, entries per data file,
// how many earlier packages each one imports, and the big files (one in each of the first big
// packages) with their entries: a real project's largest files hold megabytes.
type shape struct {
	packages, files, entries, imports int
	big, bigEntries                   int
}

// The memory bench's shape, about a large real project's (DECISIONS 368): 80 packages of
// 60 data files, 9 entries each, and two big files of 15,000 entries, ~5,000 files and ~700k
// lines; and the always-on test's small one.
var (
	shapeLarge = shape{packages: 80, files: 60, entries: 9, imports: 2, big: 2, bigEntries: 15000}
	shapeSmall = shape{packages: 6, files: 4, entries: 3, imports: 2, big: 1, bigEntries: 1500}
)

// The synthetic project's vocabulary.
const (
	synthAnchors = 8
	synthLevels  = 500
	synthWeights = 1000
	synthPkg     = "p%02d"
	synthFile    = "d%03d"
	synthRows    = "rows%02dF%03d"
	synthLevel   = "level: "
	synthBig     = "big"
	synthBigLet  = "big%02d"
	synthPerm    = 0o644
	synthDirPerm = 0o755
)

var synthKinds = []string{"K_A", "K_B", "K_C", "K_D"}

// writeSynth writes a project of sh under dir: project.canon, then per package its types, a
// French translation and its data files, each a table of entries whose refs reach the package's
// anchors and an imported package's.
func writeSynth(t testing.TB, dir string, sh shape) {
	t.Helper()
	put := func(rel, text string) {
		name := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(name), synthDirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(text), synthPerm); err != nil {
			t.Fatal(err)
		}
	}
	put("project.canon", "project synth {\n  canon: \"0.1\"\n  languages: [en, fr]\n}\n")
	for p := range sh.packages {
		pkg := fmt.Sprintf(synthPkg, p)
		put(pkg+"/"+pkg+".canon", synthTypes(p, sh))
		put(pkg+"/"+pkg+".fr.canon", synthFrench(p))
		for f := range sh.files {
			put(pkg+"/"+fmt.Sprintf(synthFile, f)+".canon", synthTable(p, fmt.Sprintf(synthRows, p, f), f*sh.entries, sh.entries, sh))
		}
		if p < sh.big {
			put(pkg+"/"+synthBig+".canon", synthTable(p, fmt.Sprintf(synthBigLet, p), sh.files*sh.entries, sh.bigEntries, sh))
		}
	}
}

// synthDeps are the earlier packages p imports.
func synthDeps(p int, sh shape) []int {
	var out []int
	for d := p - sh.imports; d < p; d++ {
		if d >= 0 {
			out = append(out, d)
		}
	}
	return out
}

// synthTypes is a package's main file: imports, an enum, the anchors table, the row record
// with a ref to its anchors and one to an imported package's, a function and a check.
func synthTypes(p int, sh shape) string {
	var b strings.Builder
	fmt.Fprintf(&b, "/// Package %d of the synthetic project.\npackage p%02d\n\n", p, p)
	deps := synthDeps(p, sh)
	for _, d := range deps {
		fmt.Fprintf(&b, "import p%02d { anchors%02d }\n", d, d)
	}
	fmt.Fprintf(&b, "\n/// A row's kind.\nenum Kind%02d { %s }\n\n", p, strings.Join(synthKinds, ", "))
	fmt.Fprintf(&b, "/// What rows point at.\nrecord Anchor%02d {\n  /// Its name.\n  name: String(1..)\n  /// Its weight.\n  weight: Int(0..=%d)\n}\n\n", p, synthWeights)
	fmt.Fprintf(&b, "/// Every anchor of package %d.\nlet anchors%02d: table Anchor%02d = {\n", p, p, p)
	for a := range synthAnchors {
		fmt.Fprintf(&b, "  A%d { name: \"anchor %d of %d\", weight: %d }\n", a, a, p, a*p%synthWeights)
	}
	b.WriteString("}\n\n")
	fmt.Fprintf(&b, "/// A row of package %d.\nrecord Row%02d {\n", p, p)
	b.WriteString("  /// Name shown.\n  name: String(1..)\n")
	fmt.Fprintf(&b, "  /// Its level.\n  level: Int(1..=%d)\n", synthLevels)
	fmt.Fprintf(&b, "  /// Its kind.\n  kind: Kind%02d\n", p)
	fmt.Fprintf(&b, "  /// Its anchor.\n  anchor: ref anchors%02d\n", p)
	if len(deps) > 0 {
		fmt.Fprintf(&b, "  /// An anchor of another package.\n  far: ref anchors%02d?\n", deps[0])
	}
	b.WriteString("  /// Free tags.\n  tags: [String]\n  /// A note.\n  note: String\n")
	b.WriteString("\n  check level > 0 else \"a row's level is at least 1, here {level}\"\n}\n\n")
	fmt.Fprintf(&b, "/// The label of a row's level.\nfn label%02d(r: Row%02d) -> String {\n  return \"{r.name} at {r.level}\"\n}\n\n", p, p)
	fmt.Fprintf(&b, "check "+synthRows+".values().map(.name).isUnique() else \"row names are unique\"\n", p, 0)
	return b.String()
}

// synthFrench is a package's French translation of its row record's labels.
func synthFrench(p int) string {
	return fmt.Sprintf("package p%02d\ntranslation fr\n\nRow%02d.name \"Nom\"\nRow%02d.level \"Niveau\"\nRow%02d.kind \"Genre\"\n", p, p, p, p)
}

// synthTable is one data file: the table let of count rows, each over several lines, numbered
// from first.
func synthTable(p int, let string, first, count int, sh shape) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package p%02d\n\n/// Rows from %d.\nlet %s: table Row%02d = {\n", p, first, let, p)
	far := len(synthDeps(p, sh)) > 0
	for e := range count {
		n := first + e
		fmt.Fprintf(&b, "  R%d {\n    name: \"row %d of %d\"\n    level: %d\n", e, n, p, 1+n%synthLevels)
		fmt.Fprintf(&b, "    kind: %s\n    anchor: A%d\n", synthKinds[n%len(synthKinds)], n%synthAnchors)
		if far {
			fmt.Fprintf(&b, "    far: A%d\n", (n+p)%synthAnchors)
		}
		fmt.Fprintf(&b, "    tags: [\"t%d\", \"u%d\"]\n    note: \"note %d\"\n  }\n", n%len(synthKinds), p, n)
	}
	b.WriteString("}\n")
	return b.String()
}

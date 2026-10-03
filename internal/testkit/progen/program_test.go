package progen_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
	"github.com/fantasim/canonlang/internal/testkit/progen/grammar"
)

const (
	genFile        = "gen/gen.canon"
	genPackage     = "gen"
	genEmpty       = "package gen\n"
	bom            = "\xef\xbb\xbf"
	genProject     = "project gen {\n  canon: \"0.1\"\n  budget: 200_000\n}\n"
	genSize        = 160 // nodes per generated file (DECISIONS 200: a per-case size budget)
	genDepth       = 7
	shortGrammar   = 300
	shortCorrupt   = 300
	propRoundTrip  = "roundtrip"
	propCorruption = "corruption"
	contextDepth   = 2
)

// TestGrammar is DECISIONS 200's grammar-driven suite on valid files: a generated file parses,
// formats, reparses to its shape, is a fixed point, and checks without a crash.
func TestGrammar(t *testing.T) {
	if supervise(t, testGrammar, total(shortGrammar)) {
		return
	}
	from, to := cases(shortGrammar)
	for i := from; i < to; i++ {
		seed := caseSeed(suiteGrammar, i)
		announce(suiteGrammar, propRoundTrip, i, seed)
		kind := caseKind(i)
		src := generate(seed, kind)
		if v := roundTrip(src, false, kind); v.Kind != "" {
			reportProgram(t, programFailure{suiteGrammar, propRoundTrip, kind, i, seed, src, v})
		}
	}
}

// TestCorruption is DECISIONS 200's grammar-driven suite on corrupted files: no crash, and a
// file that does not parse gets an error, every finding inside the file.
func TestCorruption(t *testing.T) {
	if supervise(t, testCorruption, total(shortCorrupt)) {
		return
	}
	from, to := cases(shortCorrupt)
	for i := from; i < to; i++ {
		seed := caseSeed(suiteCorrupt, i)
		announce(suiteCorrupt, propCorruption, i, seed)
		r := progen.NewRand(seed)
		kind := caseKind(i)
		src := grammar.Corrupt(r, grammar.GenerateKind(r, progen.NewBudget(genSize, genDepth), kind))
		if v := corruption(src, kind); v.Kind != "" {
			reportProgram(t, programFailure{suiteCorrupt, propCorruption, kind, i, seed, src, v})
		}
	}
}

// caseKind is the kind of file case k generates: the kinds in turn (the second wave of
// DECISIONS 200: layer, translation and project files beside source files).
func caseKind(k int) grammar.Kind {
	kinds := grammar.Kinds()
	return kinds[k%len(kinds)]
}

func generate(seed uint64, kind grammar.Kind) []byte {
	return grammar.GenerateKind(progen.NewRand(seed), progen.NewBudget(genSize, genDepth), kind)
}

// roundTrip is "" for a file that parses clean (warnings aside, when allowed), formats, reparses
// to its shape, is a fixed point of the formatter and checks without a crash; else the verdict.
func roundTrip(src []byte, warnings bool, kind grammar.Kind) (v verdict) {
	defer recoverVerdict(&v)
	tree, findings := grammar.ParseKind(kind.File(), src, kind)
	if len(findings) > 0 && (!warnings || hasError(findings)) {
		return verdict{Kind: "unparsed", Sig: "unparsed " + diagShapes(findings), Text: "unparsed: a generated file has findings: " + located(findings)}
	}
	out, err := formatText(src, kind)
	if err != nil {
		return verdict{Kind: "unformatted", Sig: "unformatted", Text: "unformatted: " + err.Error()}
	}
	again, reparsed := grammar.ParseKind(kind.File(), out, kind)
	switch {
	case hasError(reparsed) || len(reparsed) > len(findings):
		where := nodeAt(again, int(reparsed[0].Span.Start))
		return verdict{Kind: "reparse", Sig: "reparse " + diagShapes(reparsed) + " in " + where, Text: "reparse: the formatted file has findings: " + located(reparsed) + "\n" + string(out)}
	case grammar.Shape(tree) != grammar.Shape(again):
		return verdict{Kind: "shape", Sig: "shape " + shapeSig(firstDifference(grammar.Shape(tree), grammar.Shape(again))), Text: "shape: formatting changed the tree:\n" + string(out)}
	}
	if twice, err := formatText(out, kind); err != nil || !bytes.Equal(twice, out) {
		where := nodeAt(again, commonPrefix(out, twice))
		return verdict{Kind: "idempotence", Sig: "idempotence in " + where, Text: "idempotence: formatting the formatted file changes it:\n" + string(twice)}
	}
	return checked(src, kind)
}

// corruption is "" for a file that does not crash the parser, the formatter or the compiler,
// and gets a located error when it does not parse; a file that parses must round-trip.
func corruption(src []byte, kind grammar.Kind) (v verdict) {
	defer recoverVerdict(&v)
	tree, findings := grammar.ParseKind(kind.File(), src, kind)
	if !hasError(findings) {
		return roundTrip(src, true, kind)
	}
	for _, f := range findings {
		if f.Span.File != tree.Src.ID || f.Span.Start > f.Span.End || int(f.Span.End) > len(tree.Src.Content) {
			return verdict{Kind: "unlocated", Sig: "unlocated " + shapeOf(f.Code, f.Message), Text: fmt.Sprintf("unlocated: %s has span %+v outside the file", f.Code, f.Span)}
		}
	}
	if onlyBOM(findings) {
		return bomFormat(src, kind)
	}
	if _, err := formatText(src, kind); !errors.Is(err, format.ErrSyntax) {
		return verdict{Kind: "formatted", Sig: "formatted", Text: fmt.Sprintf("formatted: a file with syntax errors formats: %v", err)}
	}
	return checked(src, kind)
}

// onlyBOM tells findings that are all E1123, a byte order mark at the start (GRAMMAR.md §1).
func onlyBOM(fs []diag.Finding) bool {
	for _, f := range fs {
		if f.Code != diag.E1123.Def().Code && f.Severity == diag.Error {
			return false
		}
	}
	return true
}

// bomFormat is "" for a file with a BOM that formats as the file without it (FORMATTER.md §10).
func bomFormat(src []byte, kind grammar.Kind) verdict {
	out, err := formatText(src, kind)
	want, werr := formatText(bytes.TrimPrefix(src, []byte(bom)), kind)
	if err != nil || werr != nil || !bytes.Equal(out, want) {
		return verdict{Kind: "formatted", Sig: "formatted bom", Text: fmt.Sprintf("formatted: a byte order mark changes the layout: %v %v", err, werr)}
	}
	if v := roundTrip(bytes.TrimPrefix(src, []byte(bom)), true, kind); v.Kind != "" {
		return v
	}
	return checked(src, kind)
}

// checked runs the compiler on the file in a project of one package: no panic, no internal
// error, and every finding located. A layer or translation file sits in a package of an empty
// source file; project.canon, with one.
func checked(src []byte, kind grammar.Kind) verdict {
	p := progen.NewProject()
	p.Set(projectFile, []byte(genProject))
	p.Set(genFile, []byte(genEmpty))
	p.Set(kind.File(), src)
	out := progen.Run(context.Background(), p, progen.RunOptions{Packages: []string{genPackage}})
	if v, bad := broken(out); bad {
		return v
	}
	for _, f := range out.Findings {
		if f.Path == "" || f.Line == 0 {
			return verdict{Kind: "unlocated", Sig: "unlocated " + shapeOf(f.Code, f.Message), Text: fmt.Sprintf("unlocated: %s %q has no location", f.Code, f.Message)}
		}
	}
	return verdict{}
}

// diagShapes are the findings' shapes, sorted, each once.
func diagShapes(fs []diag.Finding) string {
	var out []string
	for _, f := range fs {
		out = append(out, shapeOf(f.Code, f.Message))
	}
	slices.Sort(out)
	return strings.Join(slices.Compact(out), shapeSep)
}

// nodeAt names where an offset of a parsed file lies: the kinds of its two innermost nodes.
func nodeAt(tree *syntax.File, off int) string {
	path := []string{syntax.KindFile.String()}
	for n := syntax.Node(tree); n != nil; {
		var inner syntax.Node
		for c := range syntax.Children(n) {
			if s := tree.Span(c); int(s.Start) <= off && off < int(s.End) {
				inner = c
				break
			}
		}
		if inner != nil {
			path = append(path, inner.Kind().String())
		}
		n = inner
	}
	return strings.Join(path[max(0, len(path)-contextDepth):], "/")
}

// commonPrefix is the length of the longest prefix a and b share.
func commonPrefix(a, b []byte) int {
	n := 0
	for n < min(len(a), len(b)) && a[n] == b[n] {
		n++
	}
	return n
}

// firstDifference is the first line of a that b does not have at its place.
func firstDifference(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i, line := range la {
		if i >= len(lb) || lb[i] != line {
			return line
		}
	}
	return ""
}

// shapeSig is a line of a shape as a signature: a comment by its tags only, its text being free.
func shapeSig(line string) string {
	if tags, _, ok := strings.Cut(line, `"`); ok && strings.HasPrefix(line, "comment ") {
		return strings.TrimSpace(tags)
	}
	return line
}

func hasError(fs []diag.Finding) bool {
	for _, f := range fs {
		if f.Severity == diag.Error {
			return true
		}
	}
	return false
}

func located(fs []diag.Finding) string {
	parts := make([]string, 0, len(fs))
	for _, f := range fs {
		parts = append(parts, fmt.Sprintf("%s@%d %q", f.Code, f.Span.Start, f.Message))
	}
	return strings.Join(parts, "; ")
}

// formatText is canon fmt's layout of a file of kind.
func formatText(src []byte, kind grammar.Kind) ([]byte, error) {
	fs := &source.FileSet{}
	f, err := fs.Add(kind.File(), "/"+kind.File(), src)
	if err != nil {
		return nil, err
	}
	return format.Source(f, kind.Syntax(), diag.NewBag(fs, ""))
}

// recoverVerdict turns a panic of the parser or the formatter into a verdict.
func recoverVerdict(v *verdict) {
	if r := recover(); r != nil {
		first := fmt.Sprint(r)
		sig := kindPanic + " " + unplaced(first) + " in " + compilerFrames(string(debug.Stack()))
		*v = verdict{Kind: kindPanic, Sig: sig, Text: "panic " + first}
	}
}

// programFailure is a generated file that failed its suite's property.
type programFailure struct {
	suite, prop string
	kind        grammar.Kind
	k           int
	seed        uint64
	src         []byte
	v           verdict
}

// reportProgram reports a failing generated file, shrunk and kept under -progen.keep when its
// signature is new.
func reportProgram(t *testing.T, f programFailure) {
	t.Helper()
	if reported(t, f.suite, f.prop, f.seed, f.v) {
		return
	}
	check := func(src []byte) verdict { return roundTrip(src, false, f.kind) }
	if f.suite == suiteCorrupt {
		check = func(src []byte) verdict { return corruption(src, f.kind) }
	}
	src, _ := progen.ShrinkText(f.src, nil, func(text []byte, _ []progen.Region) bool {
		heartbeat()
		return check(text).Sig == f.v.Sig
	}, shrinkTries)
	files := progen.NewProject()
	files.Set(f.kind.File(), src)
	v := check(src)
	c := &progen.Counterexample{Suite: f.suite, Name: f.prop, Case: f.k, Seed: f.seed, Sig: v.Sig, Want: f.prop, Note: v.Text, Files: files}
	report(t, c, v)
}

// replayProgram re-runs a kept grammar or corruption counterexample; Kind "" when it passes.
func replayProgram(c *progen.Counterexample) verdict {
	kind := archiveKind(c.Files)
	src, _ := c.Files.Get(kind.File())
	if c.Suite == suiteCorrupt {
		return corruption(src, kind)
	}
	return roundTrip(src, false, kind)
}

// archiveKind is the kind of file an archive holds: the one of its files that is not the
// source file, else the source file, else project.canon alone.
func archiveKind(files *progen.Project) grammar.Kind {
	for _, k := range []grammar.Kind{grammar.Layer, grammar.Translation, grammar.Source} {
		if _, ok := files.Get(k.File()); ok {
			return k
		}
	}
	return grammar.Project
}

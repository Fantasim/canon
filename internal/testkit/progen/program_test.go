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
		src := generate(seed)
		if v := roundTrip(src, false); v.Kind != "" {
			reportProgram(t, programFailure{suiteGrammar, propRoundTrip, i, seed, src, v})
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
		src := grammar.Corrupt(r, grammar.Generate(r, progen.NewBudget(genSize, genDepth)))
		if v := corruption(src); v.Kind != "" {
			reportProgram(t, programFailure{suiteCorrupt, propCorruption, i, seed, src, v})
		}
	}
}

func generate(seed uint64) []byte {
	return grammar.Generate(progen.NewRand(seed), progen.NewBudget(genSize, genDepth))
}

// roundTrip is "" for a file that parses clean (warnings aside, when allowed), formats, reparses
// to its shape, is a fixed point of the formatter and checks without a crash; else the verdict.
func roundTrip(src []byte, warnings bool) (v verdict) {
	defer recoverVerdict(&v)
	tree, findings := grammar.Parse(genFile, src)
	if len(findings) > 0 && (!warnings || hasError(findings)) {
		return verdict{Kind: "unparsed", Sig: "unparsed " + diagShapes(findings), Text: "unparsed: a generated file has findings: " + located(findings)}
	}
	out, err := formatText(src)
	if err != nil {
		return verdict{Kind: "unformatted", Sig: "unformatted", Text: "unformatted: " + err.Error()}
	}
	again, reparsed := grammar.Parse(genFile, out)
	switch {
	case hasError(reparsed) || len(reparsed) > len(findings):
		where := nodeAt(again, int(reparsed[0].Span.Start))
		return verdict{Kind: "reparse", Sig: "reparse " + diagShapes(reparsed) + " in " + where, Text: "reparse: the formatted file has findings: " + located(reparsed) + "\n" + string(out)}
	case grammar.Shape(tree) != grammar.Shape(again):
		return verdict{Kind: "shape", Sig: "shape " + shapeSig(firstDifference(grammar.Shape(tree), grammar.Shape(again))), Text: "shape: formatting changed the tree:\n" + string(out)}
	}
	if twice, err := formatText(out); err != nil || !bytes.Equal(twice, out) {
		where := nodeAt(again, commonPrefix(out, twice))
		return verdict{Kind: "idempotence", Sig: "idempotence in " + where, Text: "idempotence: formatting the formatted file changes it:\n" + string(twice)}
	}
	return checked(src)
}

// corruption is "" for a file that does not crash the parser, the formatter or the compiler,
// and gets a located error when it does not parse; a file that parses must round-trip.
func corruption(src []byte) (v verdict) {
	defer recoverVerdict(&v)
	tree, findings := grammar.Parse(genFile, src)
	if !hasError(findings) {
		return roundTrip(src, true)
	}
	for _, f := range findings {
		if f.Span.File != tree.Src.ID || f.Span.Start > f.Span.End || int(f.Span.End) > len(tree.Src.Content) {
			return verdict{Kind: "unlocated", Sig: "unlocated " + shapeOf(f.Code, f.Message), Text: fmt.Sprintf("unlocated: %s has span %+v outside the file", f.Code, f.Span)}
		}
	}
	if _, err := formatText(src); !errors.Is(err, format.ErrSyntax) {
		return verdict{Kind: "formatted", Sig: "formatted", Text: fmt.Sprintf("formatted: a file with syntax errors formats: %v", err)}
	}
	return checked(src)
}

// checked runs the compiler on the file as the one package of a project: no panic, no internal
// error, and every finding located.
func checked(src []byte) verdict {
	p := progen.NewProject()
	p.Set(projectFile, []byte(genProject))
	p.Set(genFile, src)
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
	return strings.Join(slices.Compact(out), ", ")
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

// formatText is canon fmt's layout of a source file.
func formatText(src []byte) ([]byte, error) {
	fs := &source.FileSet{}
	f, err := fs.Add(genFile, "/"+genFile, src)
	if err != nil {
		return nil, err
	}
	return format.Source(f, syntax.FileSource, diag.NewBag(fs, ""))
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
	check := func(src []byte) verdict { return roundTrip(src, false) }
	if f.suite == suiteCorrupt {
		check = corruption
	}
	src, _ := progen.ShrinkText(f.src, nil, func(text []byte, _ []progen.Region) bool {
		heartbeat()
		return check(text).Sig == f.v.Sig
	}, shrinkTries)
	files := progen.NewProject()
	files.Set(genFile, src)
	v := check(src)
	c := &progen.Counterexample{Suite: f.suite, Name: f.prop, Case: f.k, Seed: f.seed, Sig: v.Sig, Want: f.prop, Note: v.Text, Files: files}
	report(t, c, v)
}

// replayProgram re-runs a kept grammar or corruption counterexample; Kind "" when it passes.
func replayProgram(c *progen.Counterexample) verdict {
	src, _ := c.Files.Get(genFile)
	if c.Suite == suiteCorrupt {
		return corruption(src)
	}
	return roundTrip(src, false)
}

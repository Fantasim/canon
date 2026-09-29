package testkit

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

const (
	ruleIDPattern = `[A-Z][0-9]+[a-z]?\b`
	// A citation: `API.md`, an optional `§4.1`, ids joined by `,` or `and`; a `-`, `..` or `…` after the last is a range.
	citePattern = `API\.md(?:\s+§[0-9.]+)?[\s,:]*(` + ruleIDPattern +
		`(?:(?:\s*,\s*|\s+and\s+)(?:§[0-9.]+\s+)?` + ruleIDPattern + `)*)(-|–|\.\.|…)?`
	citeMarker    = "API.md"
	knownBugTag   = "knownbug"
	testFileSufx  = "_test.go"
	txtarSufx     = ".txtar"
	testdataDir   = "testdata"
	testFuncStart = "Test"
	exampleStart  = "Example"
	fuzzStart     = "Fuzz"
)

// skippedTop are the directories of the module root that are not scanned: generated goldens, the
// spec itself, real data, tools. Hidden directories (.git, .claude worktrees) are skipped anywhere.
var skippedTop = []string{"tools", "testdata-real", "spec", "meta", "examples"}

// harnessDirs are the golden directories a harness reads; a txtar citing a rule elsewhere proves nothing.
var harnessDirs = []string{
	"internal/build/testdata/incremental", "internal/check/testdata/findings",
	"internal/cli/testdata/commands", "internal/cli/testdata/watch", "internal/diag/testdata/render",
	"internal/edit/testdata/edits", "internal/eval/testdata/findings", "internal/project/testdata/findings",
	"internal/rules/testdata/findings", "internal/verify/testdata/findings",
}

// citations are the rule ids a source cites, and the range-form citations it wrote.
type citations struct {
	ids    []string
	ranges []string
}

var (
	citeRE  = regexp.MustCompile(citePattern)
	identRE = regexp.MustCompile(ruleIDPattern)
)

func (c *citations) add(text string) {
	for _, m := range citeRE.FindAllStringSubmatch(text, -1) {
		if m[2] != "" {
			c.ranges = append(c.ranges, m[0])
			continue
		}
		c.ids = append(c.ids, identRE.FindAllString(m[1], -1)...)
	}
}

// goCitations reads the citations of a Go test file: the doc comment and body of its Test, Example
// and Fuzz functions only. A file whose build constraint names knownbug does not run, so it counts
// for nothing.
func goCitations(name string, src []byte) (citations, error) {
	var c citations
	if !bytes.Contains(src, []byte(citeMarker)) {
		return c, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		return c, fmt.Errorf("parse %s: %w", name, err)
	}
	if knownBug(file, fset) {
		return c, nil
	}
	for _, d := range file.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || !isTestFunc(fn.Name.Name) {
			continue
		}
		start := fn.Pos()
		if fn.Doc != nil {
			start = fn.Doc.Pos()
		}
		c.add(string(src[fset.Position(start).Offset:fset.Position(fn.End()).Offset]))
	}
	return c, nil
}

func isTestFunc(name string) bool {
	return strings.HasPrefix(name, testFuncStart) || strings.HasPrefix(name, exampleStart) || strings.HasPrefix(name, fuzzStart)
}

// knownBug reports whether a `//go:build` line before the package clause requires knownbug: the
// constraint holds with the tag and fails without it (`!knownbug` does not skip a file).
func knownBug(file *ast.File, fset *token.FileSet) bool {
	for _, group := range file.Comments {
		if fset.Position(group.Pos()).Offset > fset.Position(file.Package).Offset {
			return false
		}
		for _, line := range group.List {
			if constraint.IsGoBuild(line.Text) && requiresKnownBug(line.Text) {
				return true
			}
		}
	}
	return false
}

func requiresKnownBug(line string) bool {
	expr, err := constraint.Parse(line)
	if err != nil {
		return false
	}
	with := func(tag string) bool { return true }
	without := func(tag string) bool { return tag != knownBugTag }
	return expr.Eval(with) && !expr.Eval(without)
}

// txtarCitations reads the citations of a txtar file: its comment, the text before the first file.
func txtarCitations(src []byte) citations {
	var c citations
	c.add(string(txtar.Parse(src).Comment))
	return c
}

// scanRepo lists, by file, what every test and golden of the repository cites.
func scanRepo(t *testing.T) map[string]citations {
	found := map[string]citations{}
	err := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(moduleRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			return skipDir(rel, d.Name())
		}
		c, err := fileCitations(rel, path)
		if err != nil {
			return err
		}
		if len(c.ids)+len(c.ranges) > 0 {
			found[rel] = c
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// skipDir skips hidden directories anywhere and the skipped ones of the module root.
func skipDir(rel, name string) error {
	if rel != "." && (strings.HasPrefix(name, ".") || slices.Contains(skippedTop, rel)) {
		return fs.SkipDir
	}
	return nil
}

// fileCitations reads one file when it is a Go test file, or a txtar under a testdata directory.
func fileCitations(rel, path string) (citations, error) {
	isTxtar := strings.HasSuffix(rel, txtarSufx) && slices.Contains(strings.Split(rel, "/"), testdataDir)
	if !isTxtar && !strings.HasSuffix(rel, testFileSufx) {
		return citations{}, nil
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return citations{}, err
	}
	if isTxtar {
		c := txtarCitations(src)
		if dir := pathpkg.Dir(rel); len(c.ids)+len(c.ranges) > 0 && !slices.Contains(harnessDirs, dir) {
			return c, fmt.Errorf("%s cites a rule but %s is not a directory a harness reads (harnessDirs)", rel, dir)
		}
		return c, nil
	}
	return goCitations(rel, src)
}

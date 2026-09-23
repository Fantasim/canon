package diagnostics

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
	"github.com/fantasim/canonlang/tools/audit/internal/threshold"
)

// runFixture runs the lane on testdata/repo through the real runner, every rule on.
func runFixture(t *testing.T) ([]finding.Finding, []lane.Skip) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", "repo"))
	if err != nil {
		t.Fatal(err)
	}
	var rel []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, repo.GoExt) {
			r, _ := filepath.Rel(root, p)
			rel = append(rel, filepath.ToSlash(r))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(rel)
	tree, perrs := gosrc.Parse(root, rel)
	if len(perrs) > 0 {
		t.Fatal(perrs)
	}
	on := map[string]bool{}
	for _, id := range ruleIDs {
		on[id] = true
	}
	ctx := &lane.Context{
		Repo: &repo.Repo{Root: root, Name: "canon", Module: "example.com/canon"}, Go: tree, Enabled: on,
		Limits: threshold.Set{DiagTextWords: 4},
	}
	return lane.Run(ctx, []lane.Lane{New()})
}

func TestFixtureFindings(t *testing.T) {
	fs, skips := runFixture(t)
	for _, s := range skips {
		t.Errorf("skip: %+v", s)
	}
	var got strings.Builder
	for _, f := range fs {
		fmt.Fprintf(&got, "%s\t%s\t%d\t%s\t%s\n", f.Rule, f.File, f.Line, f.Symbol, f.Detail)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "want.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Fatalf("findings differ from testdata/want.tsv\n got:\n%s\nwant:\n%s", got.String(), want)
	}
}

func TestFixedRuns(t *testing.T) {
	got := fixedRuns(`{{{name}}} reads {{field}}\nthen {a} and \\ end`)
	want := []string{"{", "} reads {field}", "then ", ` and \ end`}
	if !slices.Equal(got, want) {
		t.Fatalf("fixedRuns = %q, want %q", got, want)
	}
}

func TestSegmentsKeepLongRunsOnce(t *testing.T) {
	c := &catalogue{templates: []template{
		{code: "a", text: `unknown project key "{key}"`},
		{code: "b", text: `{kind} {name} has no doc comment`},
		{code: "c", text: `{name} has no doc comment`},
	}}
	got := c.segments(4)
	if len(got) != 1 || got[0] != (segment{code: "b", text: "has no doc comment"}) {
		t.Fatalf("segments(4) = %+v, want only b's run, once", got)
	}
	if n := len(c.segments(3)); n != 2 {
		t.Fatalf("segments(3) = %d runs, want 2", n)
	}
}

// The project's own catalogue parses to the counts its summary sentence states.
func TestProjectCatalogue(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "..", catalogueFile)
	src, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("no project catalogue next to this tool")
	}
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`The catalogue holds (\d+) codes: .*, with (\d+) messages`).FindStringSubmatch(string(src))
	if m == nil {
		t.Fatal("no count sentence in the catalogue")
	}
	c := parseCatalogue(string(src))
	codes, messages := mustAtoi(t, m[1]), mustAtoi(t, m[2])
	if len(c.codes) != codes || len(c.templates) != messages+runtimeRows(string(src)) {
		t.Fatalf("parsed %d codes and %d templates; the catalogue states %d codes and %d messages", len(c.codes), len(c.templates), codes, messages)
	}
}

// runtimeRows counts the rows of the table of texts signalled by generated code.
func runtimeRows(src string) int {
	_, table, _ := strings.Cut(src, headerRuntime+lineBreak)
	n := 0
	for _, line := range strings.Split(table, lineBreak)[1:] {
		if !strings.HasPrefix(line, tableRow) {
			break
		}
		n++
	}
	return n
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCatalogueFile(t *testing.T) {
	dir := t.TempDir()
	if c, err := readCatalogue(filepath.Join(dir, "none.md")); c != nil || err != nil {
		t.Fatalf("missing catalogue = %v, %v; want nothing to check", c, err)
	}
	empty := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(empty, []byte("# no tables\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readCatalogue(empty); !errors.Is(err, errNoCodes) {
		t.Fatalf("catalogue without codes: error = %v, want errNoCodes", err)
	}
}

package testkit

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const (
	apiSpecPath = "../../spec/API.md"
	scanDataDir = "testdata/scan"
	// A rule opens a list item with its bold id: `- **E20.** ...`, or `- **V4a. JSON form.**`.
	ruleOpenPattern = `(?m)^\s*[-*]?\s*\*\*` + ruleIDPattern + `\.`
	ruleDeclPattern = `(?m)^\s*[-*]?\s*\*\*(` + ruleIDPattern + `)\.(?:\s[^*]*)?\*\*`
)

var (
	ruleOpenRE = regexp.MustCompile(ruleOpenPattern)
	ruleDeclRE = regexp.MustCompile(ruleDeclPattern)
)

// IMPLEMENTATION-PLAN.md §7.4: every API.md rule is cited as `API.md <id>` by a test, a golden, and only rules.
func TestEveryAPIRuleHasATest(t *testing.T) {
	spec, err := os.ReadFile(apiSpecPath)
	if err != nil {
		t.Fatal(err)
	}
	rules := declaredRules(t, string(spec))
	found := scanRepo(t)
	cited := map[string]bool{}
	for _, file := range sortedFiles(found) {
		c := found[file]
		for _, r := range c.ranges {
			t.Errorf("%s: %q is a range, not a citation: list the ids each test proves", file, r)
		}
		for _, id := range c.ids {
			cited[id] = true
			if !slices.Contains(rules, id) {
				t.Errorf("%s cites API.md %s, which API.md does not declare", file, id)
			}
		}
	}
	var missing []string
	for _, id := range rules {
		if !cited[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d of %d API.md rules have no test citing them: %s", len(missing), len(rules), strings.Join(missing, " "))
	}
}

func sortedFiles(found map[string]citations) []string {
	var files []string
	for file := range found { //canon:unordered the keys are sorted below
		files = append(files, file)
	}
	slices.Sort(files)
	return files
}

// declaredRules lists the rule ids API.md declares, in document order, each once. Every line that
// opens with a bold id and a dot must be a rule the declaration form reads.
func declaredRules(t *testing.T, spec string) []string {
	t.Helper()
	var ids []string
	for _, m := range ruleDeclRE.FindAllStringSubmatch(spec, -1) {
		if slices.Contains(ids, m[1]) {
			t.Errorf("API.md declares %s twice", m[1])
			continue
		}
		ids = append(ids, m[1])
	}
	if openers := len(ruleOpenRE.FindAllString(spec, -1)); openers != len(ids) {
		t.Errorf("API.md has %d rule openers but %d ids were read", openers, len(ids))
	}
	if len(ids) == 0 {
		t.Fatal("no rule found in API.md")
	}
	return ids
}

// IMPLEMENTATION-PLAN.md §7.4: a citation counts in a Test, Example or Fuzz function or a golden's comment only.
func TestCitationScanner(t *testing.T) {
	cases := []struct {
		file   string
		ids    []string
		ranges int
	}{
		{"var_only.go.txt", nil, 0},
		{"counted.go.txt", []string{"E1", "E2", "E3", "M6", "V4a", "N1", "N2"}, 0},
		{"tagged.go.txt", nil, 0},
		{"range.go.txt", nil, 4},
		{"negated.go.txt", []string{"E1"}, 0},
		{"tagged_and.go.txt", nil, 0},
		{"comment.txtar.txt", []string{"E1", "E2"}, 0},
	}
	for _, c := range cases {
		src, err := os.ReadFile(filepath.Join(scanDataDir, c.file))
		if err != nil {
			t.Fatal(err)
		}
		var got citations
		if strings.Contains(c.file, ".txtar.") {
			got = txtarCitations(src)
		} else if got, err = goCitations(c.file, src); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got.ids, c.ids) || len(got.ranges) != c.ranges {
			t.Errorf("%s: cited %v with %d ranges, want %v with %d", c.file, got.ids, len(got.ranges), c.ids, c.ranges)
		}
	}
}

// IMPLEMENTATION-PLAN.md §7.4: each directory of harnessDirs is named by a test file of its package.
func TestHarnessDirsAreRead(t *testing.T) {
	for _, dir := range harnessDirs {
		pkg, sub, _ := strings.Cut(dir, "/testdata/")
		tests, err := filepath.Glob(filepath.Join(moduleRoot, pkg, "*"+testFileSufx))
		if err != nil {
			t.Fatal(err)
		}
		named := regexp.MustCompile(`testdata["/,\s]+` + regexp.QuoteMeta(sub))
		read := slices.ContainsFunc(tests, func(name string) bool {
			src, err := os.ReadFile(name)
			return err == nil && named.Match(src)
		})
		if !read {
			t.Errorf("no test of %s names testdata/%s", pkg, sub)
		}
	}
}

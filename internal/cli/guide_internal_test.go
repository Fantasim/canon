package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// A fence's info string is `<lang> [arg] [warns=CODE,...]`: `canon` alone is a whole file,
// `canon fragment` is not checked, `<lang> <path>` is a file of the topic's project; `warns=`
// names the warnings that file demonstrates on purpose, the only ones its project may report.
const (
	fenceMark      = "```"
	fragment       = "fragment"
	projectFile    = "project.canon"
	defaultProject = "project guide {\n  canon: \"0.1\"\n  languages: [en, fr]\n}\n"
	looseFile      = "guide.canon"
	reqDir         = "req/" // recipes' request files, written into every project a recipe runs in
	warnsMark      = "warns="
	codeSep        = ","
)

// fenceLangs are the info words a guide may use: anything else is a typo the tests would skip.
var fenceLangs = []string{"canon", "json", "csv", "text", "c", "sh"}

// guideFence is one fenced block of a topic.
type guideFence struct {
	topic, lang, arg, body string
	warns                  []string
	line                   int
}

func (f guideFence) String() string { return f.topic + guideExt + ":" + strconv.Itoa(f.line) }

// guideTexts is every embedded topic's text, by topic.
func guideTexts(t *testing.T) map[string]string {
	t.Helper()
	topics, err := guideTopics()
	if err != nil || len(topics) == 0 {
		t.Fatalf("topics %v: %v", topics, err)
	}
	out := map[string]string{}
	for _, topic := range topics {
		data, err := guideFS.ReadFile(guideDir + pathSep + topic + guideExt)
		if err != nil {
			t.Fatal(err)
		}
		out[topic] = string(data)
	}
	return out
}

// fencesOf splits a topic's fenced blocks out of its text.
func fencesOf(topic, text string) []guideFence {
	var out []guideFence
	var cur *guideFence
	var body []string
	for i, l := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(l, fenceMark) && cur == nil:
			info := strings.Fields(strings.TrimPrefix(l, fenceMark))
			cur = &guideFence{topic: topic, line: i + 1}
			if len(info) > 0 {
				cur.lang = info[0]
			}
			if len(info) > 1 {
				cur.arg = info[1]
			}
			if len(info) > 2 {
				cur.warns = strings.Split(strings.TrimPrefix(info[2], warnsMark), codeSep)
			}
			body = nil
		case strings.HasPrefix(l, fenceMark):
			cur.body = strings.Join(body, "\n") + "\n"
			out = append(out, *cur)
			cur = nil
		case cur != nil:
			body = append(body, l)
		}
	}
	return out
}

func allFences(t *testing.T) []guideFence {
	t.Helper()
	var out []guideFence
	texts := guideTexts(t)
	for _, topic := range sortedKeys(texts) {
		out = append(out, fencesOf(topic, texts[topic])...)
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// DECISIONS 276, CLI.md §3.17: the index lists exactly the embedded topics, one row each.
func TestGuideIndexListsTopics(t *testing.T) {
	texts := guideTexts(t)
	rows := regexp.MustCompile("(?m)^\\| `([a-z0-9-]+)` \\|").FindAllStringSubmatch(texts[guideIndex], -1)
	var listed []string
	for _, r := range rows {
		listed = append(listed, r[1])
	}
	var want []string
	for _, topic := range sortedKeys(texts) {
		if topic != guideIndex {
			want = append(want, topic)
		}
	}
	got := slices.Clone(listed)
	slices.Sort(got)
	if !slices.Equal(got, want) || len(listed) != len(want) {
		t.Errorf("index lists %v, embedded topics are %v", listed, want)
	}
	for topic, text := range texts {
		if !strings.HasPrefix(text, "# ") || !strings.HasSuffix(text, "\n") || strings.HasSuffix(text, "\n\n") {
			t.Errorf("%s: a topic starts with a `# ` title and ends with one newline", topic)
		}
	}
}

// DECISIONS 276, SPEC §2.1: every whole Canon file of the guide parses and is canonical.
func TestGuideCanonFilesParse(t *testing.T) {
	for _, f := range allFences(t) {
		if !slices.Contains(fenceLangs, f.lang) {
			t.Errorf("%s: fence language %q is not one of %v", f, f.lang, fenceLangs)
			continue
		}
		if f.lang != "canon" || f.arg == fragment {
			continue
		}
		name := looseFile
		if f.arg != "" {
			name = f.arg
		}
		out, err := canon.Format(name, []byte(f.body))
		switch {
		case err != nil:
			t.Errorf("%s: %v", f, err)
		case string(out) != f.body:
			t.Errorf("%s: not in canonical layout; canon fmt gives:\n%s", f, out)
		}
	}
}

// guideProjects groups each topic's files (fences with a path) into one project, by topic.
func guideProjects(t *testing.T) map[string]map[string]string {
	t.Helper()
	out := map[string]map[string]string{}
	for _, f := range allFences(t) {
		if f.arg == "" || f.arg == fragment || f.lang == "sh" || strings.HasPrefix(f.arg, reqDir) {
			continue
		}
		if out[f.topic] == nil {
			out[f.topic] = map[string]string{projectFile: defaultProject}
		}
		if _, twice := out[f.topic][f.arg]; twice && f.arg != projectFile {
			t.Errorf("%s: %s is given twice", f, f.arg)
		}
		out[f.topic][f.arg] = f.body
	}
	return out
}

// guideWarns are, by topic, the warnings its fences demonstrate on purpose (`warns=`).
func guideWarns(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, f := range allFences(t) {
		out[f.topic] = append(out[f.topic], f.warns...)
	}
	return out
}

// DECISIONS 276, CLI.md §3.3-§3.5: each topic's files check without findings, test green and build.
func TestGuideProjectsCheck(t *testing.T) {
	warns := guideWarns(t)
	for topic, files := range guideProjects(t) {
		t.Run(topic, func(t *testing.T) {
			checkGuideProject(t, writeFiles(t, files), warns[topic])
		})
	}
}

func checkGuideProject(t *testing.T, dir string, warns []string) {
	t.Helper()
	ctx := context.Background()
	p, err := canon.Open(dir, canon.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	res, err := p.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	reportFindings(t, "check", res.Findings, warns)
	for _, code := range warns {
		if !slices.ContainsFunc(res.Findings, func(f canon.Finding) bool { return f.Code == code }) {
			t.Errorf("warns=%s is marked but not reported", code)
		}
	}
	tests, err := p.Test(ctx, canon.TestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range tests.Tests {
		if !c.Passed {
			t.Errorf("test %q fails: %+v", c.Name, c.Failures)
		}
	}
	out, err := p.Build(ctx, canon.BuildOptions{Check: true})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	reportFindings(t, "build", out.Check.Findings, warns)
}

// reportFindings fails on every finding but a warning the topic marks with `warns=`.
func reportFindings(t *testing.T, what string, findings []canon.Finding, warns []string) {
	t.Helper()
	for _, f := range findings {
		if f.Severity == canon.SeverityError || !slices.Contains(warns, f.Code) {
			line, _ := json.Marshal(f)
			t.Errorf("%s: %s", what, line)
		}
	}
}

// writeFiles writes files, by slash path, into a new directory and returns it.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// DECISIONS 276: each recipe's `sh <topic>` steps run, in order, on that topic's project with the
// recipes' request files, and every step exits 0.
func TestGuideRecipesRun(t *testing.T) {
	projects := guideProjects(t)
	requests := map[string]string{}
	var runs []guideFence
	for _, f := range allFences(t) {
		switch {
		case strings.HasPrefix(f.arg, reqDir):
			requests[f.arg] = f.body
		case f.lang == "sh" && f.arg != "":
			runs = append(runs, f)
		}
	}
	if len(runs) == 0 {
		t.Fatal("no recipe to run")
	}
	for _, f := range runs {
		if projects[f.arg] == nil {
			t.Errorf("%s: no project for topic %q", f, f.arg)
			continue
		}
		files := maps.Clone(projects[f.arg])
		maps.Copy(files, requests)
		runRecipe(t, f, writeFiles(t, files))
	}
}

// runRecipe runs each line of a recipe as a command line in dir; quotes are the shell's.
func runRecipe(t *testing.T, f guideFence, dir string) {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(f.body), "\n") {
		words := strings.Fields(line)
		for i, w := range words {
			words[i] = strings.Trim(w, "'")
		}
		var out, errs bytes.Buffer
		env := Env{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errs, Dir: dir}
		if code := Main(context.Background(), words[1:], env); code != exitOK {
			t.Errorf("%s: %q: exit %d\n%s%s", f, line, code, out.String(), errs.String())
		}
	}
}

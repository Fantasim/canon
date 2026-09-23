package gorules

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

type fixture struct{ name, module string }

var (
	svcFixture = fixture{"svc", "example.com/svc"}
	libFixture = fixture{"lib", "example.com/lib"}
)

// runFixture runs the lane on testdata/modules/<name> with every rule on, naming symbols the
// way the runner does.
func runFixture(t *testing.T, fx fixture) ([]finding.Finding, []lane.Skip) {
	t.Helper()
	modules, err := filepath.Abs(filepath.Join("testdata", "modules"))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(modules, fx.name)
	var rel []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, ".go") {
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
	r := &repo.Repo{Root: root, Name: fx.name, Module: fx.module}
	on := map[string]bool{}
	for _, id := range ruleIDs {
		if rl, ok := rules.Lookup(id); ok && r.Mode(*rl) != rules.Off {
			on[id] = true
		}
	}
	res, err := New().Run(&lane.Context{Repo: r, Go: tree, Enabled: on})
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range res.Findings {
		if f.Symbol == "" && strings.HasSuffix(f.File, ".go") {
			res.Findings[i].Symbol = tree.Enclosing(f.File, f.Line)
		}
	}
	return res.Findings, res.Skipped
}

func keys(fs []finding.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, fmt.Sprintf("%s|%s|%s|%s|%d", f.Rule, f.File, f.Symbol, f.Detail, f.Value))
	}
	slices.Sort(out)
	return out
}

func TestFixtureFindings(t *testing.T) {
	cases := []struct {
		fx    fixture
		want  []string
		skips []string
	}{
		{svcFixture, svcWant, []string{ruleImportBound}},
		{libFixture, libWant, nil},
	}
	for _, c := range cases {
		t.Run(c.fx.name, func(t *testing.T) {
			fs, skips := runFixture(t, c.fx)
			got := keys(fs)
			for _, g := range got {
				if !slices.Contains(c.want, g) {
					t.Errorf("unexpected finding %s", g)
				}
			}
			for _, w := range c.want {
				if !slices.Contains(got, w) {
					t.Errorf("missing finding %s", w)
				}
			}
			var what []string
			for _, s := range skips {
				what = append(what, s.What)
			}
			if !slices.Equal(what, c.skips) {
				t.Errorf("skips = %v, want %v", skips, c.skips)
			}
		})
	}
}

func TestFixtureFixes(t *testing.T) {
	cases := []struct {
		fx                 fixture
		rule, file, detail string
		fix, message       string
	}{
		{svcFixture, ruleMagicString, "internal/api", "GET", "use http.MethodGet", `"GET" x2 here`},
		{svcFixture, ruleMagicString, "internal/api", "only-here", "name it in internal/api/constants.go", ""},
		{svcFixture, ruleMagicString, "internal/db", "application/json", fixConstShared, "x3 in 2 packages"},
		{svcFixture, ruleMagicString, "internal/api", `^([0-9]{4})_([a-z0-9_]+)\.sql$`, "name it in internal/api/constants.go", ""},
		{svcFixture, ruleMagicNumber, "internal/api/handlers.go", "5", "name it in internal/api/constants.go", `"5 * b"`},
		{svcFixture, ruleConstDup, "internal/db/constants.go", "user", "keep one: kindUser in internal/api/constants.go", ""},
		{svcFixture, ruleEnvKey, "internal/config/config.go", "Getenv SVC_DEBUG", "name the key in internal/config/constants.go", ""},
		{svcFixture, ruleEnvKey, "internal/api/handlers.go", "Getenv config.EnvPort", fixEnvOutside, ""},
		{svcFixture, ruleExportedLocal, "internal/api/handlers.go", "func", "unexport it (rename to helper)", ""},
		{svcFixture, ruleExportedLocal, "internal/api/constants.go", "const", "unexport it (rename to maxThing)", ""},
		{svcFixture, ruleStdlib, "internal/api/stdlib.go", "map keys m", "use slices.Sorted(maps.Keys(m))", "keys of m"},
		{svcFixture, ruleStdlib, "internal/api/stdlib.go", patMax, "use the builtin max", "maxOf"},
		{libFixture, ruleMagicString, "geoip", `^([0-9]{4})_([a-z0-9_]+)\.sql$`, "name it in geoip/constants.go", ""},
		{libFixture, ruleEnvKey, "geoip/geoip.go", "Getenv envPath", fixEnvOutside, ""},
		{libFixture, ruleBareGo, "geoip/geoip.go", "Path", fixBareGo, ""},
	}
	found := map[fixture][]finding.Finding{}
	for _, c := range cases {
		if _, ok := found[c.fx]; !ok {
			found[c.fx], _ = runFixture(t, c.fx)
		}
		i := slices.IndexFunc(found[c.fx], func(f finding.Finding) bool {
			return f.Rule == c.rule && f.File == c.file && f.Detail == c.detail && strings.HasPrefix(f.Fix, c.fix)
		})
		if i < 0 {
			t.Errorf("%s %s %q: no finding with fix %q", c.rule, c.file, c.detail, c.fix)
			continue
		}
		if msg := found[c.fx][i].Message; !strings.Contains(msg, c.message) {
			t.Errorf("%s %s %q: message %q lacks %q", c.rule, c.file, c.detail, msg, c.message)
		}
	}
}

func TestExportedDupHasNoRename(t *testing.T) {
	fs, _ := runFixture(t, svcFixture)
	i := slices.IndexFunc(fs, func(f finding.Finding) bool { return f.Rule == ruleExportedLocal && f.Symbol == "Dup" })
	if i < 0 || fs[i].Fix != "" {
		t.Fatalf("Dup collides with dup: want the rulebook's fix, got %+v", fs)
	}
}

func TestTypedLoadFailureSkipsOnlyTypedRule(t *testing.T) {
	fs, skips := runFixture(t, fixture{"broken", "example.com/broken"})
	if len(skips) != 1 || skips[0].What != ruleExportedLocal {
		t.Fatalf("skips = %v, want one exported-but-local skip", skips)
	}
	if !slices.ContainsFunc(fs, func(f finding.Finding) bool { return f.Rule == ruleMagicNumber && f.Detail == "42" }) {
		t.Errorf("syntax rules must still run: %v", keys(fs))
	}
}

func TestNoGoTree(t *testing.T) {
	res, err := New().Run(&lane.Context{Enabled: map[string]bool{ruleMagicString: true}})
	if err != nil || len(res.Findings)+len(res.Skipped) != 0 {
		t.Fatalf("want an empty result, got %+v, %v", res, err)
	}
}

func TestLowerFirst(t *testing.T) {
	cases := map[string]string{
		"Foo": "foo", "HTTPClient": "httpClient", "IDs": "ids", "TOTPURIScheme": "totpURIScheme",
		"ERR_CODE": "err_code", "URL": "url", "GeoIPMin": "geoIPMin", "PGStatusOK": "pgStatusOK", "IPv4": "ipv4",
		"PBKDF2Rounds": "pbkdf2Rounds", "APIV1Prefix": "apiV1Prefix", "UTF8Name": "utf8Name", "E164Pattern": "e164Pattern",
	}
	for in, want := range cases {
		if got := lowerFirst(in); got != want {
			t.Errorf("lowerFirst(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValueClasses(t *testing.T) {
	cases := []struct {
		v             string
		trivial, oneW bool
	}{
		{"x", true, true},
		{"%s: %w", true, false},
		{", ", true, false},
		{"\r\n", true, false},
		{"status", false, true},
		{"not-configured", false, true},
		{"60s", false, true},
		{"2006-01-02", false, false},
		{"6c3f9b2a-1f4b-4a9d-8b2e-5c7d1e0a3f44", false, false},
		{"invalid %s", false, false},
	}
	for _, c := range cases {
		if got := trivialString(c.v); got != c.trivial {
			t.Errorf("trivialString(%q) = %v", c.v, got)
		}
		if got := oneWord(c.v); got != c.oneW {
			t.Errorf("oneWord(%q) = %v", c.v, got)
		}
	}
}

func TestMatchTree(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"internal/api/...", "internal/api", true},
		{"internal/api/...", "internal/api/v1", true},
		{"internal/api/...", "internal/apix", false},
		{"internal/*/...", "internal/db/x", true},
		{"internal/api", "internal/api/v1", false},
		{"os/exec", "os/exec", true},
	}
	for _, c := range cases {
		if got := matchTree(c.pattern, c.path); got != c.want {
			t.Errorf("matchTree(%q, %q) = %v", c.pattern, c.path, got)
		}
	}
}

func TestQuietWithoutTypes(t *testing.T) {
	code := `package p
import "log/slog"
func f(logger *slog.Logger, r router, m map[string]int) {
	slog.Info("a-msg")
	logger.Warn("b-msg")
	r.Get("/c/path", nil)
	_ = map[string]int{"d-key": 1}
	_ = m["e-key"]
	_ = []string{"loud"}
}`
	af, err := parser.ParseFile(token.NewFileSet(), "p.go", code, 0)
	if err != nil {
		t.Fatal(err)
	}
	f := &src{File: &gosrc.File{Path: "p.go", AST: af}, imports: importTable(af)}
	got := map[string]bool{}
	for _, j := range f.judgedLits(nil) {
		if v, ok := unquote(j.lit); ok {
			got[v] = j.quietStr
		}
	}
	want := map[string]bool{"a-msg": true, "b-msg": true, "/c/path": true, "d-key": true, "e-key": false, "loud": false}
	for v, q := range want {
		if got[v] != q {
			t.Errorf("%q quiet = %v, want %v", v, got[v], q)
		}
	}
}

func TestSQLStatement(t *testing.T) {
	for v, want := range map[string]bool{
		"\n\tSELECT id FROM t":               true,
		"/* c */ -- d\nwith x AS (SELECT 1)": true,
		"UPDATE t SET a = $1":                true,
		"SELECT pg_advisory_xact_lock($1)":   true,
		"SELECT ":                            false,
		"DELETE FROM":                        false,
		" ORDER BY id":                       false,
		"id, name, state":                    false,
		"Select a server":                    false,
		"settings":                           false,
	} {
		if got := sqlStatement(v); got != want {
			t.Errorf("sqlStatement(%q) = %v, want %v", v, got, want)
		}
	}
}

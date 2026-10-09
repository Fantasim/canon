package canon_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

const (
	consumerTestProject = "project a {\n  canon: \"0.1\"\n  roots {\n    out: \"out\"\n    admin: \"admin\"\n  }\n  consumer_roots: [admin]\n}\n"
	consumerTestPackage = "/// A.\npackage a\n\n/// V.\nlet v: Int = 1\n\nemit json { out: [\"@out/a/\", \"@admin/a/\"] }\n"
)

// outputPaths is the display path of every output of res.
func outputPaths(res *canon.BuildResult) []string {
	var out []string
	for _, o := range res.Outputs {
		out = append(out, o.Path)
	}
	return out
}

// API.md B1c, DECISIONS 343: a plain Build never writes under a consumer root; Build with
// OnlyRoot writes exactly the outputs under it, and nothing else of the project.
func TestBuildOnlyRoot(t *testing.T) {
	ctx := context.Background()
	fsys := newMemFS(map[string][]byte{"/law/project.canon": []byte(consumerTestProject), "/law/a/a.canon": []byte(consumerTestPackage)})
	p := openTierProject(t, fsys)
	res, err := p.Build(ctx, canon.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := outputPaths(res); !slices.Equal(got, []string{"@out/a/v.json"}) {
		t.Fatalf("plain build: %v", got)
	}
	if _, ok := fsys.files["/law/admin/a/v.json"]; ok {
		t.Fatal("a plain build wrote under the consumer root")
	}
	check, err := p.Build(ctx, canon.BuildOptions{Check: true, OnlyRoot: "admin"})
	if err != nil || !check.Stale || !slices.Equal(outputPaths(check), []string{"@admin/a/v.json"}) {
		t.Fatalf("--only-root --check: %v, stale %t, outputs %v", err, check != nil && check.Stale, outputPaths(check))
	}
	before := maps.Clone(fsys.files)
	res, err = p.Build(ctx, canon.BuildOptions{OnlyRoot: "admin"})
	if err != nil || !slices.Equal(outputPaths(res), []string{"@admin/a/v.json"}) || len(res.Lock) != 0 {
		t.Fatalf("--only-root: %v, outputs %v", err, outputPaths(res))
	}
	for name, data := range fsys.files { //canon:unordered each file judged alone
		if old, ok := before[name]; ok && string(old) != string(data) || !ok && !strings.HasPrefix(name, "/law/admin/") {
			t.Errorf("--only-root changed %s", name)
		}
	}
	if _, ok := fsys.files["/law/admin/a/v.json"]; !ok {
		t.Error("--only-root did not write the consumer root's output")
	}
}

// API.md B1c, V1, DECISIONS 343: an OnlyRoot that is not a consumer root, or given with Adopt, is
// *ValueError, and nothing is written.
func TestBuildOnlyRootRefused(t *testing.T) {
	fsys := newMemFS(map[string][]byte{"/law/project.canon": []byte(consumerTestProject), "/law/a/a.canon": []byte(consumerTestPackage)})
	p := openTierProject(t, fsys)
	for _, c := range []struct {
		opt      canon.BuildOptions
		expected string
	}{
		{canon.BuildOptions{OnlyRoot: "out"}, "a consumer root of the project (consumer_roots)"},
		{canon.BuildOptions{OnlyRoot: "nope", Check: true}, "a consumer root of the project (consumer_roots)"},
		{canon.BuildOptions{OnlyRoot: "admin", Adopt: []string{"@admin/a/v.json"}}, "OnlyRoot without Adopt (--only-root without --adopt)"},
	} {
		_, err := p.Build(context.Background(), c.opt)
		var verr *canon.ValueError
		if !errors.Is(err, canon.ErrBadValue) || !errors.As(err, &verr) || verr.Expected != c.expected {
			t.Errorf("%+v: %v", c.opt, err)
		}
	}
	if _, ok := fsys.files["/law/out/a/v.json"]; ok {
		t.Error("a refused build wrote an output")
	}
}

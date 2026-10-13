package build_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
)

// The 2026-10-12 repro: rounds of a package with an emit text into a new directory, its source
// changed, and builders building it at once.
const (
	concurrentRule = "DECISIONS 351, 352: "
	rounds         = 8
	builders       = 2
)

// raceSource is package a writing t.sql holding content into @out/<dir>, and one JSON value.
func raceSource(dir, content string) string {
	return "/// P.\npackage a\n\n/// V.\nlet v: Int = " + fmt.Sprint(len(content)) + "\n\n/// F.\n@text(\"t.sql\")\nexport fn f() -> String { return \"" + content + "\" }\n\n" +
		"emit text { out: \"@out/" + dir + "\" }\n\nemit json { out: \"@out/" + dir + "/json/\" }\n"
}

// buildOS opens the project at dir on the OS's file system and builds it.
func buildOS(dir string, opt build.BuildOptions) (*build.BuildResult, error) {
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		return nil, err
	}
	return p.Build(context.Background(), opt)
}

// staged lists the staging names left under dir.
func staged(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(d.Name(), ".canon-tmp") {
			out = append(out, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// DECISIONS 351, 352: concurrent builds of a changed package into a new directory end complete and correct (repro: 80/80 lost).
func TestConcurrentBuilds(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "project.canon"), []byte(textProject), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "a"), 0o700); err != nil {
		t.Fatal(err)
	}
	for round := range rounds {
		out, content := fmt.Sprintf("sql%d", round), fmt.Sprintf("v%d", round)
		if err := os.WriteFile(filepath.Join(dir, "a", "a.canon"), []byte(raceSource(out, content)), 0o600); err != nil {
			t.Fatal(err)
		}
		errs := make(chan error, builders)
		for range builders {
			go func() {
				res, err := buildOS(dir, build.BuildOptions{})
				if err == nil && res.Summary.Errors != 0 {
					err = fmt.Errorf("findings %v", codes(res.List))
				}
				errs <- err
			}()
		}
		for range builders {
			if err := <-errs; err != nil {
				t.Fatalf("%sround %d: %v", concurrentRule, round, err)
			}
		}
		checkRound(t, dir, out, content)
		if _, err := os.Stat(filepath.Join(dir, "out", fmt.Sprintf("sql%d", round-1), "t.sql")); round > 0 && err == nil {
			t.Errorf("%sround %d: the previous round's t.sql stayed", concurrentRule, round)
		}
	}
}

// checkRound fails unless out holds t.sql with content and the JSON value, canon.outputs lists
// it, and nothing staged is left.
func checkRound(t *testing.T, dir, out, content string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(dir, "out", out, "t.sql"))
	if err != nil || string(got) != content {
		t.Errorf("%s%s/t.sql holds %q (%v)", concurrentRule, out, got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out", out, "json", "v.json")); err != nil {
		t.Errorf("%s%s/json/v.json: %v", concurrentRule, out, err)
	}
	listing, err := os.ReadFile(filepath.Join(dir, "a", "canon.outputs"))
	if err != nil || string(listing) != outputsFor("a", map[string]string{"@out/" + out + "/t.sql": content}) {
		t.Errorf("%scanon.outputs %q (%v)", concurrentRule, listing, err)
	}
	if left := staged(t, dir); len(left) > 0 {
		t.Errorf("%sstaged files left: %v", concurrentRule, left)
	}
}

// writeFiles writes files, name to text, under dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files { //canon:unordered each file is written on its own
		abs := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil || os.WriteFile(abs, []byte(text), 0o600) != nil {
			t.Fatal(name, err)
		}
	}
}

// sharedPkg is package name writing its C++ into @out/shared/, whose runtime file others write too.
func sharedPkg(name string) string {
	return "/// P.\npackage " + name + "\n\n/// V.\nlet v: Int = 1\n\nemit cpp { out: \"@out/shared/\" }\n"
}

// DECISIONS 351, 352: under its exclusive locks a build removes the stages a dead run left beside
// its outputs, any token, and the OS's temporary file of one; 0.1.5's fixed name, and a stage beside a runtime file a package the
// build holds no lock on also writes, stay.
func TestDeadStagesSwept(t *testing.T) {
	dir := t.TempDir()
	dead, fixed, runtime := "out/shared/.a.gen.h.ZZZZZZZZZZZZZ.canon-tmp", "out/shared/.a.gen.h.canon-tmp", "out/shared/.canon_runtime.h.ZZZZZZZZZZZZZ.canon-tmp"
	writeFiles(t, dir, map[string]string{
		"project.canon": textProject, "a/a.canon": sharedPkg("a"), "b/b.canon": sharedPkg("b"),
		dead: "half", fixed: "old run", runtime: "half", "out/shared/..a.gen.h.ZZZZZZZZZZZZZ.canon-tmp.YYYYYYYYYYYYY.canon-tmp": "half",
	})
	for _, c := range []struct {
		pkgs []string
		left []string
	}{
		{[]string{"a"}, []string{fixed, runtime}},
		{nil, []string{fixed}},
	} {
		res, err := buildOS(dir, build.BuildOptions{Packages: c.pkgs})
		if err != nil || res.Summary.Errors != 0 {
			t.Fatalf("%s%v %v", concurrentRule, err, res)
		}
		var want []string
		for _, l := range c.left {
			want = append(want, filepath.Join(dir, filepath.FromSlash(l)))
		}
		if got := staged(t, dir); !slices.Equal(got, want) {
			t.Errorf("%sbuild %v left %v, want %v", concurrentRule, c.pkgs, got, want)
		}
	}
}

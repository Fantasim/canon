package build_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

const (
	consumerProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    out: \"out\"\n    admin: \"../admin\"\n  }\n  consumer_roots: [admin]\n}\n"
	consumerSource  = "/// A.\npackage a\n\n/// V.\nlet v: Int = 1\n\n/// Tier.\nrecord Tier {\n  /// Weight.\n  weight: Int = 1\n}\n\n" +
		"/// Tiers.\nlet tiers: stable table Tier = { low {} }\n\n/// The note.\n@text(\"note.txt\")\nexport fn note() -> String { return \"hi\" }\n\n" +
		"emit json { out: [\"@out/\", \"@admin/json/\"] }\n\nemit text { out: \"@out/notes\" }\n"
	adminDir = "admin/"
)

// consumerTree is a project whose json copy goes to the consumer root @admin, present.
func consumerTree() mapFS {
	return mapFS{"p/project.canon": file(consumerProject), "p/a/a.canon": file(consumerSource), adminDir + "README": file("")}
}

// files is every file of fsys with its content.
func files(fsys mapFS) map[string]string {
	out := map[string]string{}
	for name, f := range fsys { //canon:unordered a map copied
		out[name] = string(f.Data)
	}
	return out
}

// DECISIONS 343: a plain build writes all but the consumer root's outputs, never listed.
func TestConsumerPlainBuild(t *testing.T) {
	fsys := consumerTree()
	res := buildTree(t, fsys, build.BuildOptions{})
	if len(res.List) != 0 {
		t.Fatalf("findings %v", res.List)
	}
	for _, o := range res.Outputs {
		if strings.HasPrefix(o.Path, "@admin/") {
			t.Errorf("plain build output %s", o.Path)
		}
	}
	for name := range fsys { //canon:unordered each name judged alone
		if strings.HasPrefix(name, adminDir) && name != adminDir+"README" {
			t.Errorf("%s written under the consumer root", name)
		}
	}
	if fsys["p/a/canon.lock"] == nil || strings.Contains(string(fsys["p/a/canon.outputs"].Data), "@admin") {
		t.Errorf("lock or canon.outputs wrong:\n%s", fsys["p/a/canon.outputs"].Data)
	}
}

// DECISIONS 343: after a plain build, --only-root admin writes @admin's outputs and nothing else.
func TestOnlyRootWritesTheRootAlone(t *testing.T) {
	fsys := consumerTree()
	buildTree(t, fsys, build.BuildOptions{})
	before := files(fsys)
	res := buildTree(t, fsys, build.BuildOptions{OnlyRoot: "admin"})
	if len(res.List) != 0 || len(res.Locks) != 0 {
		t.Fatalf("findings %v, locks %v", res.List, res.Locks)
	}
	after := files(fsys)
	var added []string
	for _, name := range slices.Sorted(maps.Keys(after)) {
		if old, ok := before[name]; !ok {
			added = append(added, name)
		} else if old != after[name] {
			t.Errorf("%s changed", name)
		}
	}
	var paths []string
	for _, o := range res.Outputs {
		paths = append(paths, o.Path)
	}
	if !slices.Equal(added, []string{"admin/json/tiers.json", "admin/json/v.json"}) || !slices.Equal(paths, []string{"@admin/json/tiers.json", "@admin/json/v.json"}) {
		t.Errorf("added %v, outputs %v", added, paths)
	}
}

// DECISIONS 343: a lock change makes --only-root an error that writes nothing.
func TestOnlyRootRefusesALockChange(t *testing.T) {
	fsys := consumerTree()
	buildTree(t, fsys, build.BuildOptions{})
	fsys["p/a/a.canon"] = file(strings.Replace(consumerSource, "{ low {} }", "{ low {}, high {} }", 1))
	before := files(fsys)
	res := buildTree(t, fsys, build.BuildOptions{OnlyRoot: "admin"})
	if got := codesOf(res.List); !slices.Equal(got, []diag.Code{diag.E8028.Def().Code}) || len(res.Outputs) != 0 {
		t.Errorf("findings %v, outputs %+v", res.List, res.Outputs)
	}
	if !maps.Equal(before, files(fsys)) {
		t.Error("a refused --only-root wrote a file")
	}
}

// DECISIONS 343: a non-consumer root, or --only-root with --adopt, is refused up front.
func TestOnlyRootRefused(t *testing.T) {
	p, err := build.Open(consumerTree(), "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		opt  build.BuildOptions
		want error
	}{
		{build.BuildOptions{OnlyRoot: "out"}, build.ErrNotConsumerRoot},
		{build.BuildOptions{OnlyRoot: "nope"}, build.ErrNotConsumerRoot},
		{build.BuildOptions{OnlyRoot: "admin", Adopt: []string{"@admin/json/v.json"}}, build.ErrOnlyRootAdopt},
	} {
		if _, err := p.Build(context.Background(), c.opt); !errors.Is(err, c.want) {
			t.Errorf("%+v: %v, want %v", c.opt, err, c.want)
		}
	}
}

// DECISIONS 343, 336: a plain build never removes a stale listed file under a consumer root;
// the same stale file under @out is removed.
func TestConsumerStaleKept(t *testing.T) {
	fsys := consumerTree()
	buildTree(t, fsys, build.BuildOptions{})
	old := "old\n"
	sum := sha256.Sum256([]byte(old))
	line := hex.EncodeToString(sum[:]) + "  "
	listing := string(fsys["p/a/canon.outputs"].Data) + line + "@admin/notes/old.txt\n" + line + "@out/notes/old.txt\n"
	fsys["p/a/canon.outputs"] = file(listing)
	fsys[adminDir+"notes/old.txt"], fsys["p/out/notes/old.txt"] = file(old), file(old)
	if res := buildTree(t, fsys, build.BuildOptions{}); len(res.List) != 0 {
		t.Fatalf("findings %v", res.List)
	}
	if fsys[adminDir+"notes/old.txt"] == nil || fsys["p/out/notes/old.txt"] != nil {
		t.Errorf("under @admin kept %t, under @out removed %t", fsys[adminDir+"notes/old.txt"] != nil, fsys["p/out/notes/old.txt"] == nil)
	}
}

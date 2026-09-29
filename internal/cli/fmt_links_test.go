package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
)

const (
	linkBad    = "package shared\nlet  x:Int=1\n"
	linkGood   = "package shared\n\nlet x: Int = 1\n"
	linkJSON   = "{\"k\":1}\n"
	linkPretty = "{\n  \"k\": 1\n}\n"
)

// linkTree writes files under dir, and links (name -> target, relative to the link's own directory or absolute).
func linkTree(t *testing.T, dir string, files, links map[string]string) {
	t.Helper()
	for name, text := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range links {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.FromSlash(target), path); err != nil {
			t.Skipf("no symbolic links here: %v", err)
		}
	}
}

func runFmtIn(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	var out, errs bytes.Buffer
	code := cli.Main(context.Background(), append([]string{"fmt"}, args...), cli.Env{Stdout: &out, Stderr: &errs, Dir: dir})
	if errs.Len() > 0 {
		t.Errorf("stderr %s", errs.String())
	}
	return code, out.String()
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func isLink(t *testing.T, path string) bool {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode()&os.ModeSymlink != 0
}

// CLI.md §3.6: a link to a .canon file is never replaced: an inside target is formatted in place, an outside one left alone.
func TestFmtSymlinkedSources(t *testing.T) {
	root := t.TempDir()
	proj, outside := filepath.Join(root, "proj"), filepath.Join(root, "outside")
	linkTree(t, proj, map[string]string{
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n", "shared/t.canon": linkBad,
	}, map[string]string{"a/in.canon": "../shared/t.canon", "a/gone.canon": "../shared/missing.canon", "a/out.canon": filepath.ToSlash(filepath.Join(outside, "o.canon"))})
	linkTree(t, outside, map[string]string{"o.canon": linkBad}, nil)

	code, out := runFmtIn(t, proj, "--check")
	if code != 1 || out != "a/in.canon\n" {
		t.Fatalf("--check: exit %d, %q", code, out)
	}
	if code, out := runFmtIn(t, proj); code != 0 || out != "" {
		t.Fatalf("exit %d, %q", code, out)
	}
	if !isLink(t, filepath.Join(proj, "a", "in.canon")) || readText(t, filepath.Join(proj, "shared", "t.canon")) != linkGood {
		t.Errorf("the link was replaced or its target not formatted: %q", readText(t, filepath.Join(proj, "shared", "t.canon")))
	}
	if !isLink(t, filepath.Join(proj, "a", "out.canon")) || readText(t, filepath.Join(outside, "o.canon")) != linkBad {
		t.Errorf("a link out of the project was followed")
	}
}

// CLI.md §3.6, WIRE.md §6.5: --json-sources treats the JSON files load reads, by path or glob, likewise.
func TestFmtSymlinkedJSON(t *testing.T) {
	root := t.TempDir()
	proj, outside := filepath.Join(root, "proj"), filepath.Join(root, "outside")
	linkTree(t, proj, map[string]string{
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
		"a/a.canon": "package a\n\nlet i: Int = load(\"../data/in.json\")\nlet o: Int = load(\"../data/out.json\")\n" +
			"let all: [Int] = load.dir(\"../data/*.json\")\nlet lit: [Int] = load.dir(\"../data/out.json\")\n",
		"real/in.json": linkJSON,
	}, map[string]string{"data/in.json": "../real/in.json", "data/out.json": filepath.ToSlash(filepath.Join(outside, "o.json"))})
	linkTree(t, outside, map[string]string{"o.json": linkJSON}, nil)

	code, out := runFmtIn(t, proj, "--json-sources", "--check")
	if code != 1 || out != "data/in.json\n" {
		t.Fatalf("--check: exit %d, %q", code, out)
	}
	if code, out := runFmtIn(t, proj, "--json-sources"); code != 0 || out != "" {
		t.Fatalf("exit %d, %q", code, out)
	}
	if !isLink(t, filepath.Join(proj, "data", "in.json")) || readText(t, filepath.Join(proj, "real", "in.json")) != linkPretty {
		t.Errorf("the link was replaced or its target not formatted")
	}
	if !isLink(t, filepath.Join(proj, "data", "out.json")) || readText(t, filepath.Join(outside, "o.json")) != linkJSON {
		t.Errorf("a link out of the project was followed")
	}
}

// CLI.md §3.6: a link and its target are one file: it is visited, counted and listed once.
func TestFmtLinkAndTargetOnce(t *testing.T) {
	proj := t.TempDir()
	linkTree(t, proj, map[string]string{"project.canon": "project acme {\n  canon: \"0.1\"\n}\n", "shared/t.canon": linkBad},
		map[string]string{"a/in.canon": "../shared/t.canon"})
	code, out := runFmtIn(t, proj, "--check", "--format", "json")
	want := "{\"file\":\"a/in.canon\",\"formatted\":false}\n{\"summary\":{\"files\":2,\"unformatted\":1}}\n"
	if code != 1 || out != want || strings.Count(out, "\"file\"") != 1 {
		t.Errorf("exit %d, %q", code, out)
	}
}

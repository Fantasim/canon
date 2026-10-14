package canon_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

const (
	replaceFile = "c/c.canon"
	replaceOne  = "/// C.\npackage c\n\n/// N.\nlet n: Int = 1\n"
	replaceTwo  = "/// C.\npackage c\n\n/// N.\nlet n: Int = 2\n"
	replaceYear = 2020
)

// replacePlatforms is the GOOS set whose stat carries a change time (API.md S1); the same list as
// stampPlatforms in internal/workspace/stamp_internal_test.go.
var replacePlatforms = []string{"linux", "android", "darwin", "ios", "freebsd", "netbsd", "openbsd", "dragonfly", "illumos"}

func replaceBack(t *testing.T, abs string) {
	t.Helper()
	old := time.Date(replaceYear, time.January, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(abs, old, old); err != nil {
		t.Fatal(err)
	}
}

func replaceValue(t *testing.T, c *canon.Project, want string) {
	t.Helper()
	v, err := c.Value(context.Background(), "c:n")
	if err != nil {
		t.Fatal(err)
	}
	if got := v.String(); got != want {
		t.Fatalf("c:n = %q, want %q", got, want)
	}
}

// API.md S1, DECISIONS 374: a same-size file replaced with its modification time kept is read
// again by the next call.
func TestRefreshReplacedKeepingTime(t *testing.T) {
	if !slices.Contains(replacePlatforms, runtime.GOOS) {
		t.Skip("the stat has no change time on " + runtime.GOOS)
	}
	rows := []struct {
		name  string
		apply func(t *testing.T, abs string)
	}{
		{"in place", func(t *testing.T, abs string) {
			if err := os.WriteFile(abs, []byte(replaceTwo), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"renamed over", func(t *testing.T, abs string) {
			tmp := abs + ".new"
			if err := os.WriteFile(tmp, []byte(replaceTwo), 0o644); err != nil {
				t.Fatal(err)
			}
			replaceBack(t, tmp)
			if err := os.Rename(tmp, abs); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "project.canon", "project p {\n  canon: \"0.1\"\n}\n")
			writeFile(t, dir, replaceFile, replaceOne)
			abs := filepath.Join(dir, filepath.FromSlash(replaceFile))
			replaceBack(t, abs)
			c, err := canon.Open(dir, canon.Options{})
			if err != nil {
				t.Fatal(err)
			}
			replaceValue(t, c, "1")
			row.apply(t, abs)
			replaceBack(t, abs)
			replaceValue(t, c, "2")
		})
	}
}

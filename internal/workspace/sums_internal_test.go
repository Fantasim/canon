package workspace

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
)

// sumsTree is a directory holding a file, one below it, a link to the first and one to the
// directory of the second; its '/'-separated name.
func sumsTree(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "a.json"), "[1]\n")
	writeFile(t, filepath.Join(dir, "sub", "b.json"), "[2]\n")
	writeFile(t, filepath.Join(dir, "o.json"), "[3]\n")
	if os.Symlink("a.json", filepath.Join(dir, "l.json")) != nil || os.Symlink("sub", filepath.Join(dir, "dl")) != nil {
		t.Skip("no symbolic links here")
	}
	return filepath.ToSlash(dir)
}

// API.md S1, §3.4 (P18): a snapshot's sum of any name is the SHA-256 and error its read gives, logged alike.
func TestSumFileIsRead(t *testing.T) {
	dir := sumsTree(t)
	over := map[string][]byte{dir + "/o.json": []byte("[30]\n"), dir + "/sub/b.json": nil, dir + "/new.json": []byte("[4]\n")}
	names := []string{"a.json", "sub/b.json", "o.json", "new.json", "l.json", "dl/b.json", "gone.json", "sub"}
	byRead, bySum := newSnapFS(build.OS(), over, time.Now), newSnapFS(build.OS(), over, time.Now)
	for _, n := range names {
		abs := dir + "/" + n
		_, _ = byRead.ReadFile(abs)
		if _, ok, _ := project.SumFile(bySum, abs); !ok {
			t.Errorf("%s: a snapshot does not know its sum", n)
		}
	}
	writeFile(t, filepath.Join(dir, "a.json"), "[9, 9]\n")
	for _, s := range []*snapFS{byRead, bySum} {
		for _, n := range names {
			abs := dir + "/" + n
			sum, _, err := s.SumFile(abs)
			data, rerr := s.ReadFile(abs)
			if err != rerr || err == nil && sum != sha256.Sum256(data) {
				t.Errorf("%s: sum %x (%v), read %q (%v)", n, sum, err, data, rerr)
			}
		}
	}
	if !slices.Equal(byRead.log, bySum.log) {
		t.Errorf("reads logged %v, sums logged %v", byRead.log, bySum.log)
	}
	if sum, _, _ := bySum.SumFile(dir + "/l.json"); sum != sha256.Sum256([]byte("[1]\n")) {
		t.Error("a link's sum followed the disk within one snapshot")
	}
}

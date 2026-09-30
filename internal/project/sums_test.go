package project_test

import (
	"context"
	"crypto/sha256"
	"maps"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

// sumFS is memFS knowing each file's SHA-256 as a workspace snapshot does (project.SumFile),
// counting the files it reads.
type sumFS struct {
	memFS
	reads map[string]int
}

func (s sumFS) ReadFile(name string) ([]byte, error) {
	s.reads[name]++
	return s.memFS.ReadFile(name)
}

func (s sumFS) SumFile(name string) ([sha256.Size]byte, bool, error) {
	data, err := s.memFS.ReadFile(name)
	if err != nil {
		return [sha256.Size]byte{}, true, err
	}
	return sha256.Sum256(data), true, nil
}

// IMPLEMENTATION-PLAN §7.6, API.md S3 (P18): a Reader takes kept parses by sum, unread, with a cold read's sums.
func TestReaderTakesKnownSums(t *testing.T) {
	w := newWarmProject(t)
	fsys := sumFS{memFS: w.fsys, reads: map[string]int{}}
	for _, st := range []struct {
		name   string
		change func()
		read   []string
	}{
		{"first", func() {}, w.names},
		{"unchanged", func() {}, nil},
		{"lib edited", func() { w.fsys.m[memPath(libFile)] = &fstest.MapFile{Data: []byte(libChanged)} }, []string{libFile}},
	} {
		st.change()
		clear(fsys.reads)
		bags := map[string]*diag.Bag{}
		bagOf := func(pkg string) *diag.Bag {
			if bags[pkg] == nil {
				bags[pkg] = diag.NewBag(w.set, pkg)
			}
			return bags[pkg]
		}
		r := &project.Reader{FS: fsys, Dir: projectDir, Set: w.set, BagOf: bagOf, Reuse: w.reuse}
		if _, err := r.Parse(context.Background(), w.names); err != nil {
			t.Fatal(err)
		}
		var read []string
		for _, abs := range slices.Sorted(maps.Keys(fsys.reads)) {
			read = append(read, abs[len(projectDir)+1:])
		}
		if !slices.Equal(read, st.read) {
			t.Errorf("%s: read %v, want %v", st.name, read, st.read)
		}
		cold := &project.Reader{FS: w.fsys, Dir: projectDir, Set: w.set, BagOf: func(string) *diag.Bag { return diag.NewBag(w.set, "") }}
		if _, err := cold.Parse(context.Background(), w.names); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(r.Sums, cold.Sums) {
			t.Errorf("%s: sums %v, a cold read's %v", st.name, r.Sums, cold.Sums)
		}
		if got, want := findingsText(t, w.set, bags), w.cold(t, w.names); got != want {
			t.Errorf("%s: findings\n%s\nwant\n%s", st.name, got, want)
		}
	}
}

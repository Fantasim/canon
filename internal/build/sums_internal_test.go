package build

import (
	"crypto/sha256"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

// sumPolicy is how an editFS answers SumFile when its base sets one.
type sumPolicy interface {
	sumOf(f *editFS, name string) (sha256Sum, bool, error)
}

// SumFile answers as a workspace snapshot does (project.SumFile): the suites' warm runs take kept files unread.
func (f *editFS) SumFile(name string) (sha256Sum, bool, error) {
	if p, ok := f.base.(sumPolicy); ok {
		return p.sumOf(f, name)
	}
	return contentSum(f.ReadFile(name))
}

// edited is f's replacement of name, false when it has none.
func (f *editFS) edited(name string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.over[name]
	return data, ok
}

// contentSum is SumFile's answer for a read that gave data and err.
func contentSum(data []byte, err error) (sha256Sum, bool, error) {
	if err != nil {
		return sha256Sum{}, true, err
	}
	return sha256.Sum256(data), true, nil
}

// readsOnly is a base whose editFS knows no sum: every file is read, as before P18.
type readsOnly struct{ project.FS }

func (readsOnly) sumOf(*editFS, string) (sha256Sum, bool, error) { return sha256Sum{}, false, nil }

// EvalSymlinks resolves links as the file system under it does.
func (r readsOnly) EvalSymlinks(name string) (string, error) { return project.EvalSymlinks(r.FS, name) }

// staleSums is a base whose editFS answers each name's first sum for ever: the mutant a
// snapshot's sum must never be, since a changed file would then keep its old content.
type staleSums struct {
	project.FS
	mu    sync.Mutex
	first map[string]sha256Sum
}

func (s *staleSums) sumOf(f *editFS, name string) (sha256Sum, bool, error) {
	sum, ok, err := contentSum(f.ReadFile(name))
	s.mu.Lock()
	defer s.mu.Unlock()
	if had, seen := s.first[name]; seen && err == nil {
		return had, ok, nil
	}
	s.first[name] = sum
	return sum, ok, err
}

// countedReads is a base that counts each name its ReadFile reads; its sums read nothing it counts.
type countedReads struct {
	project.FS
	mu    sync.Mutex
	reads map[string]int
}

func (c *countedReads) ReadFile(name string) ([]byte, error) {
	c.mu.Lock()
	c.reads[name]++
	c.mu.Unlock()
	return c.FS.ReadFile(name)
}

func (c *countedReads) sumOf(f *editFS, name string) (sha256Sum, bool, error) {
	if data, ok := f.edited(name); ok {
		return contentSum(data, nil)
	}
	return contentSum(c.FS.ReadFile(name))
}

// taken is the loaded and source files read since the last call, sorted: project.canon,
// project.local.canon and the canon.lock probes aside, which a snapshot reads as it did before P18.
func (c *countedReads) taken() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, name := range slices.Sorted(maps.Keys(c.reads)) {
		if base := path.Base(name); base != project.FileName && base != project.LocalFileName && base != lockName {
			out = append(out, strings.TrimPrefix(name, archiveRoot+"/"))
		}
	}
	clear(c.reads)
	return out
}

// dirAnalyzer is the load.dir case over base's policy, and the case's files to edit.
func dirAnalyzer(t *testing.T, wrap func(roFS) project.FS) (*analyzer, dirFiles) {
	t.Helper()
	z := archiveAnalyzer(t, dirPartsCase)
	m := z.fs.base.(roFS)
	z.fs = newEditFS(wrap(m))
	return z, dirFiles{m: m}
}

// API.md S1, IMPLEMENTATION-PLAN §7.6 (P18): kept files are taken by sum, unread; a changed one alone is read.
func TestKeptSumsReadOnlyChanged(t *testing.T) {
	var counted *countedReads
	z, d := dirAnalyzer(t, func(m roFS) project.FS {
		counted = &countedReads{FS: m, reads: map[string]int{}}
		return counted
	})
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	for _, st := range []struct {
		name string
		do   func(roFS)
		read []string
	}{
		{"unchanged", func(roFS) {}, nil},
		{"json edited", d.edit(dirI2, `"cost": 3`, `"cost": 5`), []string{dirI2}},
		{"source edited", d.edit(dirCap, "most: 6", "most: 5"), []string{dirCap}},
		{"unchanged again", func(roFS) {}, nil},
	} {
		st.do(d.m)
		counted.taken()
		p, err := Open(z.fs, z.dir, z.opt)
		if err != nil {
			t.Fatal(err)
		}
		if warm, err = p.WithCache(z.cache).Analyze(t.Context(), nil); err != nil {
			t.Fatal(err)
		}
		if got := counted.taken(); !slices.Equal(got, st.read) {
			t.Errorf("%s: the warm analysis read %v, want %v", st.name, got, st.read)
		}
		if cold, err = p.Analyze(t.Context(), nil); err != nil {
			t.Fatal(err)
		}
		same(t, st.name, warm, cold)
	}
}

// IMPLEMENTATION-PLAN §7.6 (P18): a file taken by sum counts toward what its snapshot uses, as a file read does.
func TestKeptSumsCountLive(t *testing.T) {
	var live [2]int
	for i, wrap := range []func(roFS) project.FS{
		func(m roFS) project.FS { return m },
		func(m roFS) project.FS { return readsOnly{FS: m} },
	} {
		z, _ := dirAnalyzer(t, wrap)
		z.pair(t)
		gen := z.cache.gen
		p, err := Open(z.fs, z.dir, z.opt)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.WithCache(z.cache).Analyze(t.Context(), nil); err != nil {
			t.Fatal(err)
		}
		gen.mu.Lock()
		live[i] = gen.live
		gen.mu.Unlock()
		if z.cache.gen != gen {
			t.Errorf("run %d: an unchanged analysis compacted the file set", i)
		}
	}
	if live[0] != live[1] || live[0] == 0 {
		t.Errorf("an unchanged analysis uses %d bytes taken by sum, %d read", live[0], live[1])
	}
}

// IMPLEMENTATION-PLAN §7.6, WIRE.md §6.5: a file system that knows no sum reads every file, warm as cold.
func TestKeptSumsReadPathEqualsCold(t *testing.T) {
	z, d := dirAnalyzer(t, func(m roFS) project.FS { return readsOnly{FS: m} })
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	for _, st := range dirSteps(d) {
		st.do(d.m)
		warm, cold = z.pair(t)
		same(t, st.name, warm, cold)
	}
}

// WIRE.md §3 (P18): a kept file that does not decode reports its raw offset, CRLF included, warm as cold.
func TestKeptSumsEncodingEqualsCold(t *testing.T) {
	var counted *countedReads
	z, d := dirAnalyzer(t, func(m roFS) project.FS {
		counted = &countedReads{FS: m, reads: map[string]int{}}
		return counted
	})
	d.put(dirI2, "{\"sku\": \"i2\",\r\n \"cost\": 3,\r\n \"cat\": \"\xff\"}")(d.m)
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	counted.taken()
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Fatal(err)
	}
	if warm, err = p.WithCache(z.cache).Analyze(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	if got := counted.taken(); !slices.Equal(got, []string{dirI2}) {
		t.Errorf("the warm analysis read %v, want only the undecodable file, for its finding", got)
	}
	if cold, err = p.Analyze(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	same(t, "unchanged", warm, cold)
	if !slices.ContainsFunc(warm.Result().List, func(f diag.Finding) bool { return f.Code == diag.E7105.Def().Code }) {
		t.Errorf("no encoding finding in %v", warm.Result().List)
	}
}

// IMPLEMENTATION-PLAN §7.6 (P18): a stale sum keeps a changed file's old content, and the gate sees it.
func TestKeptSumsStaleCaught(t *testing.T) {
	z, d := dirAnalyzer(t, func(m roFS) project.FS { return &staleSums{FS: m, first: map[string]sha256Sum{}} })
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	var differ []string
	for _, st := range dirSteps(d) {
		st.do(d.m)
		warm, cold = z.pair(t)
		if dumpAnalysis(t, warm) != dumpAnalysis(t, cold) {
			differ = append(differ, st.name)
		}
	}
	if len(differ) == 0 {
		t.Error("stale sums gave every step a cold analysis's result")
	}
}

// API.md S3, S5 (P18): taken by sum, a run tells the reads a reading run tells, and a cold run's least displays.
func TestKeptSumsReadLogEqualsCold(t *testing.T) {
	root := twoDisplaysDir(t)
	sums, reads := newEditFS(project.OS()), newEditFS(readsOnly{FS: project.OS()})
	sumsCache, readsCache := NewCache(), NewCache()
	for _, st := range []struct {
		name, file, data string
	}{
		{"first", "", ""},
		{"unchanged", "", ""},
		{"x edited", "data/x.json", "[1, 2, 3]\n"},
		{"p1 edited", "data/p1.json", "{\"id\": \"p1\", \"v\": 2}\n"},
		{"unchanged again", "", ""},
	} {
		if st.file != "" {
			sums.set(root+"/"+st.file, []byte(st.data))
			reads.set(root+"/"+st.file, []byte(st.data))
		}
		kept, read := displaysOf(t, sums, root, sumsCache, nil), displaysOf(t, reads, root, readsCache, nil)
		if kept != read || kept == "" {
			t.Errorf("%s: taken by sum\n%s\nread\n%s", st.name, kept, read)
		}
		if cold := displaysOf(t, sums, root, nil, nil); leastDisplays(kept) != leastDisplays(cold) {
			t.Errorf("%s: warm\n%s\ncold\n%s", st.name, kept, cold)
		}
	}
}

// leastDisplays is a displaysOf text with each file's told displays cut to the least, which a
// snapshot keeps (ReadRecorder), and the read sets as they are.
func leastDisplays(text string) string {
	var sb strings.Builder
	for _, line := range strings.Split(text, "\n") {
		abs, told, ok := strings.Cut(line, " [")
		if ok {
			line = abs + " " + slices.Min(strings.Fields(strings.TrimSuffix(told, "]")))
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

// twoDisplaysDir writes twoDisplays under a new directory, with link leading to data, and
// returns the directory, '/'-separated; it skips where links cannot be made.
func twoDisplaysDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range slices.Sorted(maps.Keys(twoDisplays)) {
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), dirMode); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(twoDisplays[name]), fileMode); err != nil {
			t.Fatal(err)
		}
	}
	if os.Symlink("data", filepath.Join(dir, "link")) != nil {
		t.Skip("no symbolic links here")
	}
	return filepath.ToSlash(dir)
}

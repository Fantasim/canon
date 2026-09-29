package build

import (
	"fmt"
	"math/rand/v2"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
	"golang.org/x/tools/txtar"
)

const (
	entriesCase  = "testdata/incremental/entries.txtar"
	earlierCase  = "testdata/incremental/earlier.txtar"
	archiveRoot  = "/p"
	editSeed     = 7
	editSteps    = 14
	benchEntries = "200"
	typeBreak    = `"x"`
)

// fieldNumber is a field set to a number: what an edit changes (the number) or breaks (a string).
var fieldNumber = regexp.MustCompile(`: (-?[0-9][0-9_]*)\b`)

// archiveAnalyzer opens a txtar's files as a project in archiveRoot.
func archiveAnalyzer(t *testing.T, file string) *analyzer {
	t.Helper()
	a, err := txtar.ParseFile(file)
	if err != nil {
		t.Fatal(err)
	}
	m := roFS{}
	for _, f := range a.Files {
		m[strings.TrimPrefix(archiveRoot, "/")+"/"+f.Name] = srcFile(string(f.Data))
	}
	return &analyzer{fs: newEditFS(m), dir: archiveRoot, cache: NewCache()}
}

// editor edits entry values at random: a number changed, a number made a string (a type
// error in), or the file put back as it was (the error out); every key is kept.
type editor struct {
	z       *analyzer
	rnd     *rand.Rand
	files   []string // absolute names of the files edited
	entries []string // those holding only entries, edited every other step
	orig    map[string][]byte
	cur     map[string][]byte
	lineage int // the warm analyses that continued their predecessor's epoch
}

func newEditor(t *testing.T, z *analyzer, files, entries []string) *editor {
	t.Helper()
	e := &editor{z: z, rnd: rand.New(rand.NewPCG(editSeed, uint64(len(files)))), files: files, entries: entries,
		orig: map[string][]byte{}, cur: map[string][]byte{}}
	for _, f := range files {
		data, err := z.fs.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		e.orig[f], e.cur[f] = data, data
	}
	return e
}

// edit changes one file and says how.
func (e *editor) edit(step int) string {
	pool := e.files
	if step%2 == 1 && len(e.entries) > 0 {
		pool = e.entries
	}
	f := pool[e.rnd.IntN(len(pool))]
	text, op := e.cur[f], e.rnd.IntN(len(editOps))
	locs := fieldNumber.FindAllSubmatchIndex(text, -1)
	if len(locs) == 0 || editOps[op] == "revert" {
		e.put(f, e.orig[f])
		return fmt.Sprintf("step %d: revert %s", step, f)
	}
	at := locs[e.rnd.IntN(len(locs))]
	with := typeBreak
	if editOps[op] == "change" {
		n, _ := strconv.Atoi(strings.ReplaceAll(string(text[at[2]:at[3]]), "_", ""))
		with = strconv.Itoa(n + 1 + e.rnd.IntN(len(editOps)))
	}
	e.put(f, slices.Concat(text[:at[2]], []byte(with), text[at[3]:]))
	return fmt.Sprintf("step %d: %s %s at %d", step, editOps[op], f, at[2])
}

// editOps are the edits of editor.edit.
var editOps = [...]string{"change", "break", "revert"}

func (e *editor) put(f string, text []byte) {
	e.cur[f] = text
	e.z.fs.set(f, text)
}

// run analyzes, then edits and analyzes again steps times, the warm result always the cold one.
func (e *editor) run(t *testing.T, steps int) {
	t.Helper()
	warm, cold := e.z.pair(t)
	same(t, "first", warm, cold)
	for i := range steps {
		prev := warm.r.epoch
		name := e.edit(i)
		warm, cold = e.z.pair(t)
		same(t, name, warm, cold)
		if warm.r.epoch == prev {
			e.lineage++
		}
	}
	t.Logf("%d of %d edits re-checked along the lineage", e.lineage, steps)
}

// entryFiles is every file of the analysis's program holding only entries, by absolute name.
func entryFiles(t *testing.T, z *analyzer) []string {
	t.Helper()
	warm, _ := z.pair(t)
	var out []string
	for _, cp := range warm.Program().Packages {
		for _, f := range cp.Files {
			if len(f.Decls) > 0 && !slices.ContainsFunc(f.Decls, notEntry) {
				out = append(out, f.Src.Abs)
			}
		}
	}
	slices.Sort(out)
	return out
}

func notEntry(d syntax.Decl) bool {
	_, ok := d.(*syntax.EntryDecl)
	return !ok
}

// IMPLEMENTATION-PLAN §7.6 NFR-02 (log-2026-09-29 "U11 review"): random entry edits equal cold.
func TestIncrementalEqualsColdEntries(t *testing.T) {
	z := archiveAnalyzer(t, entriesCase)
	entries := entryFiles(t, z)
	e := newEditor(t, z, entries, entries)
	e.run(t, editSteps)
	if e.lineage == 0 {
		t.Error("no edit was re-checked along the lineage")
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02 (log-2026-09-29 "U12 review PASS (a)"): the earlier path edited.
func TestIncrementalEqualsColdEarlierFile(t *testing.T) {
	z := archiveAnalyzer(t, earlierCase)
	first := path.Join(archiveRoot, "a/items/a.canon")
	e := newEditor(t, z, []string{first}, []string{first})
	e.run(t, editSteps)
	if e.lineage == 0 {
		t.Error("no edit was re-checked along the lineage")
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: one entry edited again and again, all under one epoch, as cold.
func TestMemoRecheckLineage(t *testing.T) {
	z := archiveAnalyzer(t, entriesCase)
	plain := path.Join(archiveRoot, "a/items/two.canon")
	e := newEditor(t, z, []string{plain}, []string{plain})
	e.run(t, editSteps)
	if e.lineage != editSteps {
		t.Errorf("%d of %d edits kept the epoch", e.lineage, editSteps)
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: random edits of every example, entry files every other step.
func TestIncrementalEqualsColdExamples(t *testing.T) {
	if testing.Short() {
		t.Skip("every example, analyzed twice per edit")
	}
	z := examplesAnalyzer(t)
	e := newEditor(t, z, sourcesWithNumbers(t, z), entryFiles(t, z))
	e.run(t, editSteps)
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: random edits of a 200-entry benchmark project (benchgen).
func TestIncrementalEqualsColdBench(t *testing.T) {
	if testing.Short() {
		t.Skip("writes a benchmark project with go run")
	}
	dir := filepath.Join(t.TempDir(), "bench")
	cmd := exec.Command("go", "run", "./internal/testkit/cmd/benchgen", "-seed", "1", "-n", benchEntries, "-out", dir)
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("benchgen: %v\n%s", err, out)
	}
	z := &analyzer{fs: newEditFS(project.OS()), dir: filepath.ToSlash(dir), cache: NewCache()}
	entries := entryFiles(t, z)
	e := newEditor(t, z, append(slices.Clone(entries[:len(entries)/4]), sourcesWithNumbers(t, z)...), entries)
	e.run(t, editSteps)
	if e.lineage == 0 {
		t.Error("no edit was re-checked along the lineage")
	}
}

// examplesAnalyzer opens examples/ with its roots redirected (examples/_fixtures/README.md).
func examplesAnalyzer(t *testing.T) *analyzer {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "examples"))
	if err != nil {
		t.Fatal(err)
	}
	roots := map[string]string{"resource": "_fixtures/resource", "client": "_fixtures/client"}
	for _, name := range []string{"source", "services", "sovcommon", "web", "parity", "generated"} {
		roots[name] = filepath.ToSlash(filepath.Join(t.TempDir(), name))
	}
	return &analyzer{fs: newEditFS(project.OS()), dir: filepath.ToSlash(dir), opt: Options{Roots: roots}, cache: NewCache()}
}

// sourcesWithNumbers is every source of the project whose fields a number sets, but the entry
// files: the files whose edits a Recheck refuses, and so a full check with a new epoch.
func sourcesWithNumbers(t *testing.T, z *analyzer) []string {
	t.Helper()
	entries := entryFiles(t, z)
	names, err := project.Scan(z.fs, z.dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, name := range names {
		abs := path.Join(z.dir, name)
		data, err := z.fs.ReadFile(abs)
		if err != nil {
			t.Fatal(err)
		}
		if fieldNumber.Match(data) && !slices.Contains(entries, abs) {
			out = append(out, abs)
		}
	}
	return out
}

package progen_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// The two modes of DECISIONS 200: by default a short deterministic run inside make check;
// -progen.n cases per suite for the nightly run, from -progen.seed on.
var (
	flagN    = flag.Int("progen.n", 0, "cases per suite; 0 runs the short deterministic set")
	flagSeed = flag.Uint64("progen.seed", 1, "seed of the first case")
	flagKeep = flag.Bool("progen.keep", false, "write each shrunk counterexample under testdata/counterexamples")
	update   = flag.Bool("update", false, "rewrite testdata/coverage.txt")
)

const (
	examplesDir      = "../../../examples"
	expectedDir      = "expected"
	projectFile      = "project.canon"
	lockFile         = "canon.lock"
	keptDir          = "testdata/counterexamples"
	keptGlob         = keptDir + "/*.txtar"
	shrinkTries      = 2000
	crashTries       = 600     // replays a crash's whole shrinking may run, each in a child
	caseSeedStride   = 1 << 32 // far more cases than a run makes: the suites' seeds never meet
	suiteMutation    = "mutation"
	suiteGrammar     = "grammar"
	suiteCorrupt     = "corrupt"
	suiteTyped       = "typed"       // DECISIONS 200 item 3: type-directed well-typed programs
	suiteMeta        = "metamorphic" // DECISIONS 200 item 4: renames, reordering, comments, whitespace
	modeCheck        = "check"
	modeBuild        = "build"
	modeData         = "data"        // an emit mode (CODEGEN.md §2.1)
	modeEmbedded     = "embedded"    // an emit mode (CODEGEN.md §2.1)
	kindGoFail       = "gofail"      // the generated Go's temporary module failed to build or test
	kindMissingJSON  = "missingjson" // a root's JSON output is missing
	kindBadJSON      = "badjson"     // a root's JSON output does not parse
	kindJSONMismatch = "jsonmismatch"
	kindMetaFindings = "metafindings" // a metamorphic variant gains or loses a finding
	kindMetaOutput   = "metaoutput"   // a metamorphic variant writes an output differently
	wantFields       = 5
	decimalBase      = 10
	seedBits         = 64
	crashContext     = 3000
	lockFields       = 3
	annotJSON        = "json"   // the @json annotation (WIRE.md §4)
	annotStable      = "stable" // the @stable annotation (LOCK.md §1)
	argPairs         = "pairs"  // its pairs: argument (WIRE.md §5.14)
	testRunFlag      = "test.run"
	failMark         = "progen-failure\t"
	relayMark        = "progen-log\t"
	childTest        = "^TestReplayChild$"
	childMark        = "progen-verdict\t"
	verdictFields    = 3
	scanLimit        = 1 << 24
	kindCrash        = "crash"
	kindPanic        = "panic"
	kindInternal     = "internal"
	kindMalformed    = "malformed"
	kindHarness      = "harness"
	kindMissing      = "missing"
	kindHang         = "hang"
	kindExtra        = "extra"
	kindMisplaced    = "misplaced"
	kindRepeated     = "repeated"
	kindError        = "error"
	classMismatch    = "mismatch" // born of a mutation's findings, not of a crash (doc.go)
	classProperty    = "property" // born of a grammar or corruption property
	shapeSep         = ", "
	variantSep       = "/"
	countSep         = "*"              // between a leftover's shape and its count
	childTimeout     = 2 * time.Minute  // -progen.childtimeout's default
	watchChecks      = 8                // idle checks per child time limit
	waitDelay        = 10 * time.Second // how long Wait waits for a killed child's output
	replayTimeout    = 20 * time.Second // -progen.replaytimeout's default
	hangSamples      = 32               // goroutine samples a hung child is asked for
	sampleEvery      = 20 * time.Millisecond
	sampleMark       = "progen: goroutine sample"
	sampleEnd        = "progen: end of sample"
	goroutineDump    = 2 // pprof's debug level that prints every goroutine's stack as a panic does
	goroutineProfile = "goroutine"
	cycleSep         = " ~ "           // between the sorted functions of a recursion or a hang
	quitMark         = "SIGQUIT: quit" // the first line of a child's goroutine dump
	stateRunning     = "[running"      // a dumped goroutine's state: on a thread
	stateRunnable    = "[runnable"     // a dumped goroutine's state: preempted to dump
	minTimeout       = time.Second     // the least -test.timeout a child gets
	compilerPkg      = "github.com/fantasim/canonlang/internal/"
	harnessPkg       = "testkit/"
	sigFrames        = 2  // compiler frames a crash or panic signature names
	cycleFrames      = 50 // innermost frames of an overflow, all Go prints before eliding
	stackOverflow    = "stack overflow"
	elidedMark       = "..." // starts the line where Go's traceback elides frames
	testMutations    = "TestMutations"
	testGrammar      = "TestGrammar"
	testCorruption   = "TestCorruption"
	childStack       = 128 << 20 // a child's stack: a runaway recursion dies at a fraction of the default
	examplesBudget   = "budget: 100_000_000"
	harnessBudget    = "budget: 2_000_000"
	outRoots         = "_out/"
	rootsKey         = "roots" // GRAMMAR.md §7.1
	fixtureResource  = "_fixtures/resource"
	fixtureClient    = "_fixtures/client"
)

// exampleRoots redirects the examples' roots as examples/_fixtures/README.md says: the read
// roots to the fixtures, the written ones to _out/ in the project, where a case can put a file
// a build would overwrite.
func exampleRoots() map[string]string {
	roots := map[string]string{"resource": fixtureResource, "client": fixtureClient}
	for _, name := range []string{"source", "services", "sovcommon", "web", "parity", "generated"} {
		roots[name] = outRoots + name
	}
	return roots
}

// target is one file of a package that checks clean: what mutation operators edit. all is
// every target of the corpus, for operators that read other files.
type target struct {
	pkg, path string
	src       []byte
	file      *syntax.File
	all       *[]target
}

// corpus is the examples project, its clean packages and their files.
type corpus struct {
	project  *progen.Project
	clean    []string
	targets  []target
	baseline map[string][]progen.Finding // pkg -> its own unmutated findings (DECISIONS 200)
}

var (
	corpusOnce sync.Once
	corpusVal  *corpus
	corpusErr  error
)

// examples loads the corpus once, its budget lowered so a loop ends fast (EVALUATION.md §12.2).
func examples(t testing.TB) *corpus {
	t.Helper()
	corpusOnce.Do(func() { corpusVal, corpusErr = loadCorpus() })
	if corpusErr != nil {
		t.Fatal(corpusErr)
	}
	return corpusVal
}

func loadCorpus() (*corpus, error) {
	p, err := progen.LoadDir(examplesDir, expectedDir)
	if err != nil {
		return nil, err
	}
	pc, _ := p.Get(projectFile)
	if !strings.Contains(string(pc), examplesBudget) {
		return nil, fmt.Errorf("examples/%s no longer sets %q: the harness lowers that budget", projectFile, examplesBudget)
	}
	p.Set(projectFile, []byte(strings.Replace(string(pc), examplesBudget, harnessBudget, 1)))
	locks, err := addLocks(p)
	if err != nil {
		return nil, err
	}
	units, err := progen.Units(context.Background(), p, exampleRoots())
	if err != nil {
		return nil, err
	}
	c := &corpus{project: p, baseline: map[string][]progen.Finding{}}
	for _, u := range units {
		out := progen.Run(context.Background(), p, progen.RunOptions{Packages: []string{u.Name}, Roots: exampleRoots()})
		if out.Err != nil || out.Panic != "" || disqualifies(out.Findings) {
			continue
		}
		c.clean = append(c.clean, u.Name)
		c.baseline[u.Name] = out.Findings
		for _, name := range u.Files {
			src, _ := p.Get(name)
			c.targets = append(c.targets, target{pkg: u.Name, path: name, src: src, file: parse(name, src)})
		}
		c.targets = append(c.targets, dataTargets(p, u)...)
	}
	if len(c.clean) == 0 {
		return nil, fmt.Errorf("no example package checks clean")
	}
	pc, _ = p.Get(projectFile)
	c.targets = append(c.targets, target{pkg: c.clean[0], path: projectFile, src: pc, file: parse(projectFile, pc)})
	for _, name := range locks {
		if pkg := strings.ReplaceAll(path.Dir(name), "/", "."); slices.Contains(c.clean, pkg) {
			src, _ := p.Get(name)
			c.targets = append(c.targets, target{pkg: pkg, path: name, src: src})
		}
	}
	for i := range c.targets {
		c.targets[i].all = &c.targets
	}
	return c, nil
}

// dataTargets are the JSON files under a package's directory: the data its loads may read.
func dataTargets(p *progen.Project, u progen.Unit) []target {
	if len(u.Files) == 0 {
		return nil
	}
	dir := path.Dir(u.Files[0]) + "/"
	var out []target
	for _, name := range p.Names() {
		if path.Ext(name) == ".json" && strings.HasPrefix(name, dir) {
			src, _ := p.Get(name)
			out = append(out, target{pkg: u.Name, path: name, src: src})
		}
	}
	return out
}

// addLocks puts each example's golden canon.lock beside its package, as a build left it, so
// that the lock rules (LOCK.md) have a lock to compare with; it returns their paths.
func addLocks(p *progen.Project) ([]string, error) {
	goldens, err := filepath.Glob(filepath.Join(examplesDir, "*", expectedDir, lockFile))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, g := range goldens {
		data, err := os.ReadFile(g)
		if err != nil {
			return nil, err
		}
		name := path.Join(filepath.Base(filepath.Dir(filepath.Dir(g))), lockFile)
		p.Set(name, data)
		out = append(out, name)
	}
	return out, nil
}

// parse is a file's syntax tree, whatever its findings.
func parse(name string, src []byte) *syntax.File {
	fs := &source.FileSet{}
	f, err := fs.Add(name, "/"+name, src)
	if err != nil {
		return nil
	}
	kind := syntax.FileSource
	if filepath.Base(name) == projectFile {
		kind = syntax.FileProject
	}
	return syntax.Parse(f, kind, diag.NewBag(fs, ""))
}

// goAndJSON are the targets this compiler generates today; a build case asks only for them.
var goAndJSON = []ir.Target{ir.TargetGo, ir.TargetJSON}

// suites are the suites in the order that spaces their seeds apart.
var suites = []string{suiteMutation, suiteGrammar, suiteCorrupt, suiteTyped, suiteMeta}

// caseSeed is the seed of case i of a suite: suites draw from disjoint runs of seeds.
func caseSeed(suite string, i int) uint64 {
	base := *flagSeed + uint64(slices.Index(suites, suite)+1)*caseSeedStride
	return base + uint64(i)
}

// announce names the case about to run on stderr, so that a crash the test binary cannot recover
// from (a stack overflow) still names its seed to the supervising test.
func announce(suite, name string, i int, seed uint64) {
	fmt.Fprintf(os.Stderr, "progen: %s %s case %d seed %d\n", suite, name, i, seed)
}

// kept are the suite, name and signature of the counterexamples this process wrote.
var kept = map[string]bool{}

// alreadyKept tells a counterexample of this suite, name and signature that this process or an
// earlier run kept: -progen.keep writes one of each.
func alreadyKept(suite, name, sig string) bool {
	return kept[suite+" "+name+" "+sigKey(sig)] || slices.ContainsFunc(keptArchives(), func(c *progen.Counterexample) bool {
		return standsFor(c, suite, name, sig)
	})
}

// report fails t with a shrunk counterexample, born with its signature; under -progen.keep it
// also writes the archive, the first of its suite, name and signature only.
func report(t *testing.T, c *progen.Counterexample, v verdict) {
	t.Helper()
	c.Born = bornOf(c.Suite, c.Sig)
	body := c.Format()
	if *flagKeep && !alreadyKept(c.Suite, c.Name, c.Sig) {
		kept[c.Suite+" "+c.Name+" "+sigKey(c.Sig)] = true
		name := filepath.Join(keptDir, fmt.Sprintf("%s_%s_%d.txtar", c.Suite, slug(c.Name), c.Seed))
		if err := os.MkdirAll(keptDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fail(t, "%s %s seed %d: %s\n%s%s", c.Suite, c.Name, c.Seed, v.Text, body, v.Detail)
}

func slug(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, s)
}

var (
	keptOnce sync.Once
	keptVal  []*progen.Counterexample
)

// keptArchives are the counterexamples kept when this process started.
func keptArchives() []*progen.Counterexample {
	keptOnce.Do(func() {
		paths, _ := filepath.Glob(keptGlob)
		for _, p := range paths {
			if c, err := progen.ReadCounterexample(p); err == nil {
				keptVal = append(keptVal, c)
			}
		}
	})
	return keptVal
}

// openBug reports a kept counterexample, still open, that stands for this failure.
func openBug(suite, name, sig string) bool {
	return slices.ContainsFunc(keptArchives(), func(c *progen.Counterexample) bool {
		return c.Open != "" && standsFor(c, suite, name, sig)
	})
}

package progen_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/testkit/progen"
	"github.com/fantasim/canonlang/internal/testkit/progen/grammar"
)

// Every suite runs its cases in child processes of this test binary: a case that crashes the
// process (a stack overflow cannot be recovered) is reported with its seed, and the suite goes on
// from the next case.
var (
	flagChild = flag.Bool("progen.child", false, "run the cases in this process (set by the supervising test)")
	flagFrom  = flag.Int("progen.from", 0, "first case to run, so that one case replays alone")
)

var (
	reAnnounce = regexp.MustCompile(`progen: \S+ .* case (\d+) seed (\d+)`)
	reCrash    = regexp.MustCompile(`(?m)^(fatal error: |panic: .*\n\ngoroutine )`)
	reFailure  = regexp.MustCompile(`(?m)^` + failMark + `(.*)$`)
	reRelay    = regexp.MustCompile(`(?m)^` + relayMark + `(.*)$`)
)

// relay logs a line of t; in a child it also prints it marked, and the supervising test logs it.
func relay(t *testing.T, format string, args ...any) {
	t.Helper()
	text := fmt.Sprintf(format, args...)
	t.Log(text)
	if *flagChild {
		fmt.Fprintln(os.Stderr, relayMark+strconv.Quote(text))
	}
}

// relayed are the lines a child relayed.
func relayed(out []byte) []string {
	var texts []string
	for _, m := range reRelay.FindAllSubmatch(out, -1) {
		if text, err := strconv.Unquote(string(m[1])); err == nil {
			texts = append(texts, text)
		}
	}
	return texts
}

// fail fails t; in a child it also prints the failure on one marked line, which the supervising
// test reports even when a later case crashes the child.
func fail(t *testing.T, format string, args ...any) {
	t.Helper()
	text := fmt.Sprintf(format, args...)
	t.Error(text)
	if *flagChild {
		fmt.Fprintln(os.Stderr, failMark+strconv.Quote(text))
	}
}

// failures are the failures a child printed.
func failures(out []byte) []string {
	var texts []string
	for _, m := range reFailure.FindAllSubmatch(out, -1) {
		if text, err := strconv.Unquote(string(m[1])); err == nil {
			texts = append(texts, text)
		}
	}
	return texts
}

// cases are the case numbers this process runs: from -progen.from to -progen.n, or to short in
// the short run.
func cases(short int) (from, to int) {
	return *flagFrom, total(short)
}

func total(short int) int {
	if *flagN > 0 {
		return *flagN
	}
	return short
}

// supervise runs test's cases in child processes and tells whether it did; the child runs them
// itself.
func supervise(t *testing.T, test string, cases int) bool {
	t.Helper()
	if *flagChild {
		answerSamples()
		debug.SetMaxStack(childStack)
		return false
	}
	for from := *flagFrom; from < cases; {
		out, hung, err := runChild(t, test, from, cases)
		for _, text := range relayed(out) {
			t.Log(text)
		}
		for _, text := range failures(out) {
			t.Error(text)
		}
		crash, crashed := crashVerdict(out)
		if hung {
			crash, crashed = hangVerdict(out, *flagChildTimeout), true
		}
		switch {
		case err == nil:
			return true
		case !crashed && len(failures(out)) == 0 && !bytes.Contains(out, []byte("--- FAIL")):
			t.Errorf("harness: the child of %s exited with %v and no failure:\n%s", test, err, out)
			return true
		case !crashed:
			if len(failures(out)) == 0 {
				t.Errorf("%s", out)
			}
			return true
		}
		k, seed, ok := lastCase(out)
		if !ok {
			t.Errorf("harness: the child of %s crashed before its first case:\n%s", test, crashLines(out))
			return true
		}
		handleCrash(t, test, k, seed, crash)
		from = k + 1
	}
	return true
}

// handleCrash reports the crash of case k, unless an open archive stands for it, and keeps it
// under -progen.keep.
func handleCrash(t *testing.T, test string, k int, seed uint64, crash verdict) {
	t.Helper()
	suite, prop := crashName(test, k)
	switch {
	case openBug(suite, prop, crash.Sig):
		t.Logf("known open crash (%s %s) at case %d, seed %d", suite, prop, k, seed)
	case *flagKeep && !alreadyKept(suite, prop, crash.Sig):
		t.Errorf("%s case %d (seed %d) crashes the process: %s", test, k, seed, crash.Text)
		keepCrash(t, test, k, seed, crash.Sig)
	default:
		t.Errorf("%s case %d (seed %d) crashes the process: %s", test, k, seed, crash.Text)
	}
}

// crashName is the suite and name of case k of test.
func crashName(test string, k int) (string, string) {
	switch test {
	case testGrammar:
		return suiteGrammar, propRoundTrip
	case testCorruption:
		return suiteCorrupt, propCorruption
	case testTyped:
		return suiteTyped, propTyped
	case testMeta:
		ops := metaOperators()
		return suiteMeta, ops[k%len(ops)].name
	}
	ops := catalogue()
	return suiteMutation, ops[k%len(ops)].name()
}

// runChild runs test's cases from from to cases in a child.
func runChild(t *testing.T, test string, from, cases int) ([]byte, bool, error) {
	t.Helper()
	args := []string{
		"-test.run=" + childPattern(test), "-progen.child",
		"-progen.n=" + strconv.Itoa(cases), "-progen.from=" + strconv.Itoa(from),
		"-progen.seed=" + strconv.FormatUint(*flagSeed, decimalBase),
	}
	if *flagKeep {
		args = append(args, "-progen.keep")
	}
	cmd, cancel := childCommand(t, args...)
	defer cancel()
	out, hung, err := runWatched(cmd, *flagChildTimeout)
	if err != nil {
		return out, hung, fmt.Errorf("child %s: %w", test, err)
	}
	return out, hung, nil
}

// childCommand runs this test binary with args, under t's deadline: the child's -test.timeout
// (1s at least) and its context end with it.
func childCommand(t *testing.T, args ...string) (*exec.Cmd, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.Background(), context.CancelFunc(func() {})
	timeout := "0"
	if d, ok := t.Deadline(); ok {
		ctx, cancel = context.WithDeadline(ctx, d)
		timeout = max(time.Until(d).Round(time.Second), minTimeout).String()
	}
	args = append([]string{"-test.count=1", "-test.timeout=" + timeout}, args...)
	return exec.CommandContext(ctx, os.Args[0], args...), cancel
}

// childPattern selects test in a child, and the subtests this run's -test.run selects in it.
func childPattern(test string) string {
	pattern := "^" + test + "$"
	if f := flag.Lookup(testRunFlag); f != nil {
		if _, sub, ok := strings.Cut(f.Value.String(), "/"); ok {
			pattern += "/" + sub
		}
	}
	return pattern
}

// lastCase is the last case a child announced before it died; false when it announced none.
func lastCase(out []byte) (int, uint64, bool) {
	m := reAnnounce.FindAllSubmatch(out, -1)
	if len(m) == 0 {
		return 0, 0, false
	}
	last := m[len(m)-1]
	k, err1 := strconv.Atoi(string(last[1]))
	seed, err2 := strconv.ParseUint(string(last[2]), decimalBase, seedBits)
	return k, seed, err1 == nil && err2 == nil
}

// crashLines are the crash message and the first frames of the compiler it names.
func crashLines(out []byte) string {
	i := reCrash.FindIndex(out)
	if i == nil {
		return ""
	}
	end := min(len(out), i[0]+crashContext)
	return fmt.Sprintf("%s…", out[i[0]:end])
}

// crashCase is a crashing case rebuilt: its archive, the places shrinking keeps (the expected
// finding's first), how to write its want once they moved, and whether a shrunk candidate still
// holds what its site needs (always, for a generated file).
type crashCase struct {
	c      *progen.Counterexample
	pins   []progen.Place
	want   func(progen.Place) string
	placed func(q *progen.Project, pins []progen.Place) bool
	pinned func(name string) bool // the files a metamorphic case keeps whatever it crashes on
}

// keepCrash rebuilds a crashing case, shrinks it in child replays while it crashes with sig (a
// typed case not at all, a metamorphic one by whole files only), and keeps it; -progen.fixed
// later records the findings the crash hid as its left line.
func keepCrash(t *testing.T, test string, k int, seed uint64, sig string) {
	t.Helper()
	cc, ok := crashCaseOf(t, test, k, seed)
	if !ok {
		t.Errorf("harness: case %d of %s cannot be rebuilt to keep its crash", k, test)
		return
	}
	c := cc.c
	if strings.HasPrefix(sig, kindHang) {
		v := hangVerdict(nil, *flagChildTimeout)
		v.Sig = sig
		c.Sig, c.Note = sig, v.Text
		report(t, c, v)
		return
	}
	crashes := func(q *progen.Project, pins []progen.Place) bool {
		trial := *c
		trial.Files = q
		if len(pins) > 0 {
			trial.Want = cc.want(pins[0])
		}
		return sigKey(replayArchive(t, &trial).Sig) == sigKey(sig)
	}
	left := crashTries
	switch {
	case test == testMeta:
		c.Files = trimFiles(c.Files, cc.pinned, func(q *progen.Project) bool {
			left--
			return left >= 0 && crashes(q, nil)
		}, crashTries)
	case test != testTyped:
		small, pins := shrinkProject(c.Files, cc.pins, func(q *progen.Project, pins []progen.Place) bool {
			if left--; left < 0 || !cc.placed(q, pins) {
				return false
			}
			return crashes(q, pins)
		}, crashTries)
		c.Files = small
		if len(pins) > 0 {
			c.Want = cc.want(pins[0])
		}
	}
	v := replayArchive(t, c)
	c.Sig, c.Note = v.Sig, v.Text
	report(t, c, v)
}

// crashCaseOf rebuilds case k of test's suite.
func crashCaseOf(t *testing.T, test string, k int, seed uint64) (crashCase, bool) {
	t.Helper()
	switch test {
	case testCorruption:
		return programCase(suiteCorrupt, k, seed), true
	case testMutations:
		return mutationCase(t, k, seed), true
	case testTyped:
		return crashCase{c: typedArchive(k, seed, buildTyped(seed), verdict{})}, true
	case testMeta:
		arch, tg, ok := metaCrashCase(examples(t), k, seed)
		pinned := func(name string) bool {
			return name == projectFile || name == tg.path || strings.HasPrefix(name, harnessDir)
		}
		return crashCase{c: arch, pinned: pinned}, ok
	}
	return programCase(suiteGrammar, k, seed), true
}

// replayArchive writes c to a temporary archive and replays it in a child process.
func replayArchive(t *testing.T, c *progen.Counterexample) verdict {
	t.Helper()
	name := filepath.Join(t.TempDir(), "case.txtar")
	if err := os.WriteFile(name, c.Format(), 0o644); err != nil {
		t.Fatal(err)
	}
	return replayInChild(t, name, replayLimit(c))
}

// programCase is case k's generated (and, for the corruption suite, corrupted) file.
func programCase(suite string, k int, seed uint64) crashCase {
	r := progen.NewRand(seed)
	kind := caseKind(k)
	src := grammar.GenerateKind(r, progen.NewBudget(genSize, genDepth), kind)
	prop := propRoundTrip
	if suite == suiteCorrupt {
		src = grammar.Corrupt(r, src)
		prop = propCorruption
	}
	files := progen.NewProject()
	if kind == grammar.Source {
		files.Set(projectFile, []byte(genProject))
	}
	files.Set(kind.File(), src)
	c := &progen.Counterexample{Suite: suite, Name: prop, Case: k, Seed: seed, Want: prop, Files: files}
	always := func(*progen.Project, []progen.Place) bool { return true }
	return crashCase{c: c, want: func(progen.Place) string { return prop }, placed: always}
}

// mutationCase is case k's mutated project, with the site's places pinned.
func mutationCase(t *testing.T, k int, seed uint64) crashCase {
	c := examples(t)
	ops := catalogue()
	o := ops[k%len(ops)]
	p := progen.Pick(progen.NewRand(seed), o.placements(c))
	m, err := p.site.Mutate(c.project)
	if err != nil {
		t.Fatal(err)
	}
	pins := append([]progen.Place{m.At}, m.Written...)
	arch := &progen.Counterexample{
		Suite: suiteMutation, Name: o.name(), Case: k, Seed: seed, Packages: p.pkgs, Layers: p.layers,
		Want: wantOf(o, m.At), Files: m.Project,
	}
	f := failure{o: o, run: p.run, m: m}
	placed := func(q *progen.Project, pins []progen.Place) bool { return stillPlaced(f, q, pins[1:]) }
	return crashCase{c: arch, pins: pins, want: func(at progen.Place) string { return wantOf(o, at) }, placed: placed}
}

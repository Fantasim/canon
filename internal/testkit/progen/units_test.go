package progen_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

var (
	flagHangChild = flag.Bool("progen.hangchild", false, "print one line, then sleep: the synthetic hang of TestRunWatched")
	flagLoopChild = flag.Bool("progen.loopchild", false, "print one line, then loop: the live hang of TestHangSignature")
)

const (
	hangRuns = 3      // children TestHangSignature samples
	spinWork = 10_000 // iterations of each part of the synthetic loop
	spinMul  = 31
)

// stack is a Go traceback of compiler functions, innermost first.
func stack(names ...string) string {
	var b strings.Builder
	b.WriteString("goroutine 1 [running]:\n")
	for _, n := range names {
		fmt.Fprintf(&b, "%s%s(0x1, 0x2)\n\t/src/x.go:1 +0x1\n", compilerPkg, n)
	}
	return b.String()
}

// cycle is n frames of the functions fs repeated, starting at phase.
func cycle(fs []string, phase, n int) []string {
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, fs[(phase+i)%len(fs)])
	}
	return out
}

// Decision log "M1.5 round 2 calls": an overflow is signed by the functions its frames repeat.
func TestRecursion(t *testing.T) {
	a := recursion(stack(cycle([]string{"eval.a", "eval.dispatch"}, 0, cycleFrames)...))
	b := recursion(stack(cycle([]string{"eval.b", "eval.dispatch"}, 0, cycleFrames)...))
	if a == b {
		t.Errorf("two cycles through one dispatcher share the signature %q", a)
	}
	long := make([]string, 0, cycleFrames/2)
	for i := range cap(long) {
		long = append(long, fmt.Sprintf("check.f%02d", i))
	}
	first := recursion(stack(cycle(long, 0, cycleFrames)...))
	for phase := range long {
		if got := recursion(stack(cycle(long, phase, cycleFrames)...)); got != first {
			t.Errorf("phase %d: %q, want %q", phase, got, first)
		}
	}
	if got := recursion(stack("eval.z", "eval.y")); got != "eval.y ~ eval.z" {
		t.Errorf("no repeat: %q, want every name, sorted", got)
	}
	elided := stack(cycle([]string{"eval.a", "eval.dispatch"}, 0, cycleFrames/2)...) + elidedMark + "9 frames elided...\n" + stack("eval.z", "eval.z")
	if got := recursion(elided); got != "eval.a ~ eval.dispatch" {
		t.Errorf("frames past the elision count: %q", got)
	}
}

// A panic's signature names its first compiler frames, without arguments, runtime or harness.
func TestCompilerFrames(t *testing.T) {
	s := "panic: boom\n\ngoroutine 7 [running]:\nruntime/debug.Stack()\n\t/go/x.go:1\n" +
		compilerPkg + "testkit/progen.Run.func1()\n\t/x.go:2\n" + stack("value.(*Ref).identity", "value.identity", "eval.put")
	if got := compilerFrames(s); got != "value.(*Ref).identity < value.identity" {
		t.Errorf("compilerFrames = %q", got)
	}
}

// goroutine is one goroutine of a dump in state, running the compiler functions names.
func goroutine(state string, names ...string) string {
	return strings.Replace(stack(names...), "[running]", "["+state+"]", 1)
}

// dump is a child's output: what it printed, then its SIGQUIT dump of goroutines.
func dump(before string, goroutines ...string) string {
	return before + quitMark + "\nPC=0x1 m=0 sigcode=0\n\n" + strings.Join(goroutines, "\n")
}

// sample is one goroutine sample as a child prints it.
func sample(goroutines ...string) string {
	return sampleMark + "\n" + strings.Join(goroutines, "\n") + sampleEnd + "\n"
}

// Decision log "M1.5 round 2 calls": a hang is named by the compiler functions its stuck
// goroutine never left over the samples, whichever instant each caught; "hang" without one.
func TestHangVerdict(t *testing.T) {
	waiting := goroutine("chan receive", "build.(*run).wait", "build.Run")
	loop := func(leaf ...string) string {
		return goroutine("runnable", append(leaf, "eval.loop", "check.Run")...)
	}
	for _, tc := range []struct{ name, out, want string }{
		{"the leaf moves", sample(loop("eval.step")) + sample(loop()) + sample(waiting, loop("eval.other", "eval.step")), "check.Run ~ eval.loop"},
		{"running before waiting", sample(waiting, goroutine("running", "eval.loop")), "eval.loop"},
		{"waiting only", sample(goroutine("select"), waiting), "build.(*run).wait ~ build.Run"},
		{"a sample without the compiler", sample(loop()) + sample(goroutine("running")), ""},
		{"an incomplete sample", sample(loop()) + sampleMark + "\n" + goroutine("running", "eval.zz"), "check.Run ~ eval.loop"},
		{"samples before the dump", sample(loop()) + dump("", loop("eval.step")), "check.Run ~ eval.loop"},
		{"the dump alone", dump("", waiting, goroutine("running", "eval.loop", "eval.step")), "eval.loop ~ eval.step"},
		{"frames printed before the dump", dump(stack("eval.before"), goroutine("running")), ""},
		{"no dump", stack("eval.x"), ""},
		{"no compiler frame", quitMark + "\n", ""},
	} {
		want := kindHang
		if tc.want != "" {
			want += " in " + tc.want
		}
		if got := hangVerdict([]byte(tc.out), time.Second).Sig; got != want {
			t.Errorf("%s: %q, want %q", tc.name, got, want)
		}
	}
}

// TestHangChild is the synthetic hang TestRunWatched runs.
func TestHangChild(t *testing.T) {
	if !*flagHangChild {
		t.Skip("run by TestRunWatched only")
	}
	answerSamples()
	fmt.Fprintln(os.Stderr, "progen: hanging")
	time.Sleep(time.Hour)
}

// A silent child gets SIGQUIT, dumps its goroutines and is reported hung; one that ends within
// its limit (a generous one: a -race binary starts slowly) is not.
func TestRunWatched(t *testing.T) {
	const limit = 300 * time.Millisecond
	out, hung, err := runWatched(exec.Command(os.Args[0], "-test.run=^TestHangChild$", "-progen.hangchild"), limit)
	if !hung || err == nil || !bytes.Contains(out, []byte(quitMark)) || len(samples(string(out))) == 0 {
		t.Errorf("hung %v, err %v, output:\n%s", hung, err, out)
	}
	if v := hangVerdict(out, limit); v.Sig != kindHang {
		t.Errorf("a hang in the harness is named %q, want %q", v.Sig, kindHang)
	}
	out, hung, err = runWatched(exec.Command(os.Args[0], "-test.run=^TestHangChild$"), time.Minute)
	if hung || err != nil {
		t.Errorf("a child that ends hung %v, err %v:\n%s", hung, err, out)
	}
}

// zzSpin is the synthetic compiler loop of TestLoopChild; zzStep the call it makes and leaves.
//
//go:noinline
func zzSpin(n uint64) uint64 {
	for {
		for range spinWork {
			n = n*spinMul + 1
		}
		n = zzStep(n)
	}
}

//go:noinline
func zzStep(n uint64) uint64 {
	for range spinWork {
		n ^= n << 1
	}
	return n
}

// TestLoopChild is the looping child TestHangSignature samples.
func TestLoopChild(t *testing.T) {
	if !*flagLoopChild {
		t.Skip("run by TestHangSignature only")
	}
	answerSamples()
	fmt.Fprintln(os.Stderr, "progen: looping")
	zzSpin(1)
}

// A live loop gets one signature, wherever each sample catches it: its loop, not the call it
// makes and leaves. The dumps name the loop as a compiler package's, which the harness is not.
func TestHangSignature(t *testing.T) {
	const limit = 500 * time.Millisecond
	sigs := make([]string, hangRuns)
	var wg sync.WaitGroup
	for i := range sigs {
		wg.Go(func() {
			out, _, _ := runWatched(exec.Command(os.Args[0], "-test.run=^TestLoopChild$", "-progen.loopchild"), limit)
			relabelled := bytes.ReplaceAll(out, []byte(compilerPkg+harnessPkg+"progen_test.zz"), []byte(compilerPkg+"zzloop.zz"))
			sigs[i] = hangVerdict(relabelled, limit).Sig
		})
	}
	wg.Wait()
	for i, sig := range sigs {
		if want := kindHang + " in zzloop.zzSpin"; sig != want {
			t.Errorf("run %d: %q, want %q", i, sig, want)
		}
	}
}

// undo gives back what a site replaced and removes what it added; stillPlaced finds the site
// again only while what it relies on stays.
func TestUndoAndStillPlaced(t *testing.T) {
	p := progen.NewProject()
	p.Set("a/a.canon", []byte("package a\nkeep\n"))
	p.Set("a/b.json", []byte("{}\n"))
	o := operator{code: diag.E3003.Def().Code, sites: func(tg target) []progen.Site {
		if tg.path != "a/a.canon" || !bytes.Contains(tg.src, []byte("keep")) {
			return nil
		}
		return []progen.Site{{Edits: []progen.Edit{insert(0, "// x\n"), insert(8, "zz")}, Add: map[string][]byte{"a/new.canon": []byte("n"), "a/b.json": []byte("[]")}}}
	}}
	s := o.sites(target{path: "a/a.canon", src: []byte("package a\nkeep\n")})[0]
	s.Path = "a/a.canon"
	m, err := s.Mutate(p)
	if err != nil {
		t.Fatal(err)
	}
	if u := undo(m.Project, m.Written, m.Was); !sameFiles(u, p) {
		t.Errorf("undo did not give the project back: %v", u.Names())
	}
	f := failure{o: o, run: run{pkgs: []string{"a"}}, m: m}
	if !stillPlaced(f, m.Project, m.Written) {
		t.Error("the unshrunk case is not placed")
	}
	cut := m.Project.Clone()
	src, _ := cut.Get("a/a.canon")
	cut.Set("a/a.canon", bytes.Replace(src, []byte("keep"), nil, 1))
	if stillPlaced(f, cut, m.Written) {
		t.Error("a case whose precondition was cut is still placed")
	}
}

// Two written regions that start at one place (a deletion's empty text, then an insertion
// right after it) are undone the longer first, in either order of the site's edits.
func TestUndoTies(t *testing.T) {
	p := progen.NewProject()
	p.Set("a/a.canon", []byte("abcdef"))
	cut := progen.Edit{Start: 2, End: 4}
	for _, edits := range [][]progen.Edit{{cut, insert(4, "t")}, {insert(4, "t"), cut}} {
		m, err := progen.Site{Path: "a/a.canon", Edits: edits}.Mutate(p)
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := undo(m.Project, m.Written, m.Was).Get("a/a.canon"); string(got) != "abcdef" {
			t.Errorf("edits %v: undone to %q", edits, got)
		}
	}
}

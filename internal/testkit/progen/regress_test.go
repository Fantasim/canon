package progen_test

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

var (
	flagReplay = flag.String("progen.replay", "", "replay one kept counterexample and print its verdict (TestCounterexamples' child)")
	flagResig  = flag.Bool("progen.resig", false, "rewrite the signature of each open counterexample whose failure changed")
)

// TestCounterexamples replays every kept counterexample in a child process: a crash fails its
// case, not the run; an open one must fail with its signature, a fixed one guards the fix.
func TestCounterexamples(t *testing.T) {
	paths, err := filepath.Glob(keptGlob)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			c, err := progen.ReadCounterexample(p)
			if err != nil {
				t.Fatal(err)
			}
			v := replayInChild(t, p)
			switch {
			case c.Open != "" && v.Kind != "" && sigKey(v.Sig) == sigKey(c.Sig):
				t.Skipf("open bug, owned by %s: %s", c.Open, c.Sig)
			case c.Open != "" && v.Kind != "" && *flagResig:
				t.Logf("signature rewritten: %q, was %q", v.Sig, c.Sig)
				c.Sig, c.Note = v.Sig, v.Text
				writeArchive(t, p, c)
			case c.Open != "" && v.Kind != "":
				t.Errorf("the failure changed: archived %q, replayed %q (%s)", c.Sig, v.Sig, v.Text)
			case c.Open != "":
				t.Errorf("it passes now: drop the open line of %s so that it guards the fix", p)
			case v.Kind != "":
				t.Errorf("regressed: %s", v.Text)
			}
		})
	}
}

// writeArchive writes c back to its archive p.
func writeArchive(t *testing.T, p string, c *progen.Counterexample) {
	t.Helper()
	if err := os.WriteFile(p, c.Format(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// replayInChild runs TestReplayChild on one archive and reads its verdict. A child that dies
// with a crash is a crash verdict, one silent past -progen.replaytimeout a hang; one that dies
// otherwise without a verdict fails t.
func replayInChild(t *testing.T, archive string) verdict {
	t.Helper()
	cmd, cancel := childCommand(t, "-test.run="+childTest, "-progen.replay="+archive)
	defer cancel()
	out, hung, runErr := runWatched(cmd, *flagReplayTimeout)
	if hung {
		return hangVerdict(out, *flagReplayTimeout)
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(nil, scanLimit)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), childMark); ok {
			f := strings.SplitN(rest, "\t", verdictFields)
			if len(f) == verdictFields {
				return verdict{Kind: f[0], Sig: f[1], Text: f[2]}
			}
		}
	}
	if v, ok := crashVerdict(out); ok && runErr != nil {
		return v
	}
	t.Errorf("the replay of %s printed no verdict (%v):\n%s", archive, runErr, out)
	return verdict{Kind: kindHarness, Sig: kindHarness, Text: "no verdict"}
}

// crashVerdict is the verdict of a child's output that holds a crash.
func crashVerdict(out []byte) (verdict, bool) {
	m := reCrash.FindIndex(out)
	if m == nil {
		return verdict{}, false
	}
	line, _, _ := strings.Cut(string(out[m[0]:]), "\n")
	sig := kindCrash + " " + unplaced(line) + " in " + compilerFrames(string(out[m[0]:]))
	if strings.Contains(line, stackOverflow) {
		sig = kindCrash + " " + unplaced(line) + " in " + recursion(string(out[m[0]:]))
	}
	return verdict{Kind: kindCrash, Sig: sig, Text: kindCrash + " " + firstLines(string(out[m[0]:]))}, true
}

func firstLines(s string) string {
	lines := strings.SplitN(s, "\n", 4)
	return strings.Join(lines[:min(len(lines), 3)], " | ")
}

// TestReplayChild is the child side of TestCounterexamples.
func TestReplayChild(t *testing.T) {
	if *flagReplay == "" {
		t.Skip("run by TestCounterexamples only")
	}
	answerSamples()
	debug.SetMaxStack(childStack)
	c, err := progen.ReadCounterexample(*flagReplay)
	if err != nil {
		t.Fatal(err)
	}
	var v verdict
	switch c.Suite {
	case suiteMutation:
		v = replayMutation(c)
	default:
		v = replayProgram(c)
	}
	fmt.Printf("%s%s\t%s\t%s\n", childMark, v.Kind, sigKey(v.Sig), sigKey(v.Text))
}

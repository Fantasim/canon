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
	"time"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

var (
	flagReplay = flag.String("progen.replay", "", "replay one kept counterexample and print its verdict (TestCounterexamples' child)")
	flagResig  = flag.Bool("progen.resig", false, "rewrite the signature of each open counterexample whose failure changed")
	flagFixed  = flag.Bool("progen.fixed", false, "drop the open line of each counterexample whose born bug is gone, recording its leftovers")
)

// TestCounterexamples replays each kept counterexample in a child, judged as doc.go says: an open
// one must still fail with its signature, a fixed one guards the fix within its left line (only
// -progen.fixed writes it; -progen.resig never rewrites born or left).
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
			checkHeader(t, c)
			v := replayInChild(t, p, replayLimit(c))
			switch {
			case c.Open != "" && v.Kind != "" && sigKey(v.Sig) == sigKey(c.Sig):
				t.Skipf("open bug, owned by %s: %s", c.Open, c.Sig)
			case c.Open != "" && v.Kind != "" && *flagResig:
				t.Logf("signature rewritten: %q, was %q", v.Sig, c.Sig)
				c.Sig, c.Note = v.Sig, v.Text
				writeArchive(t, p, c)
			case c.Open != "" && v.Kind != "":
				t.Errorf("the failure changed: archived %q, replayed %q (%s)", c.Sig, v.Sig, v.Text)
			case c.Open != "" && *flagFixed:
				t.Logf("fixed: open line dropped, leftovers %q", v.Sig)
				c.Open, c.Left = "", v.Sig
				writeArchive(t, p, c)
			case c.Open != "":
				t.Errorf("it passes now: -progen.fixed drops the open line of %s and records its leftovers (%s)", p, v.Text)
			case v.Kind != "":
				t.Errorf("regressed: %s", v.Text)
			case v.Text != "":
				t.Log(v.Text)
			}
		})
	}
}

// checkHeader fails t for a born line that is not its signature's class and that signature, a
// guard line on an archive not born of a crash, and a left line on one that is open or whose
// suite judges its whole property.
func checkHeader(t *testing.T, c *progen.Counterexample) {
	t.Helper()
	if c.Born == "" || c.Born != bornOf(c.Suite, bornSig(c.Born)) {
		t.Fatalf("born line %q: want the class of the signature it was born with, then that signature", c.Born)
	}
	if class, _, _ := strings.Cut(c.Born, " "); c.Guard != "" && (c.Suite != suiteMutation || !validGuard(class, c.Guard)) {
		t.Fatalf("guard line %q: only a mutation archive born of a crash may be guarded, by %q", c.Guard, kindCrash)
	}
	if c.Left != "" && (c.Suite != suiteMutation || c.Open != "") {
		t.Fatalf("left line %q: only a fixed mutation archive records leftovers", c.Left)
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
func replayInChild(t *testing.T, archive string, limit time.Duration) verdict {
	t.Helper()
	cmd, cancel := childCommand(t, "-test.run="+childTest, "-progen.replay="+archive)
	defer cancel()
	out, hung, runErr := runWatched(cmd, limit)
	if hung {
		return hangVerdict(out, limit)
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

// replayLimit is how long the replay of c may stay silent: a typed archive compiles and tests a
// Go module (goTestTimeout) besides building its program.
func replayLimit(c *progen.Counterexample) time.Duration {
	if c.Suite == suiteTyped {
		return max(*flagReplayTimeout, typedReplayTimeout)
	}
	return *flagReplayTimeout
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
		v = replayMutation(c, examples(t))
	case suiteTyped:
		v = replayTyped(c)
	case suiteMeta:
		v = replayMeta(c)
	default:
		v = replayProgram(c)
	}
	fmt.Printf("%s%s\t%s\t%s\n", childMark, v.Kind, sigKey(v.Sig), sigKey(v.Text))
}

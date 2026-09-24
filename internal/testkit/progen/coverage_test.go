package progen_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

const (
	moduleRoot   = "../../.."
	modulePrefix = "github.com/fantasim/canonlang/"
	buildPackage = "./internal/build"
	moduleAll    = "./..."
	diagDir      = "internal/diag"
	gapsFile     = "testdata/gaps.txt"
	coverageFile = "testdata/coverage.txt"
)

// reReported is a code named through the registry in compiler code (the audit's
// diag-code-untested reads the same references).
var reReported = regexp.MustCompile(`\bdiag\.([EW][0-9]{4})\b`)

// TestCoverage is DECISIONS 200's ratchet: every code the compiler can report today has a
// mutation operator or a line in testdata/gaps.txt, which may only shrink. coverage.txt is an
// informational report, written by -update.
func TestCoverage(t *testing.T) {
	reachable := reportedCodes(t)
	gaps := readGaps(t)
	ops := map[diag.Code]string{}
	for _, o := range catalogue() {
		if _, ok := ops[o.code]; !ok {
			ops[o.code] = o.rule
		}
	}
	for _, d := range diag.Registry {
		c := d.Code
		switch {
		case d.Severity == diag.Runtime:
			// Signalled by generated code: no check or build reports it.
		case reachable[c] && ops[c] == "" && gaps[c] == "":
			t.Errorf("%s is reported by the compiler and has no mutation operator: add one, or list it in %s", c, gapsFile)
		case gaps[c] != "" && ops[c] != "":
			t.Errorf("%s has an operator: remove it from %s", c, gapsFile)
		case gaps[c] != "" && !reachable[c]:
			t.Errorf("%s is no longer reported by the compiler: remove it from %s", c, gapsFile)
		case ops[c] != "" && !reachable[c]:
			t.Errorf("%s has an operator but no compiler code reports it", c)
		}
	}
	got := coverageReport(reachable, namedCodes(t, moduleAll), ops, gaps)
	if *update {
		if err := os.WriteFile(coverageFile, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if want, err := os.ReadFile(coverageFile); err != nil || !bytes.Equal(got, want) {
		t.Logf("%s is stale; it is informational (go test -run TestCoverage -update)", coverageFile)
	}
}

// reportedCodes are the codes that the hand-written compiler code a build runs names through
// the registry: the packages internal/build depends on, internal/diag aside. A package the build
// does not import (conform, today) reports nothing check or build can show.
func reportedCodes(t *testing.T) map[diag.Code]bool {
	t.Helper()
	out := map[diag.Code]bool{}
	for code := range namedCodes(t, "-deps", buildPackage) { //canon:unordered builds a set
		out[code] = true
	}
	return out
}

// namedCodes maps each code that the module's non-test code outside internal/diag names to the
// first package naming it, over the packages go list gives for args.
func namedCodes(t *testing.T, args ...string) map[diag.Code]string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"list", "-json"}, args...)...)
	cmd.Dir = moduleRoot
	data, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	out := map[diag.Code]string{}
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var p listed
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			return out
		} else if err != nil {
			t.Fatal(err)
		}
		if rel, ok := strings.CutPrefix(p.ImportPath, modulePrefix); ok && !strings.HasPrefix(rel, diagDir) {
			scanPackage(t, p, rel, out)
		}
	}
}

// scanPackage adds to out, for rel, the codes its Go files name that out does not hold yet.
func scanPackage(t *testing.T, p listed, rel string, out map[diag.Code]string) {
	t.Helper()
	for _, f := range p.GoFiles {
		src, err := os.ReadFile(filepath.Join(p.Dir, f))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range reReported.FindAllSubmatch(src, -1) {
			if out[diag.Code(m[1])] == "" {
				out[diag.Code(m[1])] = rel
			}
		}
	}
}

// listed is the part of go list's output reportedCodes reads.
type listed struct {
	ImportPath, Dir string
	GoFiles         []string
}

// readGaps reads "CODE reason" lines; '#' starts a comment line.
func readGaps(t *testing.T) map[diag.Code]string {
	t.Helper()
	data, err := os.ReadFile(gapsFile)
	if err != nil {
		t.Fatal(err)
	}
	out := map[diag.Code]string{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		code, reason, _ := strings.Cut(line, " ")
		if strings.TrimSpace(reason) == "" || out[diag.Code(code)] != "" {
			t.Errorf("%s: %q needs one line with a reason", gapsFile, line)
		}
		out[diag.Code(code)] = strings.TrimSpace(reason)
	}
	return out
}

// coverageReport is one line per catalogued code: operator, gap or unreachable, and why.
func coverageReport(reachable map[diag.Code]bool, named, ops, gaps map[diag.Code]string) []byte {
	var b bytes.Buffer
	b.WriteString("# Rule-targeted mutation coverage of spec/ERRORS.md (DECISIONS 200), by go test -update.\n")
	b.WriteString("# code, owning package, status: operator (its rule), gap (testdata/gaps.txt) or unreachable.\n")
	counts := map[string]int{}
	for _, d := range diag.Registry {
		status, why := "unreachable", "no compiler code reports it yet"
		switch {
		case d.Severity == diag.Runtime:
			why = "a runtime code, signalled by generated code"
		case !reachable[d.Code] && named[d.Code] != "":
			why = "reported by " + named[d.Code] + ", which check and build do not run yet"
		case ops[d.Code] != "":
			status, why = "operator", ops[d.Code]
		case gaps[d.Code] != "":
			status, why = "gap", gaps[d.Code]
		}
		counts[status]++
		fmt.Fprintf(&b, "%s %-9s %-11s %s\n", d.Code, d.Package, status, why)
	}
	fmt.Fprintf(&b, "# %d codes: %d with an operator, %d gaps, %d unreachable; %d reachable.\n",
		len(diag.Registry), counts["operator"], counts["gap"], counts["unreachable"], len(reachable))
	return b.Bytes()
}

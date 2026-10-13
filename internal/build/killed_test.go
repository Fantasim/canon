package build_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
)

// A build killed in a helper process after its k-th rename: this test binary run with killRun,
// then killSep, the project directory and k.
const (
	killRun    = "-test.run=^TestKilledBuildHelper$"
	killSep    = "--"
	killRule   = "DECISIONS 326, 336, 351: "
	killSteps  = 8
	killedText = "B"
	nextText   = "C"
)

// killFS is the OS's file system that kills its own process after its k-th rename, as a crash
// between a build's renames would.
type killFS struct {
	build.WriteFS
	k, n *int
}

func (f killFS) Rename(oldname, newname string) error {
	err := f.WriteFS.Rename(oldname, newname)
	if *f.n++; *f.n == *f.k {
		if self, ferr := os.FindProcess(os.Getpid()); ferr == nil {
			_ = self.Kill() // SIGKILL; TerminateProcess on Windows: nothing after it runs
		}
	}
	return err
}

// TestKilledBuildHelper builds the project after argv's killSep, killed after its k-th rename; run
// any other way it does nothing.
func TestKilledBuildHelper(t *testing.T) {
	i := slices.Index(os.Args, killSep)
	if i < 0 || len(os.Args) < i+3 {
		t.Skip("a killed build only")
	}
	k, err := strconv.Atoi(os.Args[i+2])
	if err != nil {
		os.Exit(2)
	}
	n := 0
	p, err := build.Open(killFS{WriteFS: build.OS(), k: &k, n: &n}, filepath.ToSlash(os.Args[i+1]), build.Options{})
	if err != nil {
		os.Exit(2)
	}
	if _, err := p.Build(context.Background(), build.BuildOptions{}); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

// movedText is package a writing x.sql and y.sql holding content into @out/<dir>.
func movedText(dir, content string) string {
	return "/// A.\npackage a\n\n/// X.\n@text(\"x.sql\")\nexport fn x() -> String { return \"" + content + "\" }\n\n" +
		"/// Y.\n@text(\"y.sql\")\nexport fn y() -> String { return \"" + content + "\" }\n\nemit text { out: \"@out/" + dir + "\" }\n"
}

// DECISIONS 326, 336, 351: a build moving its text files, killed after any rename, is recovered by the next.
func TestKilledBuildRecovers(t *testing.T) {
	for k := 1; k <= killSteps; k++ {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{"project.canon": textProject, "a/a.canon": movedText("d0", "A")})
		if res, err := buildOS(dir, build.BuildOptions{}); err != nil || res.Summary.Errors != 0 {
			t.Fatalf("%sfirst build: %v %v", killRule, err, res)
		}
		writeFiles(t, dir, map[string]string{"a/a.canon": movedText("d1", killedText)})
		cmd := exec.Command(os.Args[0], killRun, killSep, dir, strconv.Itoa(k))
		_ = cmd.Run() // killed, or done when the build has fewer renames
		writeFiles(t, dir, map[string]string{"a/a.canon": movedText("d1", nextText)})
		res, err := buildOS(dir, build.BuildOptions{})
		if err != nil || res.Summary.Errors != 0 {
			t.Fatalf("%skilled after rename %d: the next build: %v %v", killRule, k, err, codes(res.List))
		}
		got, err := os.ReadFile(filepath.Join(dir, "out", "d1", "x.sql"))
		_, gone := os.Stat(filepath.Join(dir, "out", "d0", "x.sql"))
		check, cerr := buildOS(dir, build.BuildOptions{Check: true})
		if err != nil || string(got) != nextText || gone == nil || cerr != nil || check.Stale {
			t.Errorf("%skilled after rename %d: d1/x.sql %q (%v), d0/x.sql left %v, --check stale %v (%v)", killRule, k, got, err, gone == nil, check.Stale, cerr)
		}
	}
}

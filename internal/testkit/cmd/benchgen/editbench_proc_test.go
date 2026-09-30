//go:build linux

package main

import (
	"bufio"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The machine TestBenchEdit reports, as Linux describes it.
const (
	cpuInfo     = "/proc/cpuinfo"
	loadAvg     = "/proc/loadavg"
	cpuModelKey = "model name"
	moduleRoot  = "../../../.."
	examplesDir = "../../../../examples"
	goldenDir   = "expected"
	cacheDir    = ".canon"
	rssUnit     = 1024 // ru_maxrss is in KiB on Linux
	loadFields  = 3    // the 1, 5 and 15 minute load averages
)

// exampleOutRoots are the roots examples/project.canon writes to, redirected to the test's
// directory; resource and client read the fixtures (examples/_fixtures/README.md).
var exampleOutRoots = []string{"source", "services", "sovcommon", "web", "parity", "generated"}

// machine is the CPU model, the cores Go sees and the load average.
func machine() string {
	model := "unknown CPU"
	if f, err := os.Open(cpuInfo); err == nil {
		defer func() { _ = f.Close() }()
		s := bufio.NewScanner(f)
		for s.Scan() {
			if k, v, ok := strings.Cut(s.Text(), ":"); ok && strings.TrimSpace(k) == cpuModelKey {
				model = strings.TrimSpace(v)
				break
			}
		}
	}
	load, _ := os.ReadFile(loadAvg)
	fields := strings.Fields(string(load))
	return "machine: " + model + ", " + strconv.Itoa(runtime.NumCPU()) + " cores, load average " + strings.Join(fields[:min(len(fields), loadFields)], " ")
}

// buildCanon builds cmd/canon into the test's directory.
func buildCanon(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "canon")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/canon")
	cmd.Dir = moduleRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// examplesCopy is examples/ copied without its goldens, every root redirected.
func examplesCopy(t *testing.T) benchTarget {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "examples")
	err := filepath.WalkDir(examplesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(examplesDir, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == goldenDir {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dir, rel), dirPerm)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, rel), data, filePerm)
	})
	if err != nil {
		t.Fatal(err)
	}
	roots := map[string]string{"resource": filepath.Join(dir, "_fixtures", "resource"), "client": filepath.Join(dir, "_fixtures", "client")}
	out := t.TempDir()
	for _, name := range exampleOutRoots {
		roots[name] = filepath.Join(out, name)
	}
	return benchTarget{name: "examples", dir: dir, roots: roots}
}

// normalize runs `canon fmt --json-sources` in the target, as a project migrates once before
// its JSON sources are edited (API.md M9, DECISIONS 12): six examples do not write them in the
// canonical layout (benchgen's project does).
func normalize(t *testing.T, bin string, target benchTarget) {
	t.Helper()
	if out, err := canonIn(bin, target, "fmt", "--json-sources").CombinedOutput(); err != nil {
		t.Fatalf("canon fmt --json-sources: %v\n%s", err, out)
	}
}

// canonIn is canon's command run in the target's directory, its roots redirected, then args.
func canonIn(bin string, target benchTarget, command string, args ...string) *exec.Cmd {
	all := []string{command}
	for _, name := range slices.Sorted(maps.Keys(target.roots)) {
		all = append(all, "-root", name+"="+target.roots[name])
	}
	cmd := exec.Command(bin, append(all, args...)...)
	cmd.Dir = target.dir
	return cmd
}

// coldChecks runs `canon check` of each unit twice, "" being the whole project: first with no
// cache (cold, with its peak RSS), then with nothing changed (warm, reported only).
func coldChecks(t *testing.T, bin string, target benchTarget, units []string, r *report) {
	t.Helper()
	for _, unit := range units {
		if err := os.RemoveAll(filepath.Join(target.dir, cacheDir)); err != nil {
			t.Fatal(err)
		}
		cold, rss := runCheck(t, bin, target, unit)
		warm, _ := runCheck(t, bin, target, unit)
		name := "canon check " + unit + ": cold, RSS GB, warm"
		if unit == "" {
			name = "canon check: cold, RSS GB, warm"
		}
		r.add(name, seconds(cold, targetCold, true), measure{value: float64(rss) / 1e9, limit: targetRSS / 1e9, unit: "GB", gated: true},
			seconds(warm, targetWarm, false))
	}
}

// runCheck is one `canon check` of a unit, which must exit 0 (a project with errors is not
// measured): its wall time and peak resident set in bytes.
func runCheck(t *testing.T, bin string, target benchTarget, unit string) (time.Duration, int64) {
	t.Helper()
	args := []string{"-q"}
	if unit != "" {
		args = append(args, unit)
	}
	cmd := canonIn(bin, target, "check", args...)
	start := time.Now()
	out, err := cmd.CombinedOutput()
	wall := time.Since(start)
	if err != nil {
		t.Fatalf("canon check %s: %v; a project with errors is not measured\n%s", unit, err, out)
	}
	usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		t.Fatal("no resource usage for canon check")
	}
	return wall, usage.Maxrss * rssUnit
}

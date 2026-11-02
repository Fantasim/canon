//go:build linux

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The scale bench's errors: a scale that is no positive whole number, a unit that is not positive.
var (
	errBadScale = errors.New("-benchgen.scales must be positive whole numbers")
	errBadUnit  = errors.New("-benchgen.unit must be a positive number of entries")
)

// The scale bench's flags (DECISIONS 441); make bench-scale sets them. The empty scales skip it.
var (
	flagScales   = flag.String("benchgen.scales", "", "run TestBenchScale at these multiples of -benchgen.unit, space-separated (make bench-scale)")
	flagUnit     = flag.Int("benchgen.unit", defaultScaleUnit, "the entries of the 1x size: Resource's source bytes (IMPLEMENTATION-PLAN.md §7.6)")
	flagScaleOut = flag.String("benchgen.scaleout", defaultScaleOut, "the TSV file one line per measured size is appended to")
)

const (
	defaultScaleUnit = 15000
	defaultScaleOut  = "/var/tmp/canon-bench-scale.tsv"
	scaleTmpPrefix   = "canon-bench-scale-"
	canonExt         = ".canon"
	statusOK         = "ok"
	statusKilled     = "killed"
	tsvFmt           = "%d\t%d\t%d\t%d\t%d\t%s\t%.3f\t%d"
	tableFmt         = "%dx %7d entries %7d files %9d lines %10d bytes  %-6s %8.1f s %7.2f GB"
	procCgroup       = "/proc/self/cgroup"
	cgroupV2Prefix   = "0::"
	cgroupRoot       = "/sys/fs/cgroup"
	memoryMaxFile    = "memory.max"
	tsvFlags         = os.O_APPEND | os.O_CREATE | os.O_WRONLY
	gigabyte         = 1e9
)

// scaleSize is one size of the bench: a multiple of the unit and the entries it makes.
type scaleSize struct{ scale, entries int }

// scaleSizes is the sizes of scales ("1 3 5"), each that many times unit; none for no scale, which
// skips the bench.
func scaleSizes(scales string, unit int) ([]scaleSize, error) {
	fields := strings.Fields(scales)
	if len(fields) == 0 {
		return nil, nil
	}
	if unit <= 0 {
		return nil, errBadUnit
	}
	sizes := make([]scaleSize, len(fields))
	for i, f := range fields {
		k, err := strconv.Atoi(f)
		if err != nil || k <= 0 {
			return nil, fmt.Errorf("%q: %w", f, errBadScale)
		}
		sizes[i] = scaleSize{scale: k, entries: k * unit}
	}
	return sizes, nil
}

// canonCount is what a project's `.canon` files add up to.
type canonCount struct{ files, lines, bytes int64 }

// scaleRow is one measured size: what was written, and the cold check's status, wall time and
// peak RSS (the cap when it was killed).
type scaleRow struct {
	size scaleSize
	canonCount
	status string
	wall   time.Duration
	rss    int64
}

// tsv is the row's line of the TSV file: scale, entries, files, lines, bytes, status, seconds, RSS.
func (r scaleRow) tsv() string {
	return fmt.Sprintf(tsvFmt, r.size.scale, r.size.entries, r.files, r.lines, r.bytes, r.status, r.wall.Seconds(), r.rss)
}

// table is the row as printed.
func (r scaleRow) table() string {
	return fmt.Sprintf(tableFmt, r.size.scale, r.size.entries, r.files, r.lines, r.bytes, r.status, r.wall.Seconds(), float64(r.rss)/gigabyte)
}

// countCanon is the number, newline count and size in bytes of the `.canon` files under dir.
func countCanon(dir string) (canonCount, error) {
	var c canonCount
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != canonExt {
			return err
		}
		data, err := os.ReadFile(path)
		c.files++
		c.lines += int64(bytes.Count(data, []byte{'\n'}))
		c.bytes += int64(len(data))
		return err
	})
	return c, err
}

// cgroupFile is the memory.max file of the cgroup (v2) that /proc/self/cgroup's text names, or
// "" when it names none.
func cgroupFile(proc string) string {
	for _, line := range strings.Split(proc, "\n") {
		if path, ok := strings.CutPrefix(line, cgroupV2Prefix); ok {
			return filepath.Join(cgroupRoot, path, memoryMaxFile)
		}
	}
	return ""
}

// parseCap is a memory.max file's bytes, 0 for "max" (no cap) or text that is no positive number.
func parseCap(data string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(data), decimal, rssBits)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// runCap is the memory cap of the cgroup this test runs in, 0 when it cannot be read.
func runCap() int64 {
	proc, err := os.ReadFile(procCgroup)
	if err != nil {
		return 0
	}
	file := cgroupFile(string(proc))
	if file == "" {
		return 0
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return 0
	}
	return parseCap(string(data))
}

// appendLine appends line and a newline to the file at path, creating it.
func appendLine(path, line string) error {
	f, err := os.OpenFile(path, tsvFlags, filePerm)
	if err != nil {
		return err
	}
	_, werr := f.WriteString(line + "\n")
	return errors.Join(werr, f.Close())
}

// measureScale writes the benchmark project of size in a fresh directory, runs one cold
// `canon check -q` in a fresh process and removes the directory. A check that fails is no
// measurement; one killed by a signal is the row of status "killed" with the cap as its RSS.
func measureScale(t *testing.T, bin string, size scaleSize, limit int64) scaleRow {
	t.Helper()
	dir, err := os.MkdirTemp("", scaleTmpPrefix)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("remove %s: %v", dir, err)
		}
	}()
	target := benchTarget{dir: filepath.Join(dir, "bench")}
	if err := generate(benchSeed, target.dir, size.entries); err != nil {
		t.Fatalf("generate: %v", err)
	}
	row := scaleRow{size: size, status: statusOK}
	if row.canonCount, err = countCanon(target.dir); err != nil {
		t.Fatal(err)
	}
	out, m, err := measured(t, canonIn(bin, target, "check", "-q"))
	row.wall, row.rss = m.wall, m.rss
	switch {
	case errors.Is(err, errSignaled):
		row.status = statusKilled
		if limit > 0 {
			row.rss = limit
		}
	case err != nil:
		t.Fatalf("canon check -q at %dx: %v; a project with errors is not measured\n%s", size.scale, err, out)
	}
	return row
}

// IMPLEMENTATION-PLAN.md §7.6, DECISIONS 441
func TestBenchScale(t *testing.T) {
	sizes, err := scaleSizes(*flagScales, *flagUnit)
	if err != nil {
		t.Fatal(err)
	}
	if len(sizes) == 0 {
		t.Skip("guarded by -benchgen.scales: run with `make bench-scale`")
	}
	t.Log(machine())
	limit := runCap()
	t.Logf("memory cap of this run: %.1f GB (0: none read)", float64(limit)/gigabyte)
	bin := buildCanon(t)
	for i, size := range sizes {
		row := measureScale(t, bin, size, limit)
		t.Log(row.table())
		if err := appendLine(*flagScaleOut, row.tsv()); err != nil {
			t.Fatal(err)
		}
		if row.status == statusKilled {
			t.Logf("%dx was killed (more than the cap): the %d larger sizes are skipped", size.scale, len(sizes)-i-1)
			return
		}
	}
}

// IMPLEMENTATION-PLAN.md §7.6 (Scale bench), DECISIONS 441: the sizes are scales times the unit.
func TestScaleSizes(t *testing.T) {
	tests := []struct {
		name   string
		scales string
		unit   int
		want   []scaleSize
		err    error
	}{
		{"default", "1 3 5", 15000, []scaleSize{{1, 15000}, {3, 45000}, {5, 75000}}, nil},
		{"empty skips", "", 15000, nil, nil},
		{"blank skips", "  ", 15000, nil, nil},
		{"one smoke size", " 2 ", 50, []scaleSize{{2, 100}}, nil},
		{"zero scale", "1 0", 15000, nil, errBadScale},
		{"negative scale", "-3", 15000, nil, errBadScale},
		{"not a number", "1 x", 15000, nil, errBadScale},
		{"zero unit", "1", 0, nil, errBadUnit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scaleSizes(tt.scales, tt.unit)
			if !errors.Is(err, tt.err) {
				t.Fatalf("scaleSizes(%q, %d) error %v, want %v", tt.scales, tt.unit, err, tt.err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("scaleSizes(%q, %d) = %v, want %v", tt.scales, tt.unit, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("size %d = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// DECISIONS 441: one TSV line per size, tab-separated: scale, entries, files, lines, bytes,
// status, wall seconds, peak RSS bytes. A killed size records the cap as its RSS.
func TestScaleRowTSV(t *testing.T) {
	ok := scaleRow{size: scaleSize{3, 45000}, canonCount: canonCount{9, 1234, 56789}, status: statusOK,
		wall: 15600 * time.Millisecond, rss: 5_800_000_000}
	if got, want := ok.tsv(), "3\t45000\t9\t1234\t56789\tok\t15.600\t5800000000"; got != want {
		t.Errorf("ok row = %q, want %q", got, want)
	}
	killed := scaleRow{size: scaleSize{5, 75000}, status: statusKilled, wall: 40 * time.Second, rss: 12 << 30}
	if got, want := killed.tsv(), "5\t75000\t0\t0\t0\tkilled\t40.000\t12884901888"; got != want {
		t.Errorf("killed row = %q, want %q", got, want)
	}
}

// DECISIONS 441: the counts are of the `.canon` files only: files, newline-terminated lines, bytes.
func TestCountCanon(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"a.canon": "x\ny\n", "sub/b.canon": "z\n", "sub/c.json": "{}\n", "d.h": "#define A 1\n"}
	//canon:unordered each file is written under its own name
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), filePerm); err != nil {
			t.Fatal(err)
		}
	}
	got, err := countCanon(dir)
	if want := (canonCount{files: 2, lines: 3, bytes: 6}); err != nil || got != want {
		t.Errorf("countCanon = %+v, %v; want %+v, nil", got, err, want)
	}
}

// DECISIONS 441: the cap of the run is read from the memory.max of the cgroup the test runs in
// (cgroup v2): its file from /proc/self/cgroup, its value as bytes, 0 for "max" or garbage.
func TestCgroupCap(t *testing.T) {
	files := []struct{ proc, want string }{
		{"0::/user.slice/run-1.scope\n", "/sys/fs/cgroup/user.slice/run-1.scope/memory.max"},
		{"4:memory:/x\n", ""},
		{"", ""},
	}
	for _, f := range files {
		if got := cgroupFile(f.proc); got != f.want {
			t.Errorf("cgroupFile(%q) = %q, want %q", f.proc, got, f.want)
		}
	}
	limits := []struct {
		data string
		want int64
	}{{"12884901888\n", 12 << 30}, {"max\n", 0}, {"many\n", 0}, {"-5\n", 0}}
	for _, l := range limits {
		if got := parseCap(l.data); got != l.want {
			t.Errorf("parseCap(%q) = %d, want %d", l.data, got, l.want)
		}
	}
}

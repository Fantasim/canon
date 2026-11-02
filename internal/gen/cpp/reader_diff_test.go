package cppgen_test

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	cppruntime "github.com/fantasim/canonlang/internal/gen/cpp/runtime"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
	"github.com/fantasim/canonlang/internal/testkit/cxx/config"
)

const (
	// readerOracle holds `git show v0.2.0:internal/gen/cpp/runtime/<name>.txt`, in no dir the runtime text checker reads.
	readerOracle = "testdata/reader/v0.2.0"
	// readerSchema is the $schema the driver's ParseDataFile expects.
	readerSchema = "reader@1"
	// readerDriverDir is where a fuzzing run's workers find the driver its coordinator built, by content key.
	readerDriverDir = "canon-reader-diff"
	// The least a TestReaderMatchesV020 run must see, so that it never passes on nothing.
	minReaderValid, minReaderTokens, minReaderOrders = 5000, 1000, 1000
	// readerSettings are the driver's: text with key orders and without, data with and without.
	readerSettings = 4
)

// readerOracleFiles are the oracle's headers: name, sha256 and size of v0.2.0's.
var readerOracleFiles = []struct {
	name, sum string
	size      int
}{
	{"canon_runtime_json.h", "82cf616172e29e52423417a1a7e8dc4e3afd455f77a3d5769484d92751b777b4", 61061},
	{"canon_runtime.h", "812e861e62f70c7010013c1a4cae5a9326cf5d4df4e8ef269a7b73b233a6b14b", 13755},
}

// readerSources are the driver's translation units, under testdata/main.
var readerSources = []string{"reader_diff_main.cpp", "reader_oracle.cpp", "reader_subject.cpp"}

// readerReport is a summary line of the enumerating driver, one per setting.
var readerReport = regexp.MustCompile(`(?m)^(\S+): (\d+) inputs, (\d+) valid, (\d+) with Float32 tokens, (\d+) with key orders, (\d+) NUL refusals, (\d+) differences$`)

// readerSchemaKey is a data file's "$schema" member, which readerSeeds sets to readerSchema.
var readerSchemaKey = regexp.MustCompile(`"\$schema": "[^"]*"`)

// readerData is a data file of readerSchema holding body.
func readerData(body string) string { return `{"$schema": "` + readerSchema + `", ` + body + `}` }

// readerNest is n levels of arrays, or of objects under the key "a", around a 1.
func readerNest(n int, object bool) string {
	if object {
		return strings.Repeat(`{"a":`, n) + "1" + strings.Repeat("}", n)
	}
	return strings.Repeat("[", n) + "1" + strings.Repeat("]", n)
}

// readerExtraSeeds are the texts where the one-pass reader could part from v0.2.0's (DECISIONS 431): a BOM nlohmann
// skips once, a NUL it reads as the end, numbers it refuses or keeps a token of, keys a pointer escapes.
func readerExtraSeeds() []string {
	nines := strings.Repeat("9", 400)
	seeds := []string{
		"\xef\xbb\xbf\xef\xbb\xbf" + `{"mode":"auto"}`,
		"\xef\xbb\xbf\xef\xbb\xbf" + readerData(`"rows": []`),
		"\xef\xbb\xbf" + `{"zz": 1e400, "mode": "auto"}`,
		readerData(`"rows": []`) + "\x00",
		readerData(`"rows": []`) + " \x00junk",
		readerData(`"rows": ["a` + "\x00" + `"]`),
		readerData(`"k\u0000": "v\u0000", "rows": []`),
		`{"a/b": 16777217.0, "c~d": [1.000000059604644775390625], "": {"x": 3.4028235677973366e38}}`,
		readerData(`"rows": [{"w": 16777217.0, "b": 1, "a": 2}], "value": {"z": {"y": 1, "x": 2}}`),
		`[-0.0, 1e-400, -1e400]`, `[` + nines + `]`, `{"n": ` + nines + `, "m": 16777217.0}`,
		`[18446744073709551615, 18446744073709551616, -9223372036854775808, -9223372036854775809]`,
		`{"b": {"d": {"f": 1, "e": 2}, "c": 3}, "a": 4}`,
		`7`, `-1.5e3`, `true`, `null`, `"s"`, `[1, "a", null, true, {}]`,
	}
	for _, n := range []int{511, 512, 513} {
		seeds = append(seeds, readerNest(n, false), readerNest(n, true))
	}
	return seeds
}

// readerSeeds are the texts TestReaderMatchesV020 enumerates the neighbourhood of: those of parseFiles, typesFiles
// and documentOrderFiles, the canon-written data files of testdata/main with their $schema made readerSchema, and
// readerExtraSeeds.
func readerSeeds(tb testing.TB) []string {
	tb.Helper()
	fixture, err := os.ReadFile(eventConfigFixture)
	if err != nil {
		tb.Fatal(err)
	}
	var seeds []string
	for _, f := range slices.Concat(typesFiles, parseFiles, documentOrderFiles) {
		text := f.text
		if f.text == "" && f.arg == "c" {
			text = strings.Replace(string(fixture), f.from, f.to, 1)
		}
		seeds = append(seeds, text)
	}
	data, err := filepath.Glob(filepath.Join("testdata", "main", "*", "good", "*.json"))
	if err != nil || len(data) == 0 {
		tb.Fatalf("no data file under testdata/main/*/good: %v", err)
	}
	for _, path := range data {
		b, err := os.ReadFile(path)
		if err != nil {
			tb.Fatal(err)
		}
		seeds = append(seeds, readerSchemaKey.ReplaceAllLiteralString(string(b), `"$schema": "`+readerSchema+`"`))
	}
	return append(seeds, readerExtraSeeds()...)
}

// readerTree is a directory holding the driver's sources, this compiler's runtime headers as written, and the
// oracle's under reader/v0.2.0.
func readerTree(tb testing.TB) string {
	tb.Helper()
	dir := tb.TempDir()
	oracle := filepath.Join("reader", "v0.2.0")
	if err := os.MkdirAll(filepath.Join(dir, oracle), 0o755); err != nil {
		tb.Fatal(err)
	}
	files := [][2]string{{"canon_runtime.h", cppruntime.Header}, {"canon_runtime_json.h", cppruntime.JSON}}
	for _, src := range readerSources {
		files = append(files, [2]string{src, readFile(tb, filepath.Join("testdata", "main", src))})
	}
	for _, f := range readerOracleFiles {
		files = append(files, [2]string{filepath.Join(oracle, f.name), readFile(tb, filepath.Join(readerOracle, f.name))})
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f[0]), []byte(f[1]), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	return dir
}

func readFile(tb testing.TB, path string) string {
	tb.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		tb.Fatal(err)
	}
	return string(b)
}

// DECISIONS 431: the oracle is v0.2.0's reader, byte for byte.
func TestReaderOracleIsV020(t *testing.T) {
	t.Parallel()
	for _, f := range readerOracleFiles {
		b := readFile(t, filepath.Join(readerOracle, f.name))
		if sum := sha256.Sum256([]byte(b)); hex.EncodeToString(sum[:]) != f.sum || len(b) != f.size {
			t.Errorf("%s: %d bytes, sha256 %x; v0.2.0's has %d, %s", f.name, len(b), sum, f.size, f.sum)
		}
	}
}

// CODEGEN.md §5.13, §7.5, WIRE.md §3.1-§3.3, DECISIONS 431: on the seeds' neighbourhood, as a text and as a data file, with key orders and without, this compiler's reader gives v0.2.0's verdict, message, typed document, Float32 tokens and key orders, but for its one listed difference: a NUL after a data document is refused.
func TestReaderMatchesV020(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs the reader driver")
	}
	t.Parallel()
	dir := readerTree(t)
	args := []string{"enumerate"}
	for i, s := range readerSeeds(t) {
		path := filepath.Join(dir, "seed"+strconv.Itoa(i)+".json")
		if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, path)
	}
	for i, out := range buildAndRun(t, dir, readerSources, args...) {
		checkReaderReport(t, "build "+strconv.Itoa(i), out)
	}
}

// checkReaderReport holds one enumerating run to its bounds: no difference, and enough valid texts, tokens,
// orders and NUL refusals that it compared something.
func checkReaderReport(t *testing.T, build, out string) {
	t.Helper()
	lines := readerReport.FindAllStringSubmatch(out, -1)
	if len(lines) != readerSettings {
		t.Fatalf("%s: %d summary lines:\n%s", build, len(lines), out)
	}
	for _, m := range lines {
		t.Logf("%s: %s", build, m[0])
		n := make([]int, len(m))
		for i := 2; i < len(m); i++ {
			n[i], _ = strconv.Atoi(m[i])
		}
		valid, tokens, orders, nul, diffs := n[3], n[4], n[5], n[6], n[7]
		data, keepOrders := strings.HasPrefix(m[1], "data"), strings.HasSuffix(m[1], "+orders")
		switch {
		case diffs != 0:
			t.Errorf("%s: %s differs from v0.2.0's reader %d times:\n%s", build, m[1], diffs, out)
		case !data && (valid < minReaderValid || tokens < minReaderTokens):
			t.Errorf("%s: %s: %d valid, %d with tokens; want at least %d and %d", build, m[0], valid, tokens, minReaderValid, minReaderTokens)
		case !data && keepOrders && orders < minReaderOrders:
			t.Errorf("%s: %s: want at least %d with key orders", build, m[0], minReaderOrders)
		case data && nul == 0:
			t.Errorf("%s: %s: no NUL after a data document was refused", build, m[0])
		}
	}
}

// CODEGEN.md §7.5, DECISIONS 431: the same comparison on fuzzed bytes, through one persistent driver per process.
func FuzzReaderMatchesV020(f *testing.F) {
	if testing.Short() {
		f.Skip("compiles and runs the reader driver")
	}
	for _, s := range readerSeeds(f) {
		f.Add([]byte(s))
	}
	server := &readerServer{bin: readerDriver(f)}
	f.Cleanup(server.stop)
	f.Fuzz(server.check)
}

// fuzzing reports whether this process runs a fuzzing coordinator or worker, rather than the seed corpus alone.
func fuzzing() bool {
	fz, worker := flag.Lookup("test.fuzz"), flag.Lookup("test.fuzzworker")
	return (fz != nil && fz.Value.String() != "") || (worker != nil && worker.Value.String() == "true")
}

// readerDriver builds the driver with the first compiler found, in plain mode: into f.TempDir(), or, while
// fuzzing, once per content key under os.TempDir(), an atomic rename publishing it to the workers (as
// internal/testkit/cxx's libc++ wrapper is).
func readerDriver(f *testing.F) string {
	f.Helper()
	cc, include := fuzzToolchain(f)
	dir := readerTree(f)
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00", cc)
	for _, src := range readerSources {
		key, err := objectKey(cc, include, nil, dir, src)
		if err != nil {
			f.Fatal(err)
		}
		fmt.Fprintf(h, "%s\x00", key)
	}
	bin := filepath.Join(f.TempDir(), exe("driver"))
	if fuzzing() {
		bin = filepath.Join(os.TempDir(), readerDriverDir, hex.EncodeToString(h.Sum(nil)), exe("driver"))
		if _, err := os.Stat(bin); err == nil {
			return bin
		}
	}
	objs := make([]string, len(readerSources))
	for i, src := range readerSources {
		obj, err := objectOf(cc, include, nil, dir, src)
		if err != nil {
			f.Fatal(err)
		}
		objs[i] = obj
	}
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		f.Fatal(err)
	}
	tmp := bin + "." + strconv.Itoa(os.Getpid())
	if out, err := compile(cc, slices.Concat(cxx.Flags, []string{"-o", tmp}, objs), dir); err != nil {
		f.Fatalf("link: %v\n%s", err, out)
	}
	if err := os.Rename(tmp, bin); err != nil {
		f.Fatal(err)
	}
	return bin
}

// fuzzToolchain is the first C++ compiler on PATH and the nlohmann/json include directory, as cxx.Toolchain finds
// them for a *testing.T.
func fuzzToolchain(f *testing.F) (cc, include string) {
	f.Helper()
	for _, name := range []string{"g++", "clang++"} {
		if p, err := exec.LookPath(name); err == nil {
			cc = p
			break
		}
	}
	include, ok := cxx.FindNlohmann()
	if cc != "" && ok {
		return cc, include
	}
	if config.RequireCxx() {
		f.Fatal("no C++ compiler or nlohmann/json.hpp")
	}
	f.Skip("no C++ compiler or nlohmann/json.hpp")
	return "", ""
}

// readerServer is a running `serve` driver: started on the first input, restarted after a crash.
type readerServer struct {
	mu  sync.Mutex
	bin string
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Reader
}

func (s *readerServer) start() error {
	cmd := exec.Command(s.bin, "serve")
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	s.cmd, s.in, s.out = cmd, in, bufio.NewReader(out)
	return nil
}

// stop ends the driver: it exits at the end of its input.
func (s *readerServer) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.end()
}

func (s *readerServer) end() {
	if s.cmd == nil {
		return
	}
	_ = s.in.Close()
	_ = s.cmd.Wait()
	s.cmd = nil
}

// check sends text to the driver and fails on any answer but "ok"; a driver that dies on it is not a difference.
func (s *readerServer) check(t *testing.T, text []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		if err := s.start(); err != nil {
			t.Fatal(err)
		}
	}
	frame := binary.LittleEndian.AppendUint32(nil, uint32(len(text)))
	_, err := s.in.Write(append(frame, text...))
	line := ""
	if err == nil {
		line, err = s.out.ReadString('\n')
	}
	if err != nil {
		_ = s.cmd.Process.Kill()
		s.end()
		t.Fatalf("driver crashed on input %q: %v", text, err)
	}
	if line = strings.TrimSuffix(line, "\n"); line != "ok" {
		t.Error(line)
	}
}

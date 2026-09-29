package cppgen_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// maxCompiles bounds the compiler processes this package's tests run at once: each one that
// includes nlohmann/json takes some hundreds of MB, and the -race run is memory-capped.
const maxCompiles = 4

var (
	// compileSlots holds one token per running compiler process.
	compileSlots = make(chan struct{}, maxCompiles)
	// objectDir holds the objects every test shares (TestMain).
	objectDir string
	// objects are the compiled objects by objectKey: tests compiling the same sources share one build.
	objects sync.Map
)

// object is one compiled source, built once.
type object struct {
	once sync.Once
	path string
	err  error
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "cppgen-objects-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	objectDir = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// compile runs one compiler process once a slot is free.
func compile(cc string, args []string, dir string) ([]byte, error) {
	compileSlots <- struct{}{}
	defer func() { <-compileSlots }()
	ctx, cancel := context.WithTimeout(context.Background(), cxx.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cc, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// objectKey is what compiling src in dir reads: the compiler, its flags, the source and every
// header under dir, each by path and content.
func objectKey(cc, include string, mode []string, dir, src string) (string, error) {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00", cc, include, strings.Join(cxx.Flags, " "), strings.Join(mode, " "), src)
	files := []string{src}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".h") {
			rel, relErr := filepath.Rel(dir, path)
			files = append(files, rel)
			return relErr
		}
		return err
	})
	if err != nil {
		return "", err
	}
	slices.Sort(files[1:])
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", f, len(b))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// objectOf compiles src of dir into an object, or reuses the one an identical build made.
func objectOf(cc, include string, mode []string, dir, src string) (string, error) {
	key, err := objectKey(cc, include, mode, dir, src)
	if err != nil {
		return "", err
	}
	v, _ := objects.LoadOrStore(key, &object{})
	o := v.(*object)
	o.once.Do(func() {
		o.path = filepath.Join(objectDir, key+".o")
		args := append(append(append([]string(nil), cxx.Flags...), mode...), "-I", dir, "-I", include, "-c", "-o", o.path, filepath.Join(dir, src))
		if out, err := compile(cc, args, dir); err != nil {
			o.err = fmt.Errorf("%s %s: %s: %w\n%s", filepath.Base(cc), strings.Join(mode, " "), src, err, out)
		}
	})
	return o.path, o.err
}

// build is one compiler and mode's build of sources in dir into bin.
type build struct {
	cc, include, bin, dir string
	mode, sources         []string
}

// buildOne compiles each source (concurrently, sharing identical builds), links them into one
// binary and runs it with args; its output, or what failed.
func buildOne(b build, args []string) (string, error) {
	cc, include, mode, bin, dir, sources := b.cc, b.include, b.mode, b.bin, b.dir, b.sources
	objs, errs := make([]string, len(sources)), make([]error, len(sources))
	var wg sync.WaitGroup
	for i, s := range sources {
		wg.Go(func() { objs[i], errs[i] = objectOf(cc, include, mode, dir, s) })
	}
	wg.Wait()
	if err := joinErrs(errs); err != nil {
		return "", err
	}
	link := append(append(append([]string(nil), cxx.Flags...), mode...), "-o", bin)
	if out, err := compile(cc, append(link, objs...), dir); err != nil {
		return "", fmt.Errorf("%s %s: link: %w\n%s", filepath.Base(cc), strings.Join(mode, " "), err, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cxx.Timeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	run := exec.CommandContext(ctx, bin, args...)
	run.Stdout, run.Stderr = &stdout, &stderr
	if err := run.Run(); err != nil {
		return "", fmt.Errorf("%s %s: run: %w\n%s%s", filepath.Base(cc), strings.Join(mode, " "), err, stdout.String(), stderr.String())
	}
	return textOut(stdout.String()), nil
}

// textOut is a test driver's output with the "\r\n" line ends of Windows' text-mode stdout
// made "\n", as every other platform prints them.
func textOut(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// exe is name as an executable's file name here: Windows runs only a name ending ".exe",
// which go build -o does not add.
func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func joinErrs(errs []error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// buildAndRun compiles sources in dir with every compiler cxx.Toolchain finds and every mode of
// cxx.Modes, all builds at once, runs each binary with args, and returns the output of each run
// in compiler then mode order; a failed build or a non-zero exit fails the test.
func buildAndRun(t *testing.T, dir string, sources []string, args ...string) []string {
	t.Helper()
	compilers, include := cxx.Toolchain(t)
	n := len(cxx.Modes)
	outs, errs := make([]string, len(compilers)*n), make([]error, len(compilers)*n)
	var wg sync.WaitGroup
	for c, cc := range compilers {
		for m, mode := range cxx.Modes {
			bin := filepath.Join(dir, filepath.Base(cc)+"-"+string(rune('a'+m)))
			b := build{cc: cc, include: include, bin: bin, dir: dir, mode: mode, sources: sources}
			wg.Go(func() { outs[c*n+m], errs[c*n+m] = buildOne(b, args) })
		}
	}
	wg.Wait()
	if err := joinErrs(errs); err != nil {
		t.Fatal(err)
	}
	return outs
}

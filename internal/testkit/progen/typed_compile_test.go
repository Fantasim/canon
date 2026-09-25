package progen_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/fantasim/canonlang/internal/build"
)

const (
	goTestTimeout      = 90 * time.Second // below childTimeout: a stuck module is a gofail, not a hang
	typedReplayTimeout = 2 * goTestTimeout
	goVersionPre       = "go1."
	typedFallback      = "1.25" // go.mod's "go" directive when this binary's toolchain names no release
)

var (
	reGoError = regexp.MustCompile(`(?m)^\S+\.go:\d+(?::\d+)?: (.*)$`)
	reGoFail  = regexp.MustCompile(`(?m)^\s*--- FAIL: (\S+)`)
	reSmoke   = regexp.MustCompile(`(?m)` + smokeMark + `(\S+)`)
	reDigits  = regexp.MustCompile(`\d+`)
)

// compileTyped is "" when a module of the build's Go outputs compiles and passes its tests: the
// smoke test, beside the build's JSON outputs it reads, and the build's conformance test.
func compileTyped(outputs []build.Output, smoke []byte) verdict {
	dir, err := os.MkdirTemp("", "progen-typed-*")
	if err != nil {
		return harnessVerdict(err)
	}
	defer os.RemoveAll(dir)
	mod := "module " + typedModule + "\n\ngo " + goLangVersion() + "\n"
	files := map[string][]byte{"go.mod": []byte(mod), filepath.Join("smoke", "smoke_test.go"): smoke}
	for _, o := range outputs {
		if rel, ok := strings.CutPrefix(o.Path, typedGoOut); ok {
			files[filepath.Join("go", filepath.FromSlash(rel))] = o.Content
		}
		if rel, ok := strings.CutPrefix(o.Path, typedDataOut); ok {
			files[filepath.Join("data", filepath.FromSlash(rel))] = o.Content
		}
	}
	for name, content := range files { //canon:unordered each file written on its own
		if err := writeTypedFile(filepath.Join(dir, name), content); err != nil {
			return harnessVerdict(err)
		}
	}
	return runTypedTests(dir)
}

func writeTypedFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

// runTypedTests runs the module's tests, compiling the generated Go on the way, under
// goTestTimeout.
func runTypedTests(dir string) verdict {
	ctx, cancel := context.WithTimeout(context.Background(), goTestTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
	cmd.WaitDelay = waitDelay
	out, err := cmd.CombinedOutput()
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return verdict{Kind: kindGoFail, Sig: kindGoFail + " timeout", Text: "go test: no end after " + goTestTimeout.String() + "\n" + string(out)}
	case err != nil:
		return verdict{Kind: kindGoFail, Sig: goFailSig(string(out)), Text: "go test: " + err.Error() + "\n" + string(out)}
	}
	return verdict{}
}

// goFailSig names a failed go test by what fails, free of the names a seed draws (digits cut):
// each compile error's message, each failing test, each smoke check's shape.
func goFailSig(out string) string {
	var parts []string
	for _, re := range []*regexp.Regexp{reGoError, reGoFail, reSmoke} {
		for _, m := range re.FindAllStringSubmatch(out, -1) {
			parts = append(parts, reDigits.ReplaceAllString(m[len(m)-1], "N"))
		}
	}
	if len(parts) == 0 {
		first, _, _ := strings.Cut(out, "\n")
		parts = append(parts, unplaced(first))
	}
	slices.Sort(parts)
	return kindGoFail + " " + strings.Join(slices.Compact(parts), shapeSep)
}

// generic is a name a seed draws (v0, R1, probe2) with its digits cut, for a signature.
func generic(name string) string { return reDigits.ReplaceAllString(name, "N") }

// goLangVersion is this test binary's Go release, for go.mod's "go" directive: the toolchain
// that runs the module's tests (GOTOOLCHAIN=local).
func goLangVersion() string {
	v := runtime.Version()
	if !strings.HasPrefix(v, goVersionPre) {
		return typedFallback
	}
	v = strings.TrimPrefix(v, "go")
	if i := strings.IndexAny(v, " -+"); i >= 0 {
		v = v[:i]
	}
	return v
}

// harnessVerdict is a failure of the harness itself, signed by the operation that failed.
func harnessVerdict(err error) verdict {
	sig := kindHarness
	var pe *fs.PathError
	if errors.As(err, &pe) {
		sig += " " + pe.Op + " " + pe.Err.Error()
	}
	return verdict{Kind: kindHarness, Sig: sig, Text: err.Error()}
}

func outputMap(outs []build.Output) map[string][]byte {
	m := make(map[string][]byte, len(outs))
	for _, o := range outs {
		m[o.Path] = o.Content
	}
	return m
}

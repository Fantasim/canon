package golden

import (
	"context"
	"io/fs"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/cxx"
	cxxconfig "github.com/fantasim/canonlang/internal/testkit/cxx/config"
)

// cppExt is a golden translation unit's extension: findGoldenCppFiles' filter.
const cppExt = ".cpp"

// windowsGOOS is compared against runtime.GOOS to gate TestGoldensCompileMSVC.
const windowsGOOS = "windows"

// TestGoldensCompileMSVC compiles every golden with cl.exe (CODEGEN.md §9).
func TestGoldensCompileMSVC(t *testing.T) {
	if runtime.GOOS != windowsGOOS {
		t.Skip("MSVC golden compile check only runs on Windows")
	}
	job := msvcJob{dir: t.TempDir()}
	job.clPath, job.include = msvcToolchain(t)
	files, err := findGoldenCppFiles(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no golden .cpp files found")
	}
	for _, src := range files {
		for i, mode := range cxx.MSVCModes {
			job.compile(t, src, mode, i)
		}
	}
}

// msvcToolchain finds cl.exe and nlohmann/json, or gates the test on whichever is missing.
func msvcToolchain(t *testing.T) (clPath, include string) {
	t.Helper()
	clPath, clOK := cxx.DetectMSVC(t)
	include, nlohmannOK := cxx.FindNlohmann()
	if clOK && nlohmannOK {
		return clPath, include
	}
	msg := "MSVC toolchain incomplete: cl.exe found=" + strconv.FormatBool(clOK) +
		", nlohmann/json found=" + strconv.FormatBool(nlohmannOK)
	if cxxconfig.RequireMSVC() {
		t.Fatal(msg)
	}
	t.Skip(msg)
	return "", ""
}

// msvcJob is TestGoldensCompileMSVC's fixed inputs: one cl.exe, one nlohmann/json, one temp dir.
type msvcJob struct {
	clPath, dir, include string
}

// compile builds src with cl.exe alone, in mode, failing the test on a diagnostic.
func (j msvcJob) compile(t *testing.T, src string, mode []string, modeIdx int) {
	t.Helper()
	obj := filepath.Join(j.dir, strconv.Itoa(modeIdx)+"-"+strings.TrimSuffix(filepath.Base(src), cppExt)+".obj")
	args := cxx.MSVCCompileArgs(mode, filepath.Dir(src), j.include, obj, src)
	ctx, cancel := context.WithTimeout(context.Background(), cxx.Timeout)
	defer cancel()
	if out, err := exec.CommandContext(ctx, j.clPath, args...).CombinedOutput(); err != nil {
		t.Errorf("%s %v: %v\n%s", src, mode, err, out)
	}
}

// TestFindGoldenCppFilesDiscoversKnownGoldens proves findGoldenCppFiles' discovery can't
// silently stop finding a golden (so TestGoldensCompileMSVC can't silently narrow, even where
// it only skips): the pipeline and features.dependent .gen.cpp files.
func TestFindGoldenCppFilesDiscoversKnownGoldens(t *testing.T) {
	files, err := findGoldenCppFiles(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(examplesDir, "pipeline", "expected", "pipeline.gen.cpp"),
		filepath.Join(examplesDir, "features", "dependent", "expected", "dependent", "out", "cpp", "dependent.gen.cpp"),
	}
	for _, w := range want {
		if !containsPath(files, w) {
			t.Errorf("findGoldenCppFiles(%q) = %v, want it to contain %q", examplesDir, files, w)
		}
	}
}

// containsPath reports whether files holds path.
func containsPath(files []string, path string) bool {
	for _, f := range files {
		if f == path {
			return true
		}
	}
	return false
}

// findGoldenCppFiles is every *.cpp file under an expected/ tree of root, sorted: the C++
// golden translation units TestGoldensCompileMSVC compiles.
func findGoldenCppFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == cppExt && underExpectedDir(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// underExpectedDir reports whether path has an expectedDir component.
func underExpectedDir(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == expectedDir {
			return true
		}
	}
	return false
}

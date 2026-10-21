//go:build linux

package cppgen_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

const (
	// bulkMaxRSSKB is the handoff's 1.5 GB bound on the compiler's peak RSS, in KB as ru_maxrss is.
	bulkMaxRSSKB = 1536 * 1024
	// bulkMaxLine bounds a generated line: the handoff's ids.gen.cpp had one of 340 KB.
	bulkMaxLine = 1000
	bulkSource  = "bulk.gen.cpp"
)

// bulkFlags are the release flags of the handoff on top of cxx.Flags: the later -O3 wins.
var bulkFlags = []string{"-O3", "-g"}

// bulkWant is what testdata/main/bulk_main.cpp prints for the bulk fixture.
const bulkWant = "names=11500 TID_GAME_TEXT_00000 TID_GAME_TEXT_05750 TID_GAME_TEXT_11499\n" +
	"values=11500 0 17250 34497\n" +
	"aliases=1 TID_ALIAS small=2 empty=0 file=defineText.h\n" +
	"item II_ITEM_00000 Item 0 level=0 price=0 rare=1 cooldown=0 tags=t0,u\n" +
	"  header=small.h key=small note=note 0 grade=low coins=0\n" +
	"item II_ITEM_00001 Item 1 level=1 price=10 rare=0 cooldown=100 tags=t1,u\n" +
	"  header=defineText.h key=big note=none grade=mid coins=-1\n" +
	"item II_ITEM_03700 Item 3700 level=100 price=37000 rare=0 cooldown=370000 tags=t0,u\n" +
	"  header=defineText.h key=big note=none grade=mid coins=-1\n" +
	"item II_ITEM_07399 Item 7399 level=79 price=73990 rare=1 cooldown=739900 tags=t4,u\n" +
	"  header=defineText.h key=big note=note 7399 grade=mid coins=7399\n" +
	"items=7400 find=Item 1234\n" +
	"weights=3000 first=II_ITEM_00000:1 last=3000\n" +
	"groups=5000 first=group0000:0 last=3:m4999_2\n" +
	"grid=100:100 102 9999\n"

// compileRSS compiles with cc once a slot is free and returns the compiler's peak resident
// memory in KB: wait4's usage of the child, its own subprocesses (cc1plus) included.
func compileRSS(t *testing.T, cc string, args []string, dir string) int64 {
	t.Helper()
	compileSlots <- struct{}{}
	defer func() { <-compileSlots }()
	ctx, cancel := context.WithTimeout(context.Background(), cxx.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cc, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", filepath.Base(cc), err, out)
	}
	usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		t.Fatalf("%s: no resource usage", filepath.Base(cc))
	}
	return usage.Maxrss
}

// DECISIONS 392, CODEGEN.md §7.3: bulk and nested data generate no long line, compile at -O3 -g within bulkMaxRSSKB and read back.
func TestBulkCompilesBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a 7 MB generated source at -O3 -g")
	}
	t.Parallel()
	compilers := cxx.Compilers(t)
	dir := t.TempDir()
	files := generate(t, bulkPackage())
	writeTree(t, dir, files, "bulk_main.cpp")
	for _, f := range files {
		for i, l := range strings.Split(string(f.Content), "\n") {
			if len(l) > bulkMaxLine {
				t.Fatalf("%s:%d is %d bytes long", f.Path, i+1, len(l))
			}
		}
	}
	for _, cc := range compilers {
		name := filepath.Base(cc)
		obj := filepath.Join(dir, name+".o")
		args := append(append(append([]string(nil), cxx.Flags...), bulkFlags...), "-I", dir, "-c", "-o", obj, filepath.Join(dir, bulkSource))
		rss := compileRSS(t, cc, args, dir)
		t.Logf("%s -O3 -g: peak RSS %d MB", name, rss/1024)
		if rss > bulkMaxRSSKB {
			t.Errorf("%s: peak RSS %d KB, over %d KB", name, rss, bulkMaxRSSKB)
		}
		bin := filepath.Join(dir, exe(name+"-bulk"))
		link := append(append([]string(nil), cxx.Flags...), "-I", dir, "-o", bin, filepath.Join(dir, "main.cpp"), obj)
		if out, err := compile(cc, link, dir); err != nil {
			t.Fatalf("%s: link: %v\n%s", name, err, out)
		}
		out, err := exec.Command(bin).Output()
		if err != nil {
			t.Fatalf("%s: run: %v", name, err)
		}
		if got := textOut(string(out)); got != bulkWant {
			t.Errorf("%s: got:\n%s\nwant:\n%s", name, got, bulkWant)
		}
	}
}

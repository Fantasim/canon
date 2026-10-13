package cppgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// openUserFiles are the texts of demo.user's Holder: an unknown member of an enum another package opens is kept, a closed enum of its own still refuses one.
var openUserFiles = []struct{ name, text string }{
	{"good.json", `{"dst": "DST_NEW", "dsts": ["DST_HP", "DST_Y"], "stat": "ATK", "fixed": "high"}`},
	{"fixed.json", `{"dst": "DST_HP", "dsts": [], "stat": "ATK", "fixed": "mid"}`},
	{"kind.json", `{"dst": 3, "dsts": [], "stat": "ATK", "fixed": "low"}`},
}

// DECISIONS 362, CODEGEN.md §2.8, §5.2: another package's types-mode emit names and reads the opened enums of its import.
func TestOpenEnumsOfAnImportCompileAndRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base := openPackage()
	for _, p := range []*ir.Package{base, openUserPackage(base)} {
		out := filepath.Join(dir, filepath.FromSlash(cppEmit(p).Dir))
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, out, generate(t, p))
	}
	copyFile(t, filepath.Join("testdata", "main", "open_user_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	args := []string{dir}
	for _, f := range openUserFiles {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.text), 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, f.name)
	}
	want := "good.json: dst=DST_NEW known=0 dsts=DST_HP+ DST_Y? stat=ATK fixed=high\n" +
		"fixed.json: error Holder: $.fixed: unknown value mid\n" +
		"kind.json: error Holder: $.dst: expected a string\n"
	for _, got := range buildAndRun(t, dir, []string{"main.cpp", "out/open/open.gen.cpp", "out/user/user.gen.cpp"}, args...) {
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	}
}

// openRefdepFiles are the Rewards of demo.refdep: each ref key of the table is found whatever order the members are declared in, a key it lacks (an unknown member, a Never arm) is no branch; the same lines whether Shape is opened too (a branch value held by address) or not.
var openRefdepFiles = []struct{ name, text, want string }{
	{"beat.json", `{"event": "e_beat", "param": 5, "points": 1}`, "event=e_beat param=count 5 points=1"},
	{"reach.json", `{"event": "e_reach", "param": "home", "points": 2}`, "event=e_reach param=place home points=2"},
	{"absent.json", `{"event": "e_zzz", "points": 3}`, "event=e_zzz param=none points=3"},
	{"day.json", `{"event": "e_day", "points": 4}`, "event=e_day param=none points=4"},
	{"unknown.json", `{"event": "e_zzz", "param": 1, "points": 3}`, "error Reward: $.param: no branch for this value"},
	{"never.json", `{"event": "e_day", "param": 1, "points": 3}`, "error Reward: $.param: no branch for this value"},
	{"wrong.json", `{"event": "e_beat", "param": "x", "points": 3}`, "error Reward: $.param: expected an integer"},
	{"wrongplace.json", `{"event": "e_reach", "param": 1, "points": 3}`, "error Reward: $.param: expected a string"},
}

// CODEGEN.md §5.6, DECISIONS 350, 362: a branch table whose ref key is an opened enum is keyed by the wire, in wire byte order, and holds an opened enum's member by address; every table is a constant, so a decode from another file's static initialiser works.
func TestOpenEnumBranchTableCompilesAndRuns(t *testing.T) {
	t.Parallel()
	for _, shape := range []bool{false, true} {
		dir := t.TempDir()
		out := filepath.Join(dir, "out", "refdep")
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, out, generate(t, openRefdepPackage(shape)))
		copyFile(t, filepath.Join("testdata", "main", "open_refdep_main.cpp"), filepath.Join(dir, "main.cpp"), same)
		copyFile(t, filepath.Join("testdata", "main", "open_refdep_early.cpp"), filepath.Join(dir, "early.cpp"), same)
		args, want := []string{dir}, "early: ok\n"
		for _, f := range openRefdepFiles {
			if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.text), 0o644); err != nil {
				t.Fatal(err)
			}
			args = append(args, f.name)
			want += f.name + ": " + f.want + "\n"
		}
		// early.cpp is linked before the generated file: its static initialiser decodes first.
		for _, got := range buildAndRun(t, dir, []string{"early.cpp", "main.cpp", "out/refdep/refdep.gen.cpp"}, args...) {
			if got != want {
				t.Errorf("shape opened %v: got:\n%s\nwant:\n%s", shape, got, want)
			}
		}
	}
}

// DECISIONS 363: a @text fn whose result is a record gets no Decode<Fn>File, and is no refusal.
func TestOpenTextDecodersOnlyForMapsAndLists(t *testing.T) {
	t.Parallel()
	p := openPackage()
	p.TextFns = append(p.TextFns, &ir.ExportFn{Name: "single", Result: ir.TypeRef{Kind: types.Record, Named: p.Types[3]}})
	for _, f := range generate(t, p) {
		if strings.HasSuffix(f.Path, ".gen.h") && (strings.Contains(string(f.Content), "DecodeSingleFile") || !strings.Contains(string(f.Content), "DecodeRatesFile")) {
			t.Errorf("%s: want DecodeRatesFile and no DecodeSingleFile", f.Path)
		}
	}
}

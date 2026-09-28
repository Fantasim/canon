package golden

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// eventsFixture is the committed EventConfig.json that examples/resource/events loads.
var eventsFixture = filepath.Join("_fixtures", "resource", "Server", "Event", "EventConfig.json")

// eventsCases are the fixture, then broken copies (from replaced by to), each refused with its path (CODEGEN.md §5.13).
var eventsCases = []struct{ name, from, to, want string }{
	{"EventConfig.json", "", "", "version=1 | madrigal_bang_hunt spawn_monster MI_BANG1=44 lifetime=1800000 windows=2 roll=local_budget" +
		" | moonstone_rain monster_drop_inject II_GEN_MAT_MOONSTONE count=1..2 levels=20..60 windows=1 roll=authoritative"},
	{"define.json", `"MI_BANG1"`, `"MI_BANG9"`, "error EventConfig: $.events[0].monsterId: define MI_BANG9 is not in this program's Monsters table"},
	{"member.json", `"day": "Wed"`, `"day": "wed"`, "error EventConfig: $.events[1].schedule[0].day: unknown value wed"},
	{"tag.json", `"type": "spawn_monster"`, `"type": "spawn"`, "error EventConfig: $.events[0].type: unknown case spawn"},
	{"fraction.json", `"levelMax": 60`, `"levelMax": 60.5`, "error EventConfig: $.events[1].levelMax: expected an integer"},
	{"unit.json", `"monsterLifetimeSec": 1800`, `"monsterLifetimeSec": 0.0001`, "error EventConfig: $.events[0].monsterLifetimeSec: expected a whole number of milliseconds"},
	{"missing.json", `"version": 1,`, ``, "error EventConfig: $.version: missing"},
}

// buildCpp builds the C++ emits of packages in a copy of examples/, roots redirected as
// runExample's are, and returns the copy and its roots; a finding fails the test.
func buildCpp(t *testing.T, packages ...string) (proj string, roots map[string]string) {
	t.Helper()
	root, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	proj = filepath.Join(tmp, "proj")
	if err := copyProject(proj, root); err != nil {
		t.Fatal(err)
	}
	roots = exampleRoots(proj, tmp)
	p, err := canon.Open(filepath.ToSlash(proj), canon.Options{Roots: roots, Cache: "off"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	res, err := p.Build(context.Background(), canon.BuildOptions{Packages: packages, Targets: []canon.Target{canon.TargetCpp}})
	if err != nil || res.Check.Summary.Errors > 0 {
		t.Fatalf("build: %v, %+v", err, res.Check.Findings)
	}
	return proj, roots
}

// CODEGEN.md §5.8, decision 222: game.items' C++ data emit, refing three define tables, builds and compiles.
func TestItemsCppCompiles(t *testing.T) {
	_, roots := buildCpp(t, "game.items")
	gen := filepath.Join(roots["source"], "Generated")
	compilers, include := cxx.Toolchain(t)
	for _, cc := range compilers {
		for _, mode := range cxx.Modes {
			args := append(append(append([]string(nil), cxx.Flags...), mode...), "-fsyntax-only", "-I", include, filepath.Join(gen, "items", "items.gen.cpp"))
			ctx, cancel := context.WithTimeout(context.Background(), cxx.Timeout)
			out, err := exec.CommandContext(ctx, cc, args...).CombinedOutput()
			cancel()
			if err != nil {
				t.Fatalf("%s %v: %v\n%s", filepath.Base(cc), mode, err, out)
			}
		}
	}
}

// M3 acceptance 6, CODEGEN.md §5.13: events' built C++ compiles; Decode accepts the fixture, refuses broken copies.
func TestEventsTypesDecode(t *testing.T) {
	proj, roots := buildCpp(t, "resource.events", "sovcommon.time")
	gen := filepath.Join(roots["source"], "Generated")
	fixture, err := os.ReadFile(filepath.Join(proj, eventsFixture))
	if err != nil {
		t.Fatal(err)
	}
	var args, want []string
	for _, c := range eventsCases {
		if c.from != "" && !strings.Contains(string(fixture), c.from) {
			t.Fatalf("%s: the fixture has no %s", c.name, c.from)
		}
		path := filepath.Join(gen, c.name)
		if err := os.WriteFile(path, []byte(strings.Replace(string(fixture), c.from, c.to, 1)), filePerm); err != nil {
			t.Fatal(err)
		}
		args, want = append(args, path), append(want, c.want)
	}
	driver, err := os.ReadFile(filepath.Join(cppCompileDir, "events_main.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gen, "main.cpp"), driver, filePerm); err != nil {
		t.Fatal(err)
	}
	for _, out := range buildAndRun(t, gen, []string{"main.cpp", "events/events.gen.cpp", "time/time.gen.cpp"}, args...) {
		if got := strings.TrimSuffix(out, "\n"); got != strings.Join(want, "\n") {
			t.Errorf("driver output:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
		}
	}
}

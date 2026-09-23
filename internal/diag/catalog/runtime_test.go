package catalog

import (
	"errors"
	"maps"
	"os"
	"slices"
	"testing"
	"testing/fstest"
)

// ERRORS.md §1.6: the runtime helpers of the pipeline goldens use only listed pairs.
func TestCheckRuntimeAcceptsTheGoldenHelpers(t *testing.T) {
	c := parseSpec(t)
	fsys := fstest.MapFS{}
	//canon:unordered each file fills its own key of a map
	for name, golden := range map[string]string{
		"go/runtime/rt.go.txt":            "../../../examples/pipeline/expected/go/rt/rt.go",
		"cpp/runtime/canon_runtime.h.txt": "../../../examples/pipeline/expected/canon_runtime.h",
	} {
		data, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		fsys[name] = &fstest.MapFile{Data: data}
	}
	if err := c.CheckRuntime(fsys); err != nil {
		t.Error(err)
	}
}

// ERRORS.md §2.1: an unlisted pair or a call without two literals is refused.
func TestCheckRuntimeRefuses(t *testing.T) {
	c := parseSpec(t)
	tests := map[string]string{
		"unlisted text":    `Fail("E4101", "integer overflow in ^")`,
		"unlisted code":    `canonFail("E4109", "clamp with lo > hi");`,
		"not a literal":    `OnEvalError("E4102", msg);`,
		"split over lines": "OnEvalError(\"E4102\",\n\"integer division by zero\");",
	}
	for _, name := range slices.Sorted(maps.Keys(tests)) {
		text := tests[name]
		t.Run(name, func(t *testing.T) {
			fsys := fstest.MapFS{"ts/runtime/canon_runtime.ts.txt": &fstest.MapFile{Data: []byte(text)}}
			if err := c.CheckRuntime(fsys); !errors.Is(err, errRuntime) {
				t.Errorf("got %v, want %v", err, errRuntime)
			}
		})
	}
	outside := fstest.MapFS{"go/emit.go": &fstest.MapFile{Data: []byte(`Fail("E4101", "x")`)}}
	if err := c.CheckRuntime(outside); err != nil {
		t.Errorf("a file outside runtime/ was checked: %v", err)
	}
}

func parseSpec(t *testing.T) *Catalog {
	t.Helper()
	doc, plan := readSpec(t)
	c, err := Parse([]byte(doc), plan)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

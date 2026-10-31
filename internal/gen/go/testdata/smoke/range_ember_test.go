package world_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	p "example.com/data/game/world/out/go"
)

// CODEGEN.md §2.8, §5.15: each run of cases.json loads its data file with LoadZones and prints "ok" or "error " and the loader's message, the directory of the file left out.
func TestRangeForeignReaders(t *testing.T) {
	raw, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var runs []struct{ Name, Want string }
	if err := json.Unmarshal(raw, &runs); err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		_, err := p.LoadZones("testdata/" + r.Name + ".json")
		got := "ok"
		if err != nil {
			got = "error " + strings.TrimPrefix(err.Error(), "testdata/")
		}
		if got != r.Want {
			t.Errorf("%s: got %q, want %q", r.Name, got, r.Want)
		}
	}
}

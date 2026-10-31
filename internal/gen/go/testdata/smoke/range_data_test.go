package items

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// CODEGEN.md §5.15, §5.9: each run of cases.json loads its data file with LoadItems and prints "ok" or "error " and the loader's message, the directory of the file left out.
func TestRangeData(t *testing.T) {
	raw, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var runs []struct{ Name, Want string }
	if err := json.Unmarshal(raw, &runs); err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		_, err := LoadItems("testdata/" + r.Name + ".json")
		got := "ok"
		if err != nil {
			got = "error " + strings.TrimPrefix(err.Error(), "testdata/")
		}
		if got != r.Want {
			t.Errorf("%s: got %q, want %q", r.Name, got, r.Want)
		}
	}
}

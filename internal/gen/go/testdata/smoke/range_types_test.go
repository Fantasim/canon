package rangeprobe

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// CODEGEN.md §5.15: each run of cases.json decodes its text as a Probe (or, with a leading @, as the file of the @text fn caps) and prints "ok" or "error " and the decoder's message.
func TestRangeCases(t *testing.T) {
	raw, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var runs []struct{ Name, Text, Want string }
	if err := json.Unmarshal(raw, &runs); err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		var err error
		if name, ok := strings.CutPrefix(r.Name, "@"); ok {
			_, err = DecodeCapsFile([]byte(r.Text))
			r.Name = name
		} else {
			_, err = DecodeProbe([]byte(r.Text))
		}
		got := "ok"
		if err != nil {
			got = "error " + err.Error()
		}
		if got != r.Want {
			t.Errorf("%s: got %q, want %q", r.Name, got, r.Want)
		}
	}
}

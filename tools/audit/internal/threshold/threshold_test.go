package threshold

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// validText is a complete file: every key once, each with value v.
func validText(v string) string {
	var b strings.Builder
	b.WriteString("# a comment\n\n")
	for _, sp := range specs {
		b.WriteString(sp.key + "\t" + v + "\n")
	}
	return b.String()
}

func TestShippedFileParses(t *testing.T) {
	s, err := Load(filepath.Join("..", "..", FileName))
	if err != nil {
		t.Fatal(err)
	}
	for _, sp := range specs {
		if *sp.field(&s) < 1 {
			t.Errorf("%s: value %d, want a positive limit", sp.key, *sp.field(&s))
		}
	}
}

func TestParseValid(t *testing.T) {
	s, err := parse(validText("7"), "t")
	if err != nil {
		t.Fatal(err)
	}
	if s.FnLines != 7 || s.DiagTextWords != 7 {
		t.Fatalf("parse = %+v, want every value 7", s)
	}
}

func TestParseRefuses(t *testing.T) {
	good := validText("7")
	tests := []struct {
		name, text string
		want       error
	}{
		{"unknown key", good + "fn-length\t60\n", errUnknown},
		{"missing key", strings.Replace(good, "fn-params\t7\n", "", 1), errMissing},
		{"duplicate key", good + "fn-params\t7\n", errDuplicate},
		{"zero", strings.Replace(good, "fn-params\t7", "fn-params\t0", 1), errValue},
		{"negative", strings.Replace(good, "fn-params\t7", "fn-params\t-7", 1), errValue},
		{"leading zero", strings.Replace(good, "fn-params\t7", "fn-params\t07", 1), errValue},
		{"not an integer", strings.Replace(good, "fn-params\t7", "fn-params\t7.5", 1), errValue},
		{"trailing space", strings.Replace(good, "fn-params\t7", "fn-params\t7 ", 1), errValue},
		{"carriage return", strings.Replace(good, "fn-params\t7", "fn-params\t7\r", 1), errValue},
		{"percent over 100", strings.Replace(good, "comment-ratio-percent\t7", "comment-ratio-percent\t101", 1), errValue},
		{"overflow", strings.Replace(good, "fn-params\t7", "fn-params\t99999999999999999999", 1), errValue},
		{"space separator", strings.Replace(good, "fn-params\t7", "fn-params 7", 1), errMalformed},
		{"extra column", strings.Replace(good, "fn-params\t7", "fn-params\t7\tmax", 1), errMalformed},
		{"indented comment", good + " # note\n", errMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parse(tt.text, "t"); !errors.Is(err, tt.want) {
				t.Fatalf("parse error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestParseReportsEveryProblem(t *testing.T) {
	text := strings.Replace(validText("7"), "fn-params\t7", "fn-params\tx", 1) + "nope\t1\n"
	_, err := parse(text, "t")
	if !errors.Is(err, errValue) || !errors.Is(err, errUnknown) {
		t.Fatalf("parse error = %v, want both the bad value and the unknown key", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), FileName)); !errors.Is(err, errRead) {
		t.Fatalf("Load error = %v, want errRead", err)
	}
}

func TestExpand(t *testing.T) {
	s := Set{FnLines: 60, FnParams: 5}
	got, err := s.Expand("over {fn-lines} lines, {fn-params} params, interface{} kept")
	if err != nil || got != "over 60 lines, 5 params, interface{} kept" {
		t.Fatalf("Expand = %q, %v", got, err)
	}
	if _, err := s.Expand("over {fn-length} lines"); !errors.Is(err, errUnknown) {
		t.Fatalf("Expand of an unknown key: error = %v, want errUnknown", err)
	}
}

func TestRaised(t *testing.T) {
	cur, err := parse(validText("7"), "t")
	if err != nil {
		t.Fatal(err)
	}
	prev := Rows("fn-lines\t8\nfn-params\t6\n# fn-results\t1\nbroken row\nnew-key\t1\n")
	got := cur.Raised(prev)
	if len(got) != 1 || got[0] != (Raise{Key: keyFnParams, Was: 6, Now: 7}) {
		t.Fatalf("Raised = %+v, want only fn-params 6 -> 7", got)
	}
}

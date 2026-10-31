package gogen_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// rangeRun is one line of a cases file: the smoke test decodes Text and compares what it prints with Want.
type rangeRun struct {
	Name string `json:"name"`
	Text string `json:"text"`
	Want string `json:"want"`
}

// casesFile is the JSON the smoke tests read their runs from.
func casesFile(t *testing.T, runs []rangeRun) map[string][]byte {
	t.Helper()
	raw, err := json.Marshal(runs)
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{"cases.json": raw}
}

// typesRuns are rangeCases applied to rangeBase, then the @text cases (named with a leading @).
func typesRuns(t *testing.T) []rangeRun {
	t.Helper()
	var runs []rangeRun
	for _, c := range rangeCases() {
		text := rangeBase
		if c.from != "" {
			text = strings.Replace(rangeBase, c.from, c.to, 1)
			if text == rangeBase && c.from != c.to {
				t.Fatalf("%s: %q not in the base", c.name, c.from)
			}
		}
		runs = append(runs, rangeRun{c.name, text, c.want})
	}
	for _, c := range textCases {
		runs = append(runs, rangeRun{"@" + c.name, c.text, c.want})
	}
	return runs
}

// CODEGEN.md §5.15, DECISIONS 399: a types-mode Decode<T> and Decode<Fn>File refuse a value outside its range or length refinement at every position, after the type rules, with one failure, in the bytes every target writes.
func TestGoRangeChecksRun(t *testing.T) {
	files := generateData(t, rangePackage())
	runData(t, files, rangeDir, "testdata/smoke/range_types_test.go", casesFile(t, typesRuns(t)))
}

// dataRuns are rangeDataCases with the data file's name for their path.
func dataRuns() []rangeRun {
	var runs []rangeRun
	for _, c := range rangeDataCases {
		runs = append(runs, rangeRun{c.name, c.text, strings.ReplaceAll(c.want, "NAME", c.name)})
	}
	return runs
}

// CODEGEN.md §5.15, §5.9: a data loader refuses a row field outside its refinement at its path, a stored `$` result and a lookup cell too, and the rows' count after their duplicates.
func TestGoRangeDataRun(t *testing.T) {
	data := casesFile(t, dataRuns())
	for _, r := range dataRuns() {
		data[r.Name+".json"] = []byte(r.Text)
	}
	runData(t, generateData(t, rangeItems()), itemsDir, "testdata/smoke/range_data_test.go", data)
}

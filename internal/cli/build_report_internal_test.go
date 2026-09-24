package cli

import (
	"encoding/json"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// DECISIONS 201: the JSON summary is check's (errors, warnings, packages, ms, truncated kept),
// followed by written and stale.
func TestBuildSummaryKeepsTruncated(t *testing.T) {
	check := &canon.CheckResult{Summary: canon.Summary{
		Errors: 1, Warnings: 2, Packages: 3,
		Truncated: []canon.Truncation{{Package: "a", Errors: 1, Warnings: 1}},
	}}
	data, err := json.Marshal(newBuildSummary(check, 4, 5))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"summary":{"errors":1,"warnings":2,"packages":3,"ms":0,"truncated":[{"package":"a","errors":1,"warnings":1}],"written":4,"stale":5}}`
	if string(data) != want {
		t.Errorf("got  %s\nwant %s", data, want)
	}
}

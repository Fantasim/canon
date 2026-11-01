package build_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"golang.org/x/tools/txtar"
)

// API.md §4.1 (Check): a hard error a named check's run raises in stage C, E4401 included, names the check.
func TestCheckRunErrorsNameTheirCheck(t *testing.T) {
	for _, c := range []struct {
		file, check string
		code        diag.Code
	}{
		{"testdata/findings/E4002_check_named.txtar", checkSixth, diag.E4002.Def().Code},
		{"testdata/findings/E4401_check_named.txtar", "heavy", diag.E4401.Def().Code},
	} {
		t.Run(c.file, func(t *testing.T) {
			a, err := txtar.ParseFile(c.file)
			if err != nil {
				t.Fatal(err)
			}
			list := buildCase(t, a).Findings.List
			if len(list) != 1 || list[0].Code != c.code || list[0].Check != c.check {
				t.Errorf("want one %s naming check %q, got %+v", c.code, c.check, list)
			}
		})
	}
}

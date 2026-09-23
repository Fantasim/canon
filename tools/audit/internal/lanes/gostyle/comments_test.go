package gostyle

import (
	"strings"
	"testing"
)

func TestCommentRules(t *testing.T) {
	fs := runFixture(t, "testdata/comments")
	counts := countByRule(fs)

	want := map[string]int{
		ruleCommentPkg:    1,
		ruleCommentHeader: 1,
		ruleCommentDecl:   1,
		ruleCommentField:  2,
		ruleCommentBlock:  2,
		ruleCommentRatio:  1,
		ruleADRNarration:  1, // content.go only: the ADR citation in LongDoc is comment-decl's
		ruleHistory:       1,
		ruleTODO:          1,
	}
	for rule, n := range want {
		if counts[rule] != n {
			t.Errorf("%s: got %d findings, want %d (%+v)", rule, counts[rule], n, only(fs, rule))
		}
	}

	assertValue(t, fs, ruleCommentPkg, wantPkgDocLines)
	assertValue(t, fs, ruleCommentHeader, wantHeaderLines)
	assertValue(t, fs, ruleCommentDecl, wantDeclLines)
	assertValue(t, fs, ruleADRNarration, wantADRLines)

	block := only(fs, ruleCommentBlock)
	if len(block) == 2 {
		if block[0].Symbol != "Blocky" || block[1].Symbol != "Blocky" {
			t.Errorf("comment-block: Symbols = %q, %q, want both %q", block[0].Symbol, block[1].Symbol, "Blocky")
		}
		if block[0].Detail != block[1].Detail {
			t.Errorf("comment-block: two blocks in one function should share a ratchet key, got details %q and %q",
				block[0].Detail, block[1].Detail)
		}
	}

	field := only(fs, ruleCommentField)
	details := map[string]bool{}
	for _, f := range field {
		details[f.Detail] = true
	}
	if !details[wantGroupedDetail] || !details[wantFieldDetail] {
		t.Errorf("comment-field: Details = %v, want %q and %q", details, wantGroupedDetail, wantFieldDetail)
	}

	todo := only(fs, ruleTODO)
	if len(todo) == 1 && !strings.Contains(todo[0].Detail, "TODO") {
		t.Errorf("todo-in-code: Detail = %q, want it to contain TODO", todo[0].Detail)
	}
}

const (
	wantPkgDocLines   = 12
	wantHeaderLines   = 5
	wantDeclLines     = 4
	wantADRLines      = 2
	wantGroupedDetail = "Grouped"
	wantFieldDetail   = "Value"
)

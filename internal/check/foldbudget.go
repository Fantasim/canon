package check

import "github.com/fantasim/canonlang/internal/diag"

// noteSpent notes p's step counter spent when the failed fold just made added an E4401 to any
// bag: a fold that succeeds reports none, so every E4401 since the last failed fold is this one's.
func (c *checker) noteSpent(p *pkgState) {
	n := 0
	for _, q := range c.order {
		n += countCode(q.bag.Findings(), diag.E4401.Def().Code)
	}
	if n > c.exhausted {
		if c.spent == nil {
			c.spent = map[string]bool{}
		}
		c.spent[p.path] = true
	}
	c.exhausted = n
}

// spentNow reports p's step counter spent by an earlier fold, the default then left to its storage
// points; a Recheck's probe asks the fold anyway, so a default checked again sends it to cold.
func (c *checker) spentNow(p *pkgState) bool {
	return c.spent[p.path] && (c.journal == nil || !c.journal.probing)
}

func countCode(list []diag.Finding, code diag.Code) int {
	n := 0
	for _, f := range list {
		if f.Code == code {
			n++
		}
	}
	return n
}

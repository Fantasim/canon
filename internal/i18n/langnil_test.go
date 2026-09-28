package i18n_test

import (
	"fmt"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// I18N.md F2: a translation file whose language did not parse gets no E1704 on top of the
// syntax error already reported for it.
func TestTranslationNoLangNoDoubleReport(t *testing.T) {
	c := checkFiles(t, map[string]string{"nl/nl.canon": `package nl
translation

foo "bar"
`})
	want := fmt.Sprintf("error[%s]  nl/nl.canon:2:12\n  expected IDENT; found NL\n\n1 error, 0 warnings in 1 package (…)\n", diag.E1116.Def().Code)
	if out := c.render(t); out != want {
		t.Errorf("a missing language should add nothing beyond the syntax error:\n%s", out)
	}
}

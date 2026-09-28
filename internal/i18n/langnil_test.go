package i18n_test

import "testing"

// I18N.md F2: a translation file whose language did not parse gets no E1704 on top of the
// syntax error already reported for it.
func TestTranslationNoLangNoDoubleReport(t *testing.T) {
	c := checkFiles(t, map[string]string{"nl/nl.canon": `package nl
translation

foo "bar"
`})
	if out := c.render(t); out != "0 errors, 0 warnings in 1 package (…)\n" {
		t.Errorf("a missing language should add nothing to the syntax error:\n%s", out)
	}
}

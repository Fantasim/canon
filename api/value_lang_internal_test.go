package canon

import (
	"context"
	"testing"
)

// API.md R4, R6a, DECISIONS 345: a Value keeps its call's language, "" for Options.Lang, and every
// Child read from it reads in the same snapshot, so in the same language.
func TestValueWithKeepsLang(t *testing.T) {
	p, err := Open("/law", Options{FS: newWriteFS(sharedLaw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx := context.Background()
	for _, lang := range []string{"", "fr"} {
		v, err := p.ValueWith(ctx, ValueRequest{Path: "a:uses", Lang: lang})
		if err != nil {
			t.Fatal(err)
		}
		c, err := v.Child(".first")
		if err != nil {
			t.Fatal(err)
		}
		if v.snap.lang != lang || c.snap != v.snap {
			t.Errorf("API.md R6a: ValueWith in %q keeps %q, its Child's snapshot shared %t", lang, v.snap.lang, c.snap == v.snap)
		}
		for _, k := range v.Children() {
			if k.snap != v.snap {
				t.Errorf("API.md R4, R6a: a child of %q left the value's snapshot", lang)
			}
		}
	}
}

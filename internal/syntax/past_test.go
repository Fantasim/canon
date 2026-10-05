package syntax_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
)

// GRAMMAR.md §4.2, §5.9 pastType: PastOf is the `past` before a primary type, NoTok for a name `past`.
func TestPastOf(t *testing.T) {
	for _, tc := range []struct {
		text string
		kind syntax.NodeKind
		past bool
	}{
		{"past K", syntax.KindNamedType, true},
		{"past ref items", syntax.KindRefType, true},
		{"past [K]", syntax.KindListType, true},
		{"past stable table T", syntax.KindTableType, true},
		{"past fn(K) -> V", syntax.KindFnType, true},
		{`past asset("@resource/Icon")`, syntax.KindAssetType, true},
		{"past match n { _ => K }", syntax.KindMatchType, true},
		{`past "lit"`, syntax.KindLiteralType, true},
		{"past _", syntax.KindAnyType, true},
		{"past past", syntax.KindNamedType, true},
		{"past", syntax.KindNamedType, false},
		{"past.K", syntax.KindNamedType, false},
		{"past(1..)", syntax.KindNamedType, false},
		{"(past K)", syntax.KindParenType, false},
		{"K", syntax.KindNamedType, false},
	} {
		tree, bag, _ := parseText(t, "a/a.canon", []byte("package a\n\ntype T = "+tc.text+"\n"))
		if n := len(bag.Findings()); n > 0 {
			t.Errorf("%s: %d findings", tc.text, n)
			continue
		}
		typ := tree.Decls[0].(*syntax.TypeDecl).Type
		p := syntax.PastOf(typ)
		switch {
		case typ.Kind() != tc.kind:
			t.Errorf("%s: a %s, want a %s", tc.text, typ.Kind(), tc.kind)
		case p.Valid() != tc.past:
			t.Errorf("%s: PastOf valid = %v, want %v", tc.text, p.Valid(), tc.past)
		case tc.past && (p != typ.First() || tree.Tokens[p].Kind != syntax.TokIdent):
			t.Errorf("%s: PastOf is token %d, want the identifier at %d", tc.text, p, typ.First())
		}
	}
}

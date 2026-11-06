package progen_test

import (
	"path"
	"regexp"
	"strconv"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// wordKey is a string key that starts an amend line as a bare word (DECISIONS 450).
var wordKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// rekeyElement adds a layer line setting a keyed-list element's key field, by its word key, to another element's key; the finding is at it (EVALUATION.md §9.3).
func rekeyElement(tg target) []progen.Site {
	if !isSource(tg) || tg.file.Package == nil {
		return nil
	}
	var out []progen.Site
	layer := path.Join(path.Dir(tg.path), "zz.layer.canon")
	head := "package " + text(tg, tg.file.Package) + "\nlayer zz\n\namend "
	for _, d := range nodes[*syntax.LetDecl](tg) {
		kt, keyed := d.Type.(*syntax.KeyedType)
		list, isList := d.Value.(*syntax.ListLit)
		if !keyed || !isList || kt.Key == nil {
			continue
		}
		keys := wordKeys(list, kt.Key.Name)
		if len(keys) < 2 {
			continue
		}
		line := d.Name.Name + " {\n  " + keys[0] + "." + kt.Key.Name + ": "
		out = append(out, progen.Site{
			Path:  layer,
			Edits: []progen.Edit{insert(0, head+line), insert(0, strconv.Quote(keys[1])), insert(0, "\n}\n")},
			Focus: 1,
		})
	}
	return out
}

// wordKeys are the keys a list literal's elements write as plain strings in field key that are
// words, in order.
func wordKeys(list *syntax.ListLit, key string) []string {
	var out []string
	for _, el := range list.Elems {
		lit, ok := el.(*syntax.BraceLit)
		if !ok {
			continue
		}
		for _, it := range lit.Items {
			f, isField := it.(*syntax.FieldItem)
			if !isField || f.Name.Name != key {
				continue
			}
			if s, isStr := f.Value.(*syntax.StringLit); isStr && !s.Multiline && len(s.Parts) == 1 && s.Parts[0].Interp == nil && wordKey.MatchString(s.Parts[0].Text) {
				out = append(out, s.Parts[0].Text)
			}
		}
	}
	return out
}

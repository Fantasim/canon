package render

import (
	"slices"

	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Messages are the messages of check c, a named one-line check that failed on self (nil for a
// package check of pkg), in each of langs that translates its key, rendered as a template in
// that language (VIEWMODEL.md J15, I18N.md T4); a rendering that fails is left out (X7).
func (r *Renderer) Messages(pkg string, c *syntax.CheckDecl, self value.Value, langs []string) map[string]string {
	kpkg, segs, ok := checkKey(pkg, c, self)
	if !ok {
		return nil
	}
	if _, catalogued := r.in.Texts.Source(kpkg, segs...); !catalogued {
		return nil
	}
	var out map[string]string
	for _, lang := range langs {
		tpl, found := r.translated(kpkg, lang, segs)
		if !found {
			continue
		}
		if text, ok := r.template(tpl, self, Magic{}, lang); ok {
			if out == nil {
				out = map[string]string{}
			}
			out[lang] = text
		}
	}
	return out
}

// checkKey is the key of c's message (I18N.md 3.3): `T.check.n` for a check of record T or at
// variant T's level, `V.c.check.n` for a check of case c, `check.n` for a package check of pkg.
func checkKey(pkg string, c *syntax.CheckDecl, self value.Value) (string, []string, bool) {
	if c == nil || c.Name == nil || c.Body != nil || c.Message == nil {
		return "", nil, false
	}
	name := []string{syntax.WordCheck, c.Name.Name}
	if self == nil {
		return pkg, name, true
	}
	rec, ok := self.(*value.Record)
	if !ok {
		return "", nil, false
	}
	owner := checkOwner(rec.T, c)
	if owner == nil {
		return "", nil, false
	}
	kpkg, prefix := i18n.TypeKey(owner)
	return kpkg, append(prefix, name...), true
}

// checkOwner is the record or case declaring c for an instance of t, or its variant for a
// variant-level check (TYPES.md 12.1); nil when c is none of these.
func checkOwner(t types.Type, c *syntax.CheckDecl) types.Type {
	switch x := t.Base().(type) {
	case *types.RecordType:
		if slices.Contains(x.Checks, c) {
			return x
		}
	case *types.AppliedRecord:
		if slices.Contains(x.Rec.Checks, c) {
			return x.Rec
		}
	case *types.CaseType:
		if slices.Contains(x.Checks, c) {
			return x
		}
		return x.Variant
	}
	return nil
}

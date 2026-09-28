package progen_test

import (
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// frTranslationSuffix names an fr translation file (I18N.md F1's convention).
const frTranslationSuffix = ".fr.canon"

// Operators on check-decl "at" clauses (VIEWMODEL.md G19) and translated templates (I18N.md T1-T3).
func viewsOperators() []operator {
	return []operator{
		op(diag.E1633.Def().Code, "VIEWMODEL.md G19 (at names no field)", checkAtUnknown),
		op(diag.E1703.Def().Code, "I18N.md T1-T2 (translated template does not type-check)", translationBadName),
	}
}

// checkAtUnknown appends a record whose check names, in its `at` clause, a field it does not
// declare (VIEWMODEL.md G19).
func checkAtUnknown(tg target) []progen.Site {
	return appendSite("/// At.\nlocal record ZzAt {\n  /// N.\n  n: Int\n\n  check n < 100 at ", "zznope", " else \"too much\"\n}")(tg)
}

// translationPrelude declares a public record with a named check, so its check.zzrule key
// joins the catalogue (I18N.md K1): translationKey is that key.
const translationPrelude = "/// Tr.\nrecord ZzTr {\n  /// N.\n  n: Int\n\n" +
	"  check zzrule: n < 100 else \"n is {n}\"\n}\n\nlocal let zzTr: ZzTr = { n: 1 }"

// translationKey is translationPrelude's check, keyed "T.check.n" (I18N.md §3.3).
const translationKey = "ZzTr.check.zzrule"

// translationBadName appends translationPrelude and a bad translation entry, only where an fr
// file already exists so the package's own W1701 baseline, if any, keeps its position (I18N W2).
func translationBadName(tg target) []progen.Site {
	if !isSource(tg) || dataMode(tg) || tg.pkg == studioPackage(tg.all) || !hasFrFile(tg) {
		return nil
	}
	end := declEnd(tg)
	transPath := path.Join(path.Dir(tg.path), "zzi18n"+frTranslationSuffix)
	head := "package " + tg.pkg + "\ntranslation fr\n\n" + translationKey + " \""
	body := head + "{zznope}\"\n"
	return []progen.Site{{
		Path:  tg.path,
		Edits: []progen.Edit{insert(end, "\n\n"+translationPrelude+"\n")},
		Add:   map[string][]byte{transPath: []byte(body)},
		Elsewhere: &progen.Place{
			Path:   transPath,
			Region: progen.Region{Start: len(head), End: len(head) + len("{zznope}")},
		},
	}}
}

// hasFrFile reports tg's directory already holding an fr translation file.
func hasFrFile(tg target) bool {
	dir := path.Dir(tg.path)
	return slices.ContainsFunc(*tg.all, func(o target) bool {
		return path.Dir(o.path) == dir && strings.HasSuffix(o.path, frTranslationSuffix)
	})
}

// studioPackage is the package project.studio names, "" without a project.canon among all.
func studioPackage(all *[]target) string {
	for _, tg := range *all {
		if !isProject(tg) || tg.file == nil || tg.file.Project == nil {
			continue
		}
		for _, e := range tg.file.Project.Items {
			if text(tg, e.Key) == "studio" {
				return text(tg, e.Value)
			}
		}
	}
	return ""
}

package progen_test

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// I18N.md §4-§5.
func i18nOperators() []operator {
	return []operator{
		op(diag.E1702.Def().Code, "I18N.md F4 (translation key matches no text)", translationKeyUnknown),
		op(diag.E1704.Def().Code, "I18N.md F2 (translation language not in project.languages)", translationLanguageUnknown),
		op(diag.E1705.Def().Code, "I18N.md F3 (translation key given twice)", translationKeyTwice),
		op(diag.E1706.Def().Code, "I18N.md F2 (translation of the source language)", translationOfSourceLanguage),
		op(diag.E1707.Def().Code, "I18N.md F6 (plain text translation interpolates)", translationInterpolatesPlain),
		op(diag.W1701.Def().Code, "I18N.md W1 (missing translation, package has emit view)", missingTranslation),
	}
}

// zzWidgetSource is a minimal package: one record, one field, both documented (else W1002),
// so each operator below controls exactly what is missing or wrong in its translation.
func zzWidgetSource(pkg string) string {
	return "package " + pkg + "\n\n/// A widget.\nrecord ZzWidget {\n  /// Its name.\n  name: String\n}\n"
}

// i18nSite is a site adding pkg's source and one translation file, anchored on project.canon:
// the corpus's one target for it, so each operator below has exactly one site.
func i18nSite(pkg, transPath, transBody string, at progen.Region) progen.Site {
	return progen.Site{
		Path: pkg + "/" + pkg + ".canon", Edits: []progen.Edit{insert(0, zzWidgetSource(pkg))}, Focus: 0,
		Add: map[string][]byte{transPath: []byte(transBody)}, Packages: []string{pkg},
		Elsewhere: &progen.Place{Path: transPath, Region: at},
	}
}

func translationKeyUnknown(tg target) []progen.Site {
	if !isProject(tg) {
		return nil
	}
	const pkg = "zze1702"
	head := "package " + pkg + "\ntranslation fr\n\n"
	const key = "ZzWidget.zzbogus"
	body := head + key + " \"Bogus\"\n"
	return []progen.Site{i18nSite(pkg, pkg+"/"+pkg+".fr.canon", body, progen.Region{Start: len(head), End: len(head) + len(key)})}
}

func translationLanguageUnknown(tg target) []progen.Site {
	if !isProject(tg) {
		return nil
	}
	const pkg = "zze1704"
	head := "package " + pkg + "\ntranslation "
	const lang = "de"
	body := head + lang + "\n\nfoo \"bar\"\n"
	return []progen.Site{i18nSite(pkg, pkg+"/"+pkg+".de.canon", body, progen.Region{Start: len(head), End: len(head) + len(lang)})}
}

func translationKeyTwice(tg target) []progen.Site {
	if !isProject(tg) {
		return nil
	}
	const pkg = "zze1705"
	head := "package " + pkg + "\ntranslation fr\n\nZzWidget.name \"Une\"\n"
	const key = "ZzWidget.name"
	body := head + key + " \"Deux\"\n"
	return []progen.Site{i18nSite(pkg, pkg+"/"+pkg+".fr.canon", body, progen.Region{Start: len(head), End: len(head) + len(key)})}
}

func translationOfSourceLanguage(tg target) []progen.Site {
	if !isProject(tg) {
		return nil
	}
	const pkg = "zze1706"
	head := "package " + pkg + "\ntranslation "
	const lang = "en"
	body := head + lang + "\n\nfoo \"bar\"\n"
	return []progen.Site{i18nSite(pkg, pkg+"/"+pkg+".en.canon", body, progen.Region{Start: len(head), End: len(head) + len(lang)})}
}

func translationInterpolatesPlain(tg target) []progen.Site {
	if !isProject(tg) {
		return nil
	}
	const pkg = "zze1707"
	head := "package " + pkg + "\ntranslation fr\n\nZzWidget.name \"Nom "
	const interp = "{name}"
	body := head + interp + "\"\n"
	return []progen.Site{i18nSite(pkg, pkg+"/"+pkg+".fr.canon", body, progen.Region{Start: len(head), End: len(head) + len(interp)})}
}

// missingTranslation adds a package with `emit view` and no translation file at all: I18N.md
// W1 reports one W1701 for it, at its package clause (W2), since it never appears in the
// corpus's own baseline (the package did not exist before this mutation).
func missingTranslation(tg target) []progen.Site {
	if !isProject(tg) {
		return nil
	}
	const pkg = "zzw1701"
	src := zzWidgetSource(pkg) + "\nemit view { out: \"@generated/" + pkg + ".view.json\" }\n"
	const clause = "package " + pkg
	return []progen.Site{{
		Path: pkg + "/" + pkg + ".canon", Edits: []progen.Edit{insert(0, src)}, Focus: 0,
		Packages:  []string{pkg},
		Elsewhere: &progen.Place{Path: pkg + "/" + pkg + ".canon", Region: progen.Region{Start: 0, End: len(clause)}},
	}}
}

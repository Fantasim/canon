package progen_test

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// VIEWMODEL.md §3.2-§3.3, §4.5.
func viewBasicsOperators() []operator {
	return []operator{
		op(diag.E1601.Def().Code, "VIEWMODEL.md C40, §4.5 (control does not accept the type)", controlTypeMismatch),
		op(diag.E1602.Def().Code, "VIEWMODEL.md G8 (name resolves to nothing)", unresolvedViewName),
		op(diag.E1603.Def().Code, "VIEWMODEL.md G4 (view outside its target's package)", viewOutsidePackage),
		op(diag.E1605.Def().Code, "VIEWMODEL.md G9 (name placed twice, two groups)", placedTwice),
		op(diag.E1606.Def().Code, "VIEWMODEL.md §3.3, VIEW-09 (columns names a method)", columnsNameMethod),
		op(diag.E1607.Def().Code, "VIEWMODEL.md G5 (second view of one target)", secondView),
		op(diag.E1608.Def().Code, "VIEWMODEL.md G20 (widget value type mismatch)", widgetTypeMismatch),
		op(diag.E1609.Def().Code, "VIEWMODEL.md C40 (control name not built-in)", unknownControlName),
	}
}

func controlTypeMismatch(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1601 {\n  name: String\n}\n\n"+
		"view ZzE1601 {\n  name { control: switch }\n}")}
}

func unresolvedViewName(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1602 {\n  label: String\n}\n\n"+
		"view ZzE1602 {\n  lable \"Label\"\n}")}
}

// viewOutsidePackage adds a record in a brand-new package and, in tg, an import of it and a
// view: VIEWMODEL.md G4 says a view must live in its target's own package.
func viewOutsidePackage(tg target) []progen.Site {
	if !isSource(tg) || tg.file.Package == nil {
		return nil
	}
	_, pe := span(tg, tg.file.Package)
	if n := len(tg.file.Imports); n > 0 {
		_, pe = span(tg, tg.file.Imports[n-1])
	}
	end := declEnd(tg)
	const other = "zze1603/zze1603.canon"
	otherSrc := "package zze1603\n\nrecord ZzE1603Item {\n  label: String\n}\n"
	return []progen.Site{{
		Path: tg.path,
		Edits: []progen.Edit{
			insert(pe, "\n\nimport zze1603 { ZzE1603Item }"),
			insert(end, "\n\nview ZzE1603Item {\n  title \"Zz\"\n}\n"),
		},
		Focus: 1,
		Add:   map[string][]byte{other: []byte(otherSrc)},
	}}
}

func placedTwice(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1605 {\n  label: String\n}\n\n"+
		"view ZzE1605 {\n  group zzg1 \"G1\" { label }\n  group zzg2 \"G2\" { label }\n}")}
}

func columnsNameMethod(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1606 {\n  label: String\n\n"+
		"  fn zzDoubled(self) -> Int {\n    return 1\n  }\n}\n\n"+
		"view ZzE1606 {\n  columns { zzDoubled }\n}")}
}

func secondView(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1607 {\n  label: String\n}\n\n"+
		"view ZzE1607 {\n  title \"{label}\"\n}\n\n"+
		"view ZzE1607 {\n  label \"Label\"\n}")}
}

func widgetTypeMismatch(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1608 {\n  label: String\n}\n\n"+
		"view ZzE1608 {\n  label { widget: weight_share }\n}")}
}

func unknownControlName(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1609 {\n  label: String\n}\n\n"+
		"view ZzE1609 {\n  label { control: zzdial }\n}")}
}

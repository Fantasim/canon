package progen_test

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// VIEWMODEL.md §3.2, §3.9, G8, G10, G15, G23, L6, L15.
func viewTargetsOperators() []operator {
	return []operator{
		op(diag.E1626.Def().Code, "VIEWMODEL.md G6 (view on a let that is not a define table)", viewOnPlainLet),
		op(diag.E1627.Def().Code, "VIEWMODEL.md G6 (item not allowed for an enum view)", enumViewWithTitle),
		op(diag.E1628.Def().Code, "VIEWMODEL.md G8 (case field names differ across cases)", ambiguousCaseField),
		op(diag.E1632.Def().Code, "VIEWMODEL.md G23 (@menu on a local let)", menuOnLocalLet),
		op(diag.E1634.Def().Code, "VIEWMODEL.md G15 (control and widget together)", controlAndWidget),
		op(diag.W1604.Def().Code, "VIEWMODEL.md G11 (field named key hides the magic name)", fieldHidesMagicName),
		op(diag.W1641.Def().Code, "VIEWMODEL.md L15 (hidden required field without a default)", hiddenRequiredField),
		op(diag.W1642.Def().Code, "VIEWMODEL.md L6 (deprecated field named in a group)", deprecatedFieldInGroup),
	}
}

func viewOnPlainLet(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local let zzE1626: Int = 1\n\n"+
		"view zzE1626 {\n  title \"Zz\"\n}")}
}

func enumViewWithTitle(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local enum ZzE1627 { zzA, zzB }\n\n"+
		"view ZzE1627 {\n  title \"Zz\"\n}")}
}

func ambiguousCaseField(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local variant ZzE1628Kind {\n  zzA { zzField: Int }\n  zzB { zzField: String }\n}\n\n"+
		"local record ZzE1628 {\n  kind: ZzE1628Kind @json(inline)\n}\n\n"+
		"view ZzE1628 {\n  zzField \"Zz\"\n}")}
}

func menuOnLocalLet(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1632 {\n  label: String\n}\n\n"+
		"@menu(items)\nlocal let zzE1632: [ZzE1632] = []")}
}

func controlAndWidget(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1634 {\n  count: Int\n}\n\n"+
		"view ZzE1634 {\n  count { control: text, widget: weight_share }\n}")}
}

func fieldHidesMagicName(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzW1604 {\n  key: String\n}\n\n"+
		"view ZzW1604 {\n  title \"{key}\"\n}")}
}

func hiddenRequiredField(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzW1641 {\n  label: String\n}\n\n"+
		"view ZzW1641 {\n  label { hidden: true }\n}")}
}

func deprecatedFieldInGroup(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzW1642 {\n  label: String\n  zzOld: Int = 0 @deprecated\n}\n\n"+
		"view ZzW1642 {\n  group zzg \"Zz\" { zzOld }\n}")}
}

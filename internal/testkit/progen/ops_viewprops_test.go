package progen_test

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// VIEWMODEL.md §3.4-§3.5.
func viewPropsOperators() []operator {
	return []operator{
		op(diag.E1610.Def().Code, "VIEWMODEL.md G16 (menu not declared by the studio)", unknownMenu),
		op(diag.E1611.Def().Code, "VIEWMODEL.md §3.5 (unit on a field that is not a number)", unitOnNonNumber),
		op(diag.E1612.Def().Code, "VIEWMODEL.md G13 (preview not an asset)", previewNotAsset),
		op(diag.E1613.Def().Code, "VIEWMODEL.md §3.5 (unknown property)", unknownProperty),
		op(diag.E1614.Def().Code, "VIEWMODEL.md G10 (label declared twice)", labelledTwice),
		op(diag.E1615.Def().Code, "VIEWMODEL.md G14 (unescaped { in plain text)", unescapedBrace),
		op(diag.E1616.Def().Code, "VIEWMODEL.md §3.3 (view names a method with parameters)", methodWithParams),
	}
}

func unknownMenu(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1610 {\n  label: String\n}\n\n"+
		"view ZzE1610 {\n  menu zzbogus icon gem\n}")}
}

func unitOnNonNumber(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1611 {\n  label: String\n}\n\n"+
		"view ZzE1611 {\n  label { unit: hp }\n}")}
}

func previewNotAsset(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1612 {\n  label: String\n}\n\n"+
		"view ZzE1612 {\n  preview label\n}")}
}

func unknownProperty(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1613 {\n  label: String\n}\n\n"+
		"view ZzE1613 {\n  label { zznotaprop: 1 }\n}")}
}

func labelledTwice(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1614 {\n  label: String\n}\n\n"+
		"view ZzE1614 {\n  label \"First\"\n  group zzg \"G\" { label \"Second\" }\n}")}
}

func unescapedBrace(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1615 {\n  label: String\n}\n\n"+
		"view ZzE1615 {\n  label \"Zz {oops}\"\n}")}
}

func methodWithParams(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1616 {\n  count: Int\n\n"+
		"  fn zzScaled(self, k: Int) -> Int {\n    return count * k\n  }\n}\n\n"+
		"view ZzE1616 {\n  zzScaled \"Zz\"\n}")}
}

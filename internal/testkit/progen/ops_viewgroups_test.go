package progen_test

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// VIEWMODEL.md §3.6, §4.5, §7.
func viewGroupsOperators() []operator {
	return []operator{
		op(diag.E1617.Def().Code, "VIEWMODEL.md G17 (duplicate group id)", duplicateGroupID),
		op(diag.E1618.Def().Code, "VIEWMODEL.md G17 (id starting with _)", reservedGroupID),
		op(diag.E1619.Def().Code, "VIEWMODEL.md §4.5 (slider without both bounds)", sliderMissingBound),
		op(diag.E1620.Def().Code, "VIEWMODEL.md T14 (filter on an unfilterable type)", unfilterableType),
		op(diag.E1621.Def().Code, "VIEWMODEL.md T14 (multi on a non-choice filter)", multiOnNonChoice),
		op(diag.E1622.Def().Code, "VIEWMODEL.md G13 (search term of the wrong type)", unsearchableType),
		op(diag.E1623.Def().Code, "VIEWMODEL.md §3.5 (step text is not a bare {index})", stepNotBareIndex),
	}
}

func duplicateGroupID(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1617 {\n  label: String\n  count: Int\n}\n\n"+
		"view ZzE1617 {\n  group zzg \"G1\" { label }\n  group zzg \"G2\" { count }\n}")}
}

func reservedGroupID(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1618 {\n  label: String\n}\n\n"+
		"view ZzE1618 {\n  group _zz \"Zz\" { label }\n}")}
}

func sliderMissingBound(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1619 {\n  count: Int\n}\n\n"+
		"view ZzE1619 {\n  count { control: slider }\n}")}
}

func unfilterableType(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1620 {\n  label: String\n}\n\n"+
		"view ZzE1620 {\n  filters { label }\n}")}
}

func multiOnNonChoice(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1621 {\n  flag: Bool\n}\n\n"+
		"view ZzE1621 {\n  filters { flag multi }\n}")}
}

func unsearchableType(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1622 {\n  flag: Bool\n}\n\n"+
		"view ZzE1622 {\n  search { flag }\n}")}
}

func stepNotBareIndex(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "local record ZzE1623 {\n  steps: [Int]\n}\n\n"+
		"view ZzE1623 {\n  steps { step: \"Zz {index + 1}\" }\n}")}
}

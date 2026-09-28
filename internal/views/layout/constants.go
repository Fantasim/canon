package layout

// The kinds of a resolved view (VIEWMODEL.md 12.4).
const (
	viewRecord  = "record"
	viewCase    = "case"
	viewVariant = "variant"
)

// The kinds of a section (VIEWMODEL.md 5.1, 12.4).
const (
	sectionGroup  = "group"
	sectionOther  = "other"
	sectionMore   = "more"
	sectionUnused = "unused"
)

// unnamedShow starts the ids of unnamed `show` lines: `_0`, `_1`, … (VIEWMODEL.md G17).
const unnamedShow = "_"

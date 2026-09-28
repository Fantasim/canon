package table

// The entry column and the column modes of VIEWMODEL.md T6, T8.
const (
	entryColumn = "$entry"
	modeEntry   = "entry"
	modeEdit    = "edit"
	modeValue   = "value"
	modeCount   = "count"
	modeCase    = "case"
	modeText    = "text"
)

// autoColumns is T7's count; dot joins T6a's `<case>.<field>`.
const (
	autoColumns = 6
	dot         = "."
)

// Column widths in pixels (VIEWMODEL.md 3.6).
const (
	minWidth = 16
	maxWidth = 2000
)

// The filter kinds of VIEWMODEL.md T11.
const (
	filterChoice   = "choice"
	filterCase     = "case"
	filterBool     = "bool"
	filterRange    = "range"
	filterContains = "contains"
)

// checkboxesMax is the most choices a `multi` filter shows as checkboxes (T11).
const checkboxesMax = 6

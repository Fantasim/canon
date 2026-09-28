package rules

// Column widths in pixels (VIEWMODEL.md §3.6).
const (
	minWidth = 16
	maxWidth = 2000
)

// The magic names a field may hide (VIEWMODEL.md G11), and the one a step text may use (§3.5).
const (
	magicKey   = "key"
	magicIndex = "index"
)

// reserved starts the ids of groups and show lines that belong to the compiler (VIEWMODEL.md G17).
const reserved = "_"

// dot joins a variant and its case, a package and a name.
const dot = "."

// targetKind is what a view describes (VIEWMODEL.md §3.2).
type targetKind uint8

// The targets of VIEWMODEL.md §3.2.
const (
	targetRecord targetKind = iota
	targetVariant
	targetCase
	targetEnum
	targetDefine
)

// idKind is the namespace of a view id: groups or show lines (VIEWMODEL.md G17).
type idKind uint8

const (
	idGroup idKind = iota
	idShow
	idKinds
)

// filterKind is the kind of filter a field's type gives (VIEWMODEL.md T11); filterNone is none.
type filterKind uint8

const (
	filterNone filterKind = iota
	filterChoice
	filterCase
	filterBool
	filterRange
	filterContains
)

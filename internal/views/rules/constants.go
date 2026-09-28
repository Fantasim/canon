package rules

// The built-in control hints (VIEWMODEL.md §4.5).
const (
	ctlSwitch     = "switch"
	ctlCheckbox   = "checkbox"
	ctlSegmented  = "segmented"
	ctlRadio      = "radio"
	ctlSelect     = "select"
	ctlSearch     = "search"
	ctlCheckboxes = "checkboxes"
	ctlChips      = "chips"
	ctlInput      = "input"
	ctlTextarea   = "textarea"
	ctlCode       = "code"
	ctlNumber     = "number"
	ctlStepper    = "stepper"
	ctlSlider     = "slider"
	ctlColor      = "color"
	ctlText       = "text"
)

// colorPattern is the regex source a String needs for `control: color` (VIEWMODEL.md §4.5).
const colorPattern = `^#[0-9a-fA-F]{6}$`

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

// ownPackage is this package in ERRORS.md's Package column.
const ownPackage = "views"

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

// The two id namespaces of a view.
const (
	idGroup idKind = iota
	idShow
	idKinds
)

// filterKind is the kind of filter a field's type gives (VIEWMODEL.md T11).
type filterKind uint8

// The filter kinds of T11; filterNone is a type that cannot be filtered.
const (
	filterNone filterKind = iota
	filterChoice
	filterCase
	filterBool
	filterRange
	filterContains
)

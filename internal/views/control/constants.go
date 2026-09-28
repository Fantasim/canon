package control

// The control kinds of VIEWMODEL.md §12.5.
const (
	CtlSwitch     = "switch"
	CtlCheckbox   = "checkbox"
	CtlSegmented  = "segmented"
	CtlRadio      = "radio"
	CtlSelect     = "select"
	ctlSearch     = "search"
	ctlCheckboxes = "checkboxes"
	ctlChips      = "chips"
	ctlInput      = "input"
	ctlTextarea   = "textarea"
	ctlCode       = "code"
	CtlNumber     = "number"
	CtlSlider     = "slider"
	ctlDuration   = "duration"
	ctlColor      = "color"
	ctlText       = "text"
	ctlFile       = "file"
	ctlSection    = "section"
	ctlCard       = "card"
	ctlTags       = "tags"
	ctlPositional = "positional"
	ctlRange      = "range"
	CtlTable      = "table"
	ctlList       = "list"
	ctlEnumRow    = "enumRow"
	ctlCards      = "cards"
	ctlMap        = "map"
	ctlVariant    = "variant"
	ctlDependent  = "dependent"
	ctlNever      = "never"
	ctlWidget     = "widget"

	hintStepper = "stepper" // the one hint that is not a kind: a number with a stepper
)

// The `unset` of an optional wrapper (VIEWMODEL.md C35).
const (
	unsetSegment = "segment"
	unsetClear   = "clear"
)

// Thresholds of VIEWMODEL.md C4, C5, C12, C18, C20 and C24.
const (
	segmentedMax  = 4
	selectMax     = 10
	stepperMax    = 20
	checkboxesMax = 6
	positionalMax = 6
	sectionMax    = 6
)

// colorPattern is the regex source a String needs for `control: color` (VIEWMODEL.md §4.5).
const colorPattern = `^#[0-9a-fA-F]{6}$`

// The words of the `where` forms of VIEWMODEL.md C32, C33 and C34.
const (
	wordIt       = "it"
	wordIsUnique = "isUnique"
	wordLen      = "len"
	rangeLen     = 2
	secondIndex  = 1
)

// The `readonly` reasons of a field view (VIEWMODEL.md C43).
const (
	ReadonlyView       = "view"
	ReadonlyDeprecated = "deprecated"
	ReadonlySingle     = "single"
	ReadonlyInput      = "input"
)

package value

// The provenance kinds of EVL-07, in API.md OriginKind order.
const (
	ProvLiteral ProvKind = iota
	ProvJSON
	ProvCSV
	ProvDefines
	ProvText
	ProvDefault
	ProvSpread
	ProvComputed
	ProvLayer
)

// The text form's punctuation (STD-06).
const (
	textNone      = "none"
	textRange     = ".."
	textSep       = ", "
	textColon     = ": "
	textOpen      = "("
	textClose     = ")"
	textOpenList  = "["
	textCloseList = "]"
	textOpenMap   = "{"
	textCloseMap  = "}"
)

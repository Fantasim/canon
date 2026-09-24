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

// pieceStride is the pieces a component of a list takes in its text: a separator and itself.
const pieceStride = 2

// memoFrom is the composite pairs an equality walk visits before it remembers the pairs it
// took, so that a small comparison allocates no memo (DECISIONS 197).
const memoFrom = 1024

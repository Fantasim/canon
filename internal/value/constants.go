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

// The kinds of a key's hash (DECISIONS 199).
const (
	keyOther keyKind = iota
	keyIdent
	keyIdentInt
	keyInt
	keyStr
	keyMember
	keyCase
	keySymbol
	keyDur
	keyFloat
	keyBool
)

// hashNodes is the most nodes of a value Hash walks: equal values share that prefix, and the
// charged equality decides (DECISIONS 199).
const hashNodes = 64

// indexStripes is how many locks the maps' key indexes share, a map taking one by its pointer.
const indexStripes = 64

// pairLen is the length a pair folds into a hash.
const pairLen = 2

// underscore alone is no word (SPEC §2.4).
const underscore = "_"

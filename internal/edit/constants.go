package edit

// The segment forms of API.md §6.1.
const (
	SegField SegKind = iota
	SegKey
	SegPos
)

// The key forms of API.md §6.1.
const (
	KeyWord KeyLitKind = iota
	KeyInt
	KeyString
)

// Path syntax (API.md §6.1).
const (
	fieldMark    = '.'
	bracketOpen  = '['
	bracketClose = "]"
	positionMark = "#"
	packageMark  = ":"
	jsonQuote    = `"`
	minus        = '-'
	zeroDigit    = '0'
	underscore   = "_"
	decimalBase  = 10
	int64Bits    = 64
)

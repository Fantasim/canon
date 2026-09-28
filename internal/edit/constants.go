package edit

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

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

// fmtSyntaxError is a SyntaxError's text: the sentinel, the reason, the offset.
const fmtSyntaxError = "%v: %v at byte %d"

// The pseudo-fields of API.md P3.
const (
	pseudoID      = "id"
	pseudoRetired = "retired"
	pseudoKind    = "kind"
)

// The editing modes of API.md W4; ModeNone is a value no edit can reach.
const (
	ModeNone Mode = iota
	ModeCanon
	ModeJSON
)

// The rows of API.md §7.2, in table order: the first that applies names the reason.
const (
	ReasonNone Reason = iota
	ReasonComputed
	ReasonLayered
	ReasonFormat
	ReasonInput
	ReasonKey
	ReasonPseudo
	ReasonOrder
	ReasonLayer
)

// The operations of API.md §8.3.
const (
	OpSet Op = iota
	OpReset
	OpAdd
	OpInsert
	OpAddEntry
	OpRemove
	OpMove
	OpRename
	OpRetire
	OpUnretire
	OpSetCase
)

// Where a value's source stands while a path is walked from its root (API.md W3).
const (
	stTree state = iota
	stAbsent
	stSpread
	stComputed
	stFormat
	stLayered
	stOpaque // an amendment's expression: replaced whole, nothing below it is a source
)

// The forms a source can take (API.md W1, §7.2 format).
const (
	formComputed form = iota
	formLiteral
	formJSON
	formFormat
)

// keyDepth is how many steps a key field lies below its keyed list: the element, then the field.
const keyDepth = 2

// layerFileSuffix names a layer file W11 creates: <package dir>/<layer>.layer.canon.
const layerFileSuffix = ".layer.canon"

// rootMark starts a path written with a root, `@root/rest` (WIRE.md §2.1).
const rootMark = '@'

// pathSep separates the directories of a display path.
const pathSep = "/"

// fmtPathError is a PathError's text: the sentinel, the segment.
const fmtPathError = "%v at segment %d"

// rootSeg is PathError.Seg for a failure at the root.
const rootSeg = -1

// forms classifies the expressions that can state a source (API.md W1); any other is computed.
var forms = map[syntax.NodeKind]func(*check.Info, syntax.Expr) form{
	syntax.KindIntLit:       alwaysLiteral,
	syntax.KindFloatLit:     alwaysLiteral,
	syntax.KindDurationLit:  alwaysLiteral,
	syntax.KindRawStringLit: alwaysLiteral,
	syntax.KindRegexLit:     alwaysLiteral,
	syntax.KindBoolLit:      alwaysLiteral,
	syntax.KindNoneLit:      alwaysLiteral,
	syntax.KindListLit:      alwaysLiteral,
	syntax.KindTypedLit:     alwaysLiteral,
	syntax.KindStringLit:    stringForm,
	syntax.KindBraceLit:     braceForm,
	syntax.KindIdentExpr:    identForm,
	syntax.KindSelectorExpr: selectorForm,
	syntax.KindLoadExpr:     loadForm,
}

// steppers moves a walk one step from each state (API.md W3).
var steppers = [...]func(*judge, cursor, int) cursor{
	stTree:     treeStep,
	stAbsent:   absentStep,
	stSpread:   leave,
	stComputed: stay,
	stFormat:   stay,
	stLayered:  stay,
	stOpaque:   leave,
}

// rows are API.md §7.2's rows in table order: the first that applies names the reason.
var rows = [...]func(*judge) Reason{computedRow, layeredRow, formatRow, inputRow, keyRow, pseudoRow, orderRow, layerRow}

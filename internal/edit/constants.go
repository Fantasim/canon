package edit

import "github.com/fantasim/canonlang/internal/diag"

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

// The constructors a ValueError names what was given by (API.md §8.2).
const (
	nameBool    = "Bool"
	nameInt     = "Int"
	nameFloat   = "Float"
	nameStr     = "Str"
	nameDur     = "Dur"
	nameMember  = "Member"
	nameKey     = "Key"
	nameIntKey  = "IntKey"
	namePathKey = "PathKey"
	nameCase    = "Case"
	callOpen    = "("
	callClose   = ")"
)

// form is the shape of a Lit that typing dispatches on; formOther is a scalar.
type form uint8

// The Lit forms typing dispatches on.
const (
	formOther form = iota
	formNone
	formList
	formObj
	formMap
	formJSON
	formSource
)

// litNames name the forms describe writes without an argument.
var litNames = [...]string{
	formNone: "None", formList: "List", formObj: "Obj", formMap: "Map", formJSON: "FromJSON", formSource: "Source",
}

// Number texts.
const (
	floatFormat  = 'g'
	shortestPrec = -1
	float32Bits  = 32
	exponentMark = "e"
)

// The details of a ValueError (API.md V1).
const (
	fmtValueError       = "%v: expected %s, got %s"
	detailAt            = "at "
	detailSep           = ": "
	detailFraction      = "holds a fraction of a millisecond"
	detailNotFinite     = "not a finite number"
	detailDependent     = "a type computed from a value takes a name only"
	detailField         = "no such field: "
	detailTwice         = "field given twice: "
	detailNotLiteral    = "not a Canon literal"
	detailNotContextual = "not a contextual name"
	detailRange         = "integer out of range"
	detailNameKey       = "a bare name keys only an enum or a ref"
	detailUndecoded     = "not decodable"
	detailInput         = "an input field has no value to give: "
	detailDeep          = "nested too deep"
	maxLitDepth         = 512
)

// The files a FromJSON or a Source is read from; a Source is the value of a `let` (API.md §8.2).
const (
	jsonName     = "value.json"
	sourceName   = "value.canon"
	sourcePrefix = "package _v\n\nlet _v = "
)

// The kinds of Ref (API.md R7), in the order R8 sorts references of one span.
const (
	RefValue RefKind = iota
	RefKey
	RefCode
	RefView
	RefCheck
	RefLayer
)

// The kinds of Change (API.md §8.1).
const (
	ChangeModified ChangeKind = iota
	ChangeCreated
	ChangeDeleted
	ChangeRenamed
)

// The members of an operation's JSON form as bits of a set; `value` and `source` are one (E24).
const (
	mOp members = 1 << iota
	mPath
	mValue
	mKey
	mIndex
	mCase
)

// The entries of jsonMembers, in the order the JSON form writes them.
const (
	iOp = iota
	iPath
	iIndex
	iKey
	iCase
	iValue
	iSource
)

// Texts of the JSON form of an operation.
const (
	jsonOpen     = '{'
	jsonClose    = '}'
	jsonComma    = ','
	jsonColon    = ':'
	jsonNullByte = 'n' // no JSON value but null starts with it
	textObject   = "an object"
	listSep      = ", "
	fmtOpJSON    = "%w: %s"
	fmtOpJSONErr = "%w: %s: %w"
	fmtBadMember = "%w: %w"
)

// opNames are the `op` strings of API.md E24, by operation.
var opNames = [...]string{
	OpSet: "set", OpReset: "reset", OpAdd: "add", OpInsert: "insert", OpAddEntry: "addEntry", OpRemove: "remove",
	OpMove: "move", OpRename: "rename", OpRetire: "retire", OpUnretire: "unretire", OpSetCase: "setCase",
}

// opShapes are the members each operation takes and needs (API.md §8.3): SetCase's fields are optional.
var opShapes = [...]opShape{
	OpSet: {mValue, mValue}, OpReset: {}, OpAdd: {mValue, mValue}, OpInsert: {mIndex | mValue, mIndex | mValue},
	OpAddEntry: {mKey | mValue, mKey | mValue}, OpRemove: {}, OpMove: {mIndex, mIndex}, OpRename: {mKey, mKey},
	OpRetire: {}, OpUnretire: {}, OpSetCase: {mCase | mValue, mCase},
}

// jsonMembers are the members of an operation's JSON form (API.md §8.8), in writing order.
var jsonMembers = [...]jsonMember{
	iOp: {"op", mOp, readOp}, iPath: {"path", mPath, readPath}, iIndex: {"index", mIndex, readIndex},
	iKey: {"key", mKey, readKey}, iCase: {"case", mCase, readCase}, iValue: {"value", mValue, readValue},
	iSource: {"source", mValue, readSource},
}

// recheckCodes are the wire findings about a value, not its static shape: the re-check reports
// them (API.md V2, V3; log-2026-09-29 M4 U4a).
var recheckCodes = map[diag.Code]bool{
	diag.E3302.Def().Code: true, diag.E3201.Def().Code: true, diag.E3202.Def().Code: true, diag.E3102.Def().Code: true,
	diag.E3317.Def().Code: true,
}

package types

import (
	"math"
	"regexp"
)

// The kinds of TYPES.md §2; Refined and KeyedList are not kinds (decision 46).
const (
	Bool Kind = iota
	Int
	Float
	String
	Duration
	Enum
	Record
	Variant
	Case
	VariantKind
	Optional
	List
	Map
	DepMap
	Table
	Ref
	LitUnion
	Never
	Range
	Func
	Pair
	TypeApp
	DepUnion
	Define
	Any
	None
	Error
)

// The wire units of @json(unit:) and @cpp(unit:); the zero value is the default, ms.
const (
	UnitMs Unit = iota
	UnitS
	UnitM
	UnitH
	UnitD
)

// The encodings of @json(int) and @json(bits); EncPlain is neither.
const (
	EncPlain Enc = iota
	EncInt
	EncBits
)

// The collections a ref can target (RES-03).
const (
	CollLet CollKind = iota
	CollField
	CollDefines
)

// Where a type argument is read from (TYPES.md §11.1).
const (
	ArgParam ArgSource = iota
	ArgField
	ArgKey
)

const (
	bitsDefault = 64
	bitsHalf    = 32
	bitsQuarter = 16
	bitsByte    = 8

	// DurationLimit is the largest stored Duration in ms (TYPES.md §7.2).
	DurationLimit int64 = 9_223_372_036_854

	msPerSecond int64 = 1_000
	msPerMinute int64 = 60 * msPerSecond
	msPerHour   int64 = 60 * msPerMinute
	msPerDay    int64 = 24 * msPerHour

	pairKeys = 2

	floatMaxExp10 = 21
	floatMinExp10 = -6
)

// The predeclared scalar types (TYPES.md §2, TYP-03).
var (
	BoolType     = Basic{K: Bool}
	IntType      = Basic{K: Int, Bits: bitsDefault, Signed: true}
	Int8Type     = Basic{K: Int, Bits: bitsByte, Signed: true}
	Int16Type    = Basic{K: Int, Bits: bitsQuarter, Signed: true}
	Int32Type    = Basic{K: Int, Bits: bitsHalf, Signed: true}
	UInt8Type    = Basic{K: Int, Bits: bitsByte}
	UInt16Type   = Basic{K: Int, Bits: bitsQuarter}
	UInt32Type   = Basic{K: Int, Bits: bitsHalf}
	UInt64Type   = Basic{K: Int, Bits: bitsDefault}
	FloatType    = Basic{K: Float, Bits: bitsDefault}
	Float32Type  = Basic{K: Float, Bits: bitsHalf}
	StringType   = Basic{K: String}
	DurationType = Basic{K: Duration}
)

// The singletons of the kinds that carry nothing.
var (
	AnyType   Type = builtin{k: Any, text: "_"}
	NoneType  Type = builtin{k: None, text: "none"}
	ErrorType Type = builtin{k: Error, text: "invalid"}
	NeverType Type = builtin{k: Never, text: "Never"}
	RangeType Type = builtin{k: Range, text: "Range"}
)

// DefineType is the element record of load.defines tables: `record Define { value: Int }`.
var DefineType = &RecordType{Name: defineName, define: true, Fields: []*Field{{Name: defineField, Wire: defineField, WirePath: []string{defineField}, Type: IntType}}}

var basicNames = map[Basic]string{
	BoolType: "Bool", IntType: "Int", Int8Type: "Int8", Int16Type: "Int16", Int32Type: "Int32",
	UInt8Type: "UInt8", UInt16Type: "UInt16", UInt32Type: "UInt32", UInt64Type: "UInt64",
	FloatType: "Float", Float32Type: "Float32", StringType: "String", DurationType: "Duration",
}

var unitNames = [...]string{UnitMs: "ms", UnitS: "s", UnitM: "m", UnitH: "h", UnitD: "d"}

var unitMillis = [...]int64{UnitMs: 1, UnitS: msPerSecond, UnitM: msPerMinute, UnitH: msPerHour, UnitD: msPerDay}

// durationParts is the decomposition order of the duration text form, largest unit first.
var durationParts = [...]Unit{UnitD, UnitH, UnitM, UnitS, UnitMs}

// quoteEscapes are the two-character escapes of a nested string (STD-06).
var quoteEscapes = [...]string{'"': `\"`, '\\': `\\`, '\n': `\n`, '\t': `\t`, '\r': `\r`, maxByte: ""}

var reWord = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

const (
	firstPrintable = 0x20
	deleteByte     = 0x7F
	maxByte        = 0xFF
)

// The text of type syntax, one constant per token.
const (
	textSep        = ", "
	textDot        = "."
	textOpen       = "("
	textClose      = ")"
	textOpenList   = "["
	textCloseList  = "]"
	textOpenMap    = "{"
	textCloseMap   = "}"
	textSlash      = "/"
	textQuote      = `"`
	textEscapeOpen = `\u{`
	textOptional   = "?"
	textWhere      = " where "
	textKeyedBy    = " keyed by "
	textTable      = "table "
	textStable     = "stable "
	textRef        = "ref "
	textUnion      = " | "
	textArrow      = ") -> "
	textFn         = "fn("
	textPair       = "Pair("
	textKind       = "Kind("
	textAsset      = "asset("
	textExt        = ", ext: ["
	textDepAll     = "(*)"
	textIn         = " in "
	textColon      = ": "
	textRange      = ".."
	textRangeIncl  = "..="
	textZeroDur    = "0s"
	textZero       = "0"
	textExp        = "e"
	textMinus      = "-"
	textPlus       = "+"
	textPoint      = "0."
	defineName     = "Define"
	defineField    = "value"
)

const maxInt = math.MaxInt64

// The formats a load reads a file as, told apart by extension or `format:` (WIRE.md §6.2).
const (
	FormatUnknown LoadFormat = iota
	FormatJSON
	FormatCSV
	FormatText
)

// formatWords are each format's `format:` symbol and file extension (WIRE.md §6.2).
var formatWords = [...]struct{ symbol, ext string }{
	FormatJSON: {"json", ".json"},
	FormatCSV:  {"csv", ".csv"},
	FormatText: {"text", ".txt"},
}

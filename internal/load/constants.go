package load

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/types"
)

// Load forms (WIRE.md §6.1): loadForm is the bare `load(path)`, whose method is nil.
const (
	loadForm    = ""
	wordLoad    = "load"
	methodDir   = "dir"
	methodCSV   = "csv"
	methodText  = "text"
	formDefines = "defines"
	doubleStar  = "**"
	sepStr      = "/"
	dotPrefix   = "."
	backslash   = `\`
	dotSeg      = "."
	dotDotSeg   = ".."
)

// Named options of a load form (WIRE.md §6.1).
const (
	optionAt     = "at"
	optFormat    = "format"
	optPartial   = "partial"
	optHeader    = "header"
	optionPrefix = "prefix"
)

// magicChars start a glob's pattern part; the segments before it are a literal path (WIRE.md §6.5).
const magicChars = "*?[{"

// globVerdict is validateGlob's outcome kind.
type globVerdict uint8

// globEmptySeg is E7001 empty, globDotSeg is E7001 dot (globCheck.seg carries "." or ".."), globInvalid is E7005 (globCheck.kind carries the cause).
const (
	globWellFormed globVerdict = iota
	globEmptySeg
	globDotSeg
	globInvalid
)

// Causes of ErrUnsupported, what this milestone does not read yet (DECISIONS 196).
const (
	causeArticle       = "a "
	causeNotLiteral    = " call this milestone reads only in its plain literal form"
	causeDirOption     = "a load.dir option, this milestone reads only its literal form"
	causeDirFormat     = "a load.dir format: this milestone reads only json"
	causeDirFileFormat = "a load.dir file whose format is not json"
	causeUnknownForm   = "a load form this milestone does not recognize"
)

// formatNames is each types.LoadFormat's E7006 message text, a name types itself does not provide (WIRE.md §6.2).
var formatNames = [...]string{
	types.FormatUnknown: "unknown",
	types.FormatJSON:    "json",
	types.FormatCSV:     "csv",
	types.FormatText:    "text",
}

// formOptions is each form's allowed named options (WIRE.md §6.1's table, DECISIONS 26).
var formOptions = map[string]map[string]bool{
	loadForm:    {optionAt: true, optPartial: true, optFormat: true, optHeader: true},
	methodDir:   {optionAt: true, optPartial: true, optFormat: true},
	methodCSV:   {optHeader: true, optPartial: true},
	methodText:  {},
	formDefines: {optionPrefix: true},
}

// atStepKind is one step of an `at:` path (WIRE.md §6.3).
type atStepKind int

const (
	atName atStepKind = iota
	atIndex
	atStar
)

// atStopChars end an at: name segment; atEscapable is what `\` may escape inside one (WIRE.md §6.3).
const (
	atStopChars  = ".*[]"
	atEscapable  = `.*[]\`
	atZeroDigit  = '0'
	atOpenIndex  = '['
	atCloseIndex = ']'
)

// defineLineRe is a #define line: NAME and the classifying rest of the line captured.
var defineLineRe = regexp.MustCompile(`^([ \t]*)#[ \t]*define[ \t]+([A-Za-z_][A-Za-z0-9_]*)(.*)$`)

// defineLineRe's FindStringSubmatchIndex offsets.
const (
	reHashEnd   = 3
	reNameStart = 4
	reNameEnd   = 5
	reRestStart = 6
	reRestEnd   = 7
)

// maxShift is WIRE.md §6.8's #define shift-count bound: 0..63.
const maxShift = 63

// twoBytes is every 2-byte marker load's readers consume as one unit: "\r\n", `\` LF, `""`,
// "//", "/*", "*/", an at: path's escape.
const twoBytes = 2

// defOp is one pending operator of the #define evaluator's operator stack (WIRE.md §6.8).
type defOp uint8

const (
	opOr defOp = iota
	opAnd
	opShl
	opShr
	opAdd
	opSub
	opNeg   // unary "-"
	opParen // an open "(", which no reduction crosses
)

// defPrec is each operator's binding strength in WIRE.md §6.8's grammar, tighter higher.
var defPrec = [...]int{opOr: 1, opAnd: 2, opShl: 3, opShr: 3, opAdd: 4, opSub: 4, opNeg: 5, opParen: 0}

// defBinaryOps is each binary operator's text, the two-byte shifts first.
var defBinaryOps = [...]struct {
	text string
	op   defOp
}{{"<<", opShl}, {">>", opShr}, {"|", opOr}, {"&", opAnd}, {"+", opAdd}, {"-", opSub}}

// An integer literal's hex prefixes and its ignored suffixes (WIRE.md §6.8's integer rule).
const (
	hexPrefixLower = "0x"
	hexPrefixUpper = "0X"
	intSuffixes    = "uUlL"
)

// blockCommentClose ends a C block comment in a #define header (WIRE.md §6.8 step 3).
const blockCommentClose = "*/"

// Integer literal bases of WIRE.md §6.8's #define expression grammar.
const (
	hexBase = 16
	octBase = 8
	decBase = 10
)

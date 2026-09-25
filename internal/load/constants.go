package load

import "regexp"

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

// Causes of E7005, an invalid load.dir glob segment (WIRE.md §6.5, a fixed vocabulary for now).
const (
	causeDoubleStar = "** must be a whole segment"
	causeBracket    = "unclosed ["
	causeBrace      = "unclosed, empty or nested {"
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

// Causes of E7004, chosen by errors.Is rather than an OS message or a path (DOCTRINE.md §5).
const (
	causeMissing    = "no such file or directory"
	causePermission = "permission denied"
	causeNotDir     = "not a directory"
	causeIsDir      = "is a directory"
	causeNotRegular = "not a regular file"
	causeTooLarge   = "too large"
	causeUnreadable = "unreadable"
)

// wireFormat is a loaded file's format, told apart by extension or `format:` (WIRE.md §6.2).
type wireFormat int

// load.dir still reads only fmtJSON matches (DECISIONS 173).
const (
	fmtUnknown wireFormat = iota
	fmtJSON
	fmtCSV
	fmtText
)

// formatNames is each wireFormat's E7006/E7007 message text (WIRE.md §6.2).
var formatNames = [...]string{fmtUnknown: "unknown", fmtJSON: "json", fmtCSV: "csv", fmtText: "text"}

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

// foreignBOMs are non-UTF-8 byte order marks, longest first: load reads this rule for csv and text.
var foreignBOMs = [...]string{"\x00\x00\xfe\xff", "\xff\xfe\x00\x00", "\xfe\xff", "\xff\xfe"}

const utf8BOM = "\xef\xbb\xbf"

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

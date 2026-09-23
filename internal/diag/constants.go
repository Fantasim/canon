package diag

import "time"

// Limits fixed by the documents.
const (
	// DefaultMaxFindings is the findings a Bag keeps per package (API.md §2.1 MaxFindings, F7).
	DefaultMaxFindings = 1000
	// MaxStackFrames is the frames a finding keeps, innermost first (EVALUATION.md §13).
	MaxStackFrames = 16
)

// Template grammar (ERRORS.md §1.2).
const (
	placeholderOpen  = '{'
	placeholderClose = '}'
	lineBreak        = "\n"
	escBackslash     = `\\`
	escNewline       = `\n`
)

// templateEscapes maps each escape of ERRORS.md §1.2 to the text it renders.
var templateEscapes = [...]struct{ from, to string }{
	{"{{", "{"},
	{"}}", "}"},
	{escBackslash, `\`},
	{escNewline, lineBreak},
}

// Argument renderings (ERRORS.md §1.3).
const (
	listSep      = ", "
	chainSep     = " -> "
	runePrefix   = "U+"
	runeHexWidth = 4
	zeroDigit    = "0"
	fragmentMark = "#"
	percent      = '%'
	locSep       = ":"
	whitespace   = " \t\n\r"
	decimalBase  = 10
	hexBase      = 16
	hexDigits    = "0123456789ABCDEF"
	hexShift     = 4
	hexLowMask   = 0x0f
)

// fragmentSafe is what a fragment holds unencoded besides letters and digits (RFC 3986 §3.5).
const fragmentSafe = "-._~!$&'()*+,;=:@/?"

// Note templates (ERRORS.md §1.5); checkWord is also the checkUnnamed template.
const (
	noteCheckTemplate = "check {name}"
	noteNameArg       = "name"
	checkWord         = "check"
)

// The text form (API.md §4.4).
const (
	gap             = "  "
	codeOpen        = "["
	codeClose       = "]"
	pathSep         = ": "
	layerPrefix     = "set by layer "
	relatedWords    = "expected by"
	framePrefix     = "in "
	space           = " "
	parenOpen       = "("
	noteOpen        = " ("
	noteClose       = ")"
	moreFramesClose = " more frames)"
)

// The summary line (API.md F15); the plural nouns are also the JSON summary's keys.
const (
	summaryFormat   = "%s, %s in %s%s %s"
	notShownFormat  = ", %d not shown"
	millisFormat    = "(%d ms)"
	secondsFormat   = "(%d.%d s)"
	durationGolden  = "(…)"
	nounError       = "error"
	errorsWord      = "errors"
	nounWarning     = "warning"
	warningsWord    = "warnings"
	nounPackage     = "package"
	packagesWord    = "packages"
	tenth           = 100 * time.Millisecond
	tenthsPerSecond = 10
)

// The forms of findings.
const (
	FormatText Format = iota
	FormatJSON
)

// The JSON form's keys (API.md F5, CLI.md §2.4).
const (
	keySeverity = "severity"
	keyCode     = "code"
	keyFile     = "file"
	keyLine     = "line"
	keyCol      = "col"
	keyEndLine  = "endLine"
	keyEndCol   = "endCol"
	keyPointer  = "pointer"
	keyPackage  = "package"
	keyPath     = "path"
	keyMessage  = "message"
	keyLayer    = "layer"
	keyRelated  = "related"
	keyStack    = "stack"
	keyReads    = "reads"
	keyNote     = "note"
	keyFn       = "fn"
	keySummary  = "summary"
	keyMillis   = "ms"
	keyTrunc    = "truncated"
)

// JSON text (RFC 8259; strings as WIRE.md §7.3 writes them).
const (
	quote            = '"'
	backslash        = '\\'
	objOpen          = '{'
	objClose         = '}'
	arrOpen          = '['
	arrClose         = ']'
	comma            = ','
	colon            = ':'
	controlLimit     = 0x20
	unicodeEscape    = `\u00`
	unicodeLetter    = 'u'
	hexLower         = "0123456789abcdef"
	hexRuneDigits    = 4
	hexRuneBits      = 16
	shortEscapeWidth = 2
	uEscapeWidth     = shortEscapeWidth + hexRuneDigits
	pairWidth        = 2 * uEscapeWidth
	surrogateLow     = 0xdc00
)

// jsonShortEscapes are the two-character escapes WIRE.md §7.3 writes, by byte.
var jsonShortEscapes = map[byte]string{
	'"':  `\"`,
	'\\': escBackslash,
	'\b': `\b`,
	'\t': `\t`,
	'\n': escNewline,
	'\f': `\f`,
	'\r': `\r`,
}

// jsonDecodedEscapes are the one-letter escapes RFC 8259 §7 reads, by the letter after `\`.
var jsonDecodedEscapes = map[byte]byte{
	'"':  '"',
	'\\': '\\',
	'/':  '/',
	'b':  '\b',
	'f':  '\f',
	'n':  '\n',
	'r':  '\r',
	't':  '\t',
}

package jsonsrc

// The kinds of JSON values.
const (
	Null Kind = iota
	Bool
	Number
	String
	Array
	Object
	kindCount
)

// maxDepth is the deepest nesting of arrays and objects a source may have (WIRE.md §3.1).
const maxDepth = 512

// indexedMembers is the member count from which an object's keys are found through a map.
const indexedMembers = 16

// UTF8BOM is the UTF-8 byte order mark: skipped at a JSON source's start (WIRE.md §3.1), never written (§7.1).
const UTF8BOM = "\xef\xbb\xbf"

const (
	byteValues = 256

	jsonSpace         = " \t\n\r"
	shortEscapes      = "\"\\/bfnrt"
	hexDigits         = "0123456789abcdef"
	hexDigitsUpper    = "0123456789ABCDEF"
	hexBits           = 4
	hexEscapeDigits   = 4
	escapePrefixWidth = 2
	hexEscapeWidth    = escapePrefixWidth + hexEscapeDigits
	surrogateLow      = 0xdc00
	escapeLead        = '\\'
	unicodeLetter     = 'u'
	quote             = '"'
	minus             = '-'
	zero              = '0'
	nine              = '9'
	controlLimit      = ' '
	dot               = '.'
	exponents         = "eE"
	signs             = "+-"

	openObject  = '{'
	closeObject = '}'
	openArray   = '['
	closeArray  = ']'
	colon       = ':'
	comma       = ','
	newline     = '\n'
	indentUnit  = "  "
	keySep      = ": "
	emptyArray  = "[]"
	emptyObject = "{}"

	pointerSep     = "/"
	tilde          = "~"
	escapedTilde   = "~0"
	escapedSlash   = "~1"
	pointerSpecial = "~/"
)

// literalWords are the JSON words, each dispatched on its first byte.
var literalWords = [...]struct {
	word string
	kind Kind
}{{"null", Null}, {"true", Bool}, {"false", Bool}}

// foreignBOMs are the UTF-32 and UTF-16 byte order marks (WIRE.md §3.1), longest first.
var foreignBOMs = [...]string{"\x00\x00\xfe\xff", "\xff\xfe\x00\x00", "\xfe\xff", "\xff\xfe"}

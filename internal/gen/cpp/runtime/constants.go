package cppruntime

// Where Scan, the C++ lexer of Identifiers and of gen/cpp's macrosIn, skips (comments, string and character literals).
const (
	lineComment = "//"
	blockOpen   = "/*"
	blockClose  = "*/"
	newline     = "\n"
	blank       = " "
	quoteMark   = '"'
	charMark    = '\''
	escapeMark  = '\\'
	underscore  = '_'
)

// The runtime stamp (CODEGEN.md §7.4, DECISIONS 436): the mark the texts hold in its place, and its length in hex digits.
const (
	stampMark = "@CANON_STAMP@"
	stampLen  = 12
)

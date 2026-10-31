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

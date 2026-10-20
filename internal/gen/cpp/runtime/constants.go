package cppruntime

// Where the C++ lexer of Identifiers skips (comments, string and character literals).
const (
	lineComment = "//"
	blockOpen   = "/*"
	blockClose  = "*/"
	newline     = "\n"
	quoteMark   = '"'
	charMark    = '\''
	escapeMark  = '\\'
	underscore  = '_'
)

package load

// This milestone reads only `load.dir` of json (WIRE.md §6, DECISIONS 196).
const (
	methodDir  = "dir"
	doubleStar = "**"
	sepStr     = "/"
	dotPrefix  = "."
	backslash  = `\`
	dotSeg     = "."
	dotDotSeg  = ".."
	crlfLen    = 2
)

// magicChars start a glob's pattern part; the segments before it are a literal path (WIRE.md §6.5).
const magicChars = "*?[{"

// Causes of E7005, an invalid load.dir glob segment (WIRE.md §6.5, a fixed vocabulary for now).
const (
	causeDoubleStar = "** must be a whole segment"
	causeBracket    = "unclosed ["
	causeBrace      = "unclosed, empty or nested {"
)

// Causes of E7004, chosen by errors.Is rather than an OS message or a path (DOCTRINE.md §5).
const (
	causeMissing    = "no such file or directory"
	causePermission = "permission denied"
	causeNotDir     = "not a directory"
	causeTooLarge   = "too large"
	causeUnreadable = "unreadable"
)

// wireFormat is load.dir's file format, told apart by extension (WIRE.md §6.2).
type wireFormat int

// Only fmtJSON is read this milestone; fmtCSV and fmtText are recognized but ErrUnsupported.
const (
	fmtUnknown wireFormat = iota
	fmtJSON
	fmtCSV
	fmtText
)

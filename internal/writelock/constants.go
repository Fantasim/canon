package writelock

import "time"

// The two modes, the stronger last.
const (
	Shared Mode = iota
	Exclusive
)

// writersPath holds the locks, below the project directory, each named lockPrefix and its
// package, so that no package name is a Windows device name (DECISIONS 352).
const (
	writersPath = ".canon/writers"
	pathSep     = "/"
	lockPrefix  = "pkg."
)

// The modes of what is created: a plain create's, before the umask (DECISIONS 329).
const (
	fileMode = 0o666
	dirMode  = 0o777
)

// pollEvery is how often a waiting writer tries again, never blocked in a system call.
const pollEvery = 100 * time.Millisecond

// The holder's line, and what stands for a host the system does not name.
const (
	lineFormat  = "%s %d\n"
	lineEnd     = "\n"
	unknownHost = "?"
	hiddenMark  = "."
	badNameRune = `/\`
	maxLine     = 256
)

// Error formats and ops.
const (
	fmtAt   = "%w: %q"
	fmtWrap = "%w"
	opOpen  = "open"
	opFlock = "flock"
)

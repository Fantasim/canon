package workspace

import "time"

// racyWindow: a file hashed this soon after its mtime is hashed again (API.md S1).
const racyWindow = 2 * time.Second

// verdictLimit is how many M9 verdicts a project keeps, the least recently used dropped (API.md M9).
const verdictLimit = 256

// The revisions remembered, as deltas with a full copy every fullEvery (API.md S4).
const (
	historyLen = 64
	fullEvery  = 16
)

// The kinds of entry a snapshot holds: a file's content, a directory's listing, a stat, the
// real path a name's links lead to; kindSources, only in the history, is a listing's sources.
const (
	kindFile kind = iota
	kindDir
	kindStat
	kindLink
	kindSources
)

// What reading an entry gave: content, nothing there, or an error; classGone marks, in a
// history delta, a name the snapshot no longer holds.
const (
	classOK class = iota
	classMissing
	classUnreadable
	classGone
)

// The bits of a stat's content key (API.md S5).
const (
	statDir byte = 1 << iota
	statRegular
)

// The causes of a new snapshot (API.md §12 EventCause).
const (
	CauseExternal Cause = "external"
	CauseEdit     Cause = "edit"
	CauseOverlay  Cause = "overlay"
)

// The operations whose concurrent identical calls share one computation (API.md S8).
const (
	OpAnalyze   Op = "analyze"
	OpPackages  Op = "packages"
	OpViewModel Op = "viewmodel"
	OpTest      Op = "test"
	OpBuild     Op = "build"
	OpLockCheck Op = "lockcheck"
	opEvaluate  Op = "evaluate"
	opDraft     Op = "draft"
)

// refreshWorkers is how many goroutines stat a snapshot's entries at a refresh (API.md S1).
const refreshWorkers = 8

// listingDisplay stands for a scan that failed in a revision (API.md S3); pathSep separates names.
const (
	listingDisplay = "."
	pathSep        = "/"
)

// Error texts (API.md X1).
const (
	fmtBadPath = "%w: %s"
	fmtWrap    = "%w: %w"
	textSep    = ": "
	listSep    = ", "
)

// A path inside another starts with one of these (API.md P8).
const (
	fieldMark = "."
	keyMark   = "["
)

// A watch's timing (API.md W14) and how often it polls, or resyncs an OS watcher.
const (
	quietFor    = 100 * time.Millisecond
	capFor      = time.Second
	pollEvery   = 50 * time.Millisecond
	resyncEvery = time.Second
)

// The Warn line of a watcher whose OS notifications failed, and its error's key.
const (
	msgPolling = "the OS cannot watch the project's files: polling them instead"
	logError   = "error"
)

// What a RenameName's preservation check writes (API.md E35): a declaration's key, the place of
// an identifier and what a captured one would name.
const (
	keySep         = "#"
	useSep         = "&"
	packageSep     = ":"
	fmtPlace       = "%s:%d:%d"
	fmtCaptured    = "%s would name %s"
	textNothing    = "nothing"
	textBuiltin    = "the built-in "
	textDeclaredAt = ", declared at "
)

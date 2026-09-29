package workspace

import "time"

// racyWindow: a file hashed this soon after its mtime is hashed again (API.md S1).
const racyWindow = 2 * time.Second

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
)

// listingDisplay stands for a scan that failed in a revision (API.md S3); pathSep separates names.
const (
	listingDisplay = "."
	pathSep        = "/"
)

// Error texts (API.md X1).
const (
	fmtBadPath = "%w: %s"
	textSep    = ": "
	listSep    = ", "
)

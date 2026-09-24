package build

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/check"
	gogen "github.com/fantasim/canonlang/internal/gen/go"
	jsongen "github.com/fantasim/canonlang/internal/gen/json"
	"github.com/fantasim/canonlang/internal/ir"
)

// The revision of a snapshot: "r1:" and the SHA-256 of its read-set listing (API.md S3).
const (
	revisionPrefix = "r1:"
	listingSep     = "\x00"
	listingEnd     = "\n"
	unreadMark     = "unreadable" // for a file that cannot be read, in place of its SHA-256
	listingMark    = "."          // the project directory: its listing, and path.Dir's top
)

// Paths: a root's mark, separators, the lock's name (LOCK.md §2.1), text line ends.
const (
	rootMark       = "@"
	pathSep        = "/"
	qnameSep       = "."
	lockName       = "canon.lock"
	lineEnd        = listingEnd
	carriageReturn = "\r"
)

// Generated-file markers (CODEGEN.md §2.4, WIRE.md §8.4), the one adoptable kind of output.
const (
	bom        = "\xef\xbb\xbf"
	objectOpen = '{'
	headerExt  = ".h"
)

// Written files: a temporary file's name around its target's, and the modes of what is created.
const (
	tempPrefix = "."
	tempSuffix = ".canon-tmp"
	fileMode   = 0o600
	dirMode    = 0o750
)

// The statuses of an output: Written is new or changed content, which the build writes.
const (
	StatusWritten Status = iota
	StatusUnchanged
	StatusAdopted
	StatusStale // Check mode: the build would write it
)

// Error formats: a wrapped error that already names its file, and the context the others add.
const (
	fmtWrap         = "%w"
	fmtWrapInternal = "%w: %w"
	fmtNoProgram    = "%w: the checker returned no program"
	fmtLoad         = "%s: %s:%d:%d"
	fmtLoadCause    = "%s (%s): %s:%d:%d"
	fmtPackage      = "package %s: %w"
	fmtEmit         = "package %s, emit %q: %w"
	fmtNoGenerator  = "package %s, emit %s: %w"
	fmtNoValue      = "%w: package %s: %s has no value"
	fmtNoLoadSite   = "%w: a forced load expression is in no file of the program"
	// fmtDisplayCause and fmtDisplayOpCause name a write's display path in place of the absolute one a *fs.PathError carries, the op kept when there is one (DECISIONS 201).
	fmtDisplayCause   = "%s: %w"
	fmtDisplayOpCause = "%s: %s: %v"
)

var (
	codeMarker   = regexp.MustCompile(`^// (Code generated|GENERATED) by canon\b.* DO NOT EDIT\.$`)
	schemaMarker = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)+@[0-9a-f]{8}$`)
	viewMarker   = regexp.MustCompile(`^canon-vm/[1-9][0-9]*$`)
	// runtimeFiles are the runtime helper files, by their path in an emit's directory.
	runtimeFiles = [...]string{"rt/rt.go", "canon_runtime.h", "canon_runtime_json.h"}
	// generators are the code generators by target; a missing one is not written yet.
	generators = [...]ir.Generator{ir.TargetGo: gogen.Generate, ir.TargetJSON: jsongen.Generate, ir.TargetView: nil}
	// targetWords are the emit target words (CODEGEN.md §2.1).
	targetWords = [...]string{
		ir.TargetGo: check.TargetGo, ir.TargetCpp: check.TargetCpp, ir.TargetTS: check.TargetTS,
		ir.TargetJSON: check.TargetJSON, ir.TargetView: check.TargetView,
	}
)

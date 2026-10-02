package build

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/conform"
	"github.com/fantasim/canonlang/internal/eval"
	cppgen "github.com/fantasim/canonlang/internal/gen/cpp"
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

// The cache: the layers of a check key are joined by layerSep; a file set past compactFloor
// bytes holding compactRatio times what the latest snapshot used starts anew.
const (
	layerSep     = listingSep
	compactFloor = 64 << 20
	compactRatio = 4
)

// lineageCap is the Recheck lineages a cache generation keeps, one per set of packages checked.
const lineageCap = 3

const defaultVersion = "0.1.0"

// CompilerVersion is the compiler's version (WIRE.md §10, API.md §14); a var for -ldflags -X (decision 276).
var CompilerVersion = defaultVersion

// The build manifest's first line, line keywords, field separator, commands and hash mark (WIRE.md §10).
const (
	manifestHeader = "canon-manifest v1"
	lineSep        = " "
	kwCompiler     = "compiler"
	kwLanguage     = "language"
	kwLayer        = "layer"
	kwLang         = "lang"
	kwCommand      = "command"
	kwTarget       = "target"
	kwPackage      = "package"
	kwRoot         = "root"
	kwFile         = "file"
	kwGlob         = "glob"
	kwList         = "list"
	commandCheck   = "check"
	commandBuild   = "build"
	commandTest    = "test"
	hashPrefix     = "sha256:"
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
	objectOpen = '{'
	headerExt  = ".h"
)

// Written files: a temporary file's name around its target's, and the modes of what is created.
const (
	tempPrefix    = "."
	tempSep       = "."
	tempSuffix    = ".canon-tmp"
	tempRandBytes = 8  // the random part of the OS WriteFile's temporary name
	maxLinks      = 40 // the symbolic links the OS WriteFile follows to the file it writes
	linkOp        = "readlink"
	fileMode      = 0o600
	dirMode       = 0o750
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
	fmtLoad         = "%s: %s:%d:%d"
	fmtLoadCause    = "%s (%s): %s:%d:%d"
	fmtPackage      = "package %s: %w"
	fmtEmit         = "package %s, emit %q: %w"
	fmtNoGenerator  = "package %s, emit %s: %w"
	fmtNoValue      = "package %s: %s has no value"
	fmtUnplaced     = "package %s, emit %q: its out does not resolve"
	fmtUnknownLimit = "a vector cut short by an unknown limit %d"
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
	// generators are the code generators by target; a missing one is not written yet (view: viewmodel.go).
	generators = [...]ir.Generator{ir.TargetGo: gogen.Generate, ir.TargetCpp: cppgen.Generate, ir.TargetJSON: jsongen.Generate, ir.TargetView: nil}
	// limits maps the evaluator's limits onto conform's (ADR-0003).
	limits = [...]conform.Limit{eval.NoLimit: conform.NoLimit, eval.StepLimit: conform.StepLimit, eval.DepthLimit: conform.DepthLimit}
	// targetWords are the emit target words (CODEGEN.md §2.1).
	targetWords = [...]string{
		ir.TargetGo: check.TargetGo, ir.TargetCpp: check.TargetCpp, ir.TargetTS: check.TargetTS,
		ir.TargetJSON: check.TargetJSON, ir.TargetView: check.TargetView,
	}
)

// readConflict marks a number token loads read as two types or texts: kept as written (M9).
const readConflict = ""

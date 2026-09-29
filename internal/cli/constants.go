package cli

import (
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// Exit codes (CLI.md §2.5).
const (
	exitOK          = 0
	exitErrors      = 1
	exitUsage       = 2
	exitInternal    = 3
	exitWarnings    = 4
	exitInterrupted = 130
)

// Command names (CLI.md §1).
const (
	cmdCheck   = "check"
	cmdBuild   = "build"
	cmdInit    = "init"
	cmdNew     = "new"
	cmdVersion = "version"
	cmdTest    = "test"
	cmdExplain = "explain"
	cmdFmt     = "fmt"
	cmdRefs    = "refs"
)

// Flags (CLI.md §2.3, §3.1, §3.4).
const (
	flagProject     = "project"
	flagRoot        = "root"
	flagFormat      = "format"
	flagQuiet       = "quiet"
	flagQuietShort  = "q"
	flagMaxWarnings = "max-warnings"
	flagName        = "name"
	flagTarget      = "target"
	flagCheck       = "check"
	flagAdopt       = "adopt"
	flagLayer       = "layer"
	flagRun         = "run"
	flagVerbose     = "v"
	flagDepth       = "depth"
	flagDiff        = "diff"
	flagJSONSources = "json-sources"
	flagWatch       = "watch"
	formatText      = "text"
	formatJSON      = "json"
	rootAssign      = "="
	endOfFlags      = "--"
	unlimited       = -1
)

const (
	usageProject     = "project root (default: search upward from the current directory)"
	usageRoot        = "use `name=dir` for a declared root; repeatable"
	usageFormat      = "output `format`: text or json"
	usageQuiet       = "print errors only"
	usageMaxWarnings = "exit with code 4 when there are more than `n` warnings"
	usageName        = "project `name` (default: the directory's name)"
	usageTarget      = "emit only this `target` (go, cpp, ts, json, view); repeatable"
	usageCheck       = "write nothing; exit 1 if any output or lock would change"
	usageAdopt       = "take over the hand-written file at `path` (repeatable)"
	usageLayer       = "apply the layer `name`; repeatable, applied in order"
	usageRun         = "run only the tests whose name matches the RE2 `regex`"
	usageVerbose     = "also print each passing test"
	usageDepth       = "print parts `n` levels deep (default: every part)"
	usageFmtCheck    = "write nothing; list the files that are not formatted and exit 1"
	usageDiff        = "print the changes instead of writing"
	usageWatch       = "run again after every change, printing what changed"
	usageJSONSources = "also normalize the JSON files read by load to the canonical JSON layout"
)

// usageText lists the commands; the flags follow it (CLI.md §1).
const usageText = `usage: canon <command> [arguments] [flags]

commands:
  build [packages...]   check, then write the outputs of packages
  check [packages...]   parse and check packages, print findings
  explain <path>        print a value, its type and where each part was set
  fmt [paths...]        rewrite sources in the canonical layout
  init                  create project.canon in the current directory
  new <package>         create a package directory with a first file
  refs <path>           list every place that references an entry or member
  test [packages...]    run the test blocks of packages
  version               print the compiler, language and format versions

flags:
`

// buildTargets are the target words --target accepts, in SPEC §14's canonical order.
var buildTargets = [...]canon.Target{canon.TargetGo, canon.TargetCpp, canon.TargetTS, canon.TargetJSON, canon.TargetView}

// Output of version (CLI.md §3.14, IMPLEMENTATION-PLAN.md §8.5).
const (
	versionFormat = "canon %s (%s)\nlanguage %s\nformats %s, %s, %s\n"
	unknownCommit = "unknown"
	listSep       = ", "
)

// Files init and new write (CLI.md §3.1, §3.2).
const (
	gitignoreFile = ".gitignore"
	cacheIgnore   = ".canon/"
	initFormat    = "project %s {\n  canon: \"%s\"\n  roots {}\n  languages: [en]\n}\n"
	newFormat     = "/// Describe package %s here.\npackage %s\n"
	filePerm      = 0o600
	dirPerm       = 0o750
	lineBreak     = "\n"
	pathSep       = "/"
	dotSep        = "."
	parentDir     = ".."
)

// canon build's own text report (IMPLEMENTATION-PLAN.md §8.5).
const (
	fmtAdopting        = "adopting %s"
	targetHeaderSuffix = ":"
	outputIndent       = "  "
	lockHeader         = "lock:"
	stalePathPrefix    = "stale "
)

// canon test's text report (CLI.md §3.5, API.md F15).
const (
	fmtTestHead    = "%s  %s:%d  %s"
	fmtAt          = "%s:%d  %s"
	fmtFindingText = "%s[%s]  %s"
	fmtTestSummary = "%d passed, %d failed (%s)"
	fmtMillis      = "%d ms"
	fmtSeconds     = "%.1f s"
	tenthSecond    = 100 * time.Millisecond
	testFail       = "FAIL"
	testOK         = "ok"
	labelExpected  = "expected:"
	labelGot       = "got:"
	gotNoFinding   = "no finding"
	fmtPoisoned    = "reads poisoned %s"
	opEqual        = "=="
	wordSep        = " "
	statusPass     = "pass"
	statusFail     = "fail"
)

// canon explain's text (CLI.md §3.7): the head line, the origin columns and their details.
const (
	fmtExplainHead = "%s = %s"
	fmtFileLine    = "%s:%d"
	fmtFrame       = "  in %s (%s:%d)"
	fmtMoreFrames  = "  (%d more frames)"
	fmtInputFrom   = "input from env %s"
	labelSetBy     = "set by"
	labelLoaded    = "loaded"
	layerWord      = "layer "
	pointerMark    = "#"
	rowWord        = "row "

	vmRecord   = "record" // the view model's kinds of a type expression (VIEWMODEL.md §12.3)
	vmVariant  = "variant"
	vmOptional = "optional"

	pkgMark       = ':'
	fieldMark     = '.'
	lineBreakRune = '\n'
	escapeByte    = '\\'
	newlineLetter = 'n' // a line break written `\n` in a one-line detail
)

// canon refs' text (CLI.md §3.8): the count line and one line per reference.
const (
	fmtRefsHead  = "%s is referenced %d %s"
	fmtRefLine   = "%s:%d  %s  %s"
	fmtQualified = "%s:%s"
	wordTime     = "time"
	wordTimes    = "times"
)

// --watch (CLI.md §3.3, §3.4, IMPLEMENTATION-PLAN.md §8.1): the cycle header, the fixed line, the JSON `change` member, the events kept waiting.
const (
	fmtWatchHead   = "-- %d files changed, %d packages re-checked"
	fmtWatchFixed  = "fixed: %s[%s]"
	fmtWatchAt     = "%s:%d:%d"
	fmtWatchChange = ",\"change\":%q}"
	watchAdded     = "added"
	watchRemoved   = "removed"
	watchBlank     = ""
	watchUnsettled = "-- outputs did not settle; waiting for a change"
	watchQueue     = 16
)

// originLabels are the origins CLI.md §3.7 names otherwise than API.md's OriginKind.
var originLabels = map[canon.OriginKind]string{
	canon.OriginLayer: labelSetBy, canon.OriginJSON: labelLoaded, canon.OriginCSV: labelLoaded,
	canon.OriginDefines: labelLoaded, canon.OriginText: labelLoaded,
}

// originDetails writes each origin's detail column (CLI.md §3.7).
var originDetails = map[canon.OriginKind]func(*explainer, canon.Origin) string{
	canon.OriginLayer: layerDetail, canon.OriginLiteral: noDetail, canon.OriginJSON: pointerDetail,
	canon.OriginCSV: rowDetail, canon.OriginDefines: noDetail, canon.OriginText: noDetail,
	canon.OriginDefault: textDetail, canon.OriginSpread: sourceDetail, canon.OriginComputed: sourceDetail,
}

// Messages of the CLI itself: usage errors and failures that are not findings.
const (
	progName       = "canon"
	msgPrefix      = progName + ": "
	fmtArgs        = "%s: %w"
	fmtQuoted      = "%q: %w"
	fmtSelector    = "%w: %s"
	fmtSelectorWhy = "%w: %s: %w"
	fmtWrap        = "%w"
	msgInterrupted = "interrupted"
	msgReportBug   = "this is a bug in canon; please report it at https://github.com/fantasim/canonlang/issues"
)

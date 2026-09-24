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
)

// usageText lists the commands; the flags follow it (CLI.md §1).
const usageText = `usage: canon <command> [arguments] [flags]

commands:
  build [packages...]   check, then write the outputs of packages
  check [packages...]   parse and check packages, print findings
  init                  create project.canon in the current directory
  new <package>         create a package directory with a first file
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

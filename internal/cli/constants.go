package cli

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
	cmdInit    = "init"
	cmdNew     = "new"
	cmdVersion = "version"
)

// Flags (CLI.md §2.3, §3.1).
const (
	flagProject     = "project"
	flagRoot        = "root"
	flagFormat      = "format"
	flagQuiet       = "quiet"
	flagQuietShort  = "q"
	flagMaxWarnings = "max-warnings"
	flagName        = "name"
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
)

// usageText lists the commands; the flags follow it (CLI.md §1).
const usageText = `usage: canon <command> [arguments] [flags]

commands:
  check [packages...]   parse and check packages, print findings
  init                  create project.canon in the current directory
  new <package>         create a package directory with a first file
  version               print the compiler, language and format versions

flags:
`

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

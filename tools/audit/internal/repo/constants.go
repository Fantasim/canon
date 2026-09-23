package repo

const (
	AuditDir      = ".sovaudit"
	StateFile     = AuditDir + "/state.tsv"
	BaselineFile  = AuditDir + "/baseline.tsv"
	ImportsFile   = AuditDir + "/imports.tsv"
	RootAllowFile = AuditDir + "/root-allow.txt"

	// GoExt is the Go source extension, owned here alongside go.mod.
	GoExt = ".go"
	// DefaultBase is the git revision the ratchet guards when --base is not given.
	DefaultBase = "HEAD"
	// MarkdownExt is the extension the project lane scans for links.
	MarkdownExt = ".md"
	// DecisionsFile is the project's decision record, which decision-dropped watches.
	DecisionsFile = "DECISIONS.md"
)

// skipPrefixes are path segments never audited: vendored, generated or agent-local trees.
var skipPrefixes = []string{"vendor/", "node_modules/", "testdata/", ".claude/worktrees/", "dist/", "build/"}

// Example trees never audited: fixtures, and every expected/ tree of generated goldens.
const (
	examplesPrefix  = "examples/"
	fixturesPrefix  = examplesPrefix + "_fixtures/"
	expectedSegment = "/expected/"
)

const filePerm = 0o644

const (
	goModFile   = "go.mod"
	goModSuffix = "/" + goModFile

	// GitNameOnly is git's --name-only flag, shared with the integrity lane's git calls.
	GitNameOnly = "--name-only"

	gitLsFiles         = "ls-files"
	gitExcludeStandard = "--exclude-standard"
	moduleDirective    = "module "
)

const stateHeader = `# canon audit rule states for this repo: <rule>\t<enforce|ratchet|observe|off>.
# Absent = the rulebook default. Loosening a row is a maintainer's call (baseline-guard).
`

package integrity

import (
	"regexp"

	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

const laneName = "integrity"

const (
	ruleBaselineGuard = "baseline-guard"
	ruleDecision      = "decision-dropped"
	ruleReason        = "ignore-reason"
	ruleCount         = "ignore-count"
	ruleState         = "rule-state"
	repoScope         = "."

	gitShow       = "show"
	gitLog        = "log"
	gitHashFormat = "--format=%H"
	gitNoFormat   = "--format="
	gitRange      = ".."
	gitPathSep    = "--"
	// gitRevPathHere joins a revision to a path relative to git -C's directory.
	gitRevPathHere = ":./"
	gitLsTree      = "ls-tree"
	gitRevParse    = "rev-parse"
	// gitInsideWorkTree prints gitTrue in a work tree; outside one git fails (or prints false).
	gitInsideWorkTree = "--is-inside-work-tree"
	gitTrue           = "true"
	// fmtGitErr names the git subcommand and revision a failure came from.
	fmtGitErr = "git %s %s: %w"

	skipPrefix   = laneName + ": "
	skipGuard    = skipPrefix + ruleBaselineGuard
	skipDecision = skipPrefix + ruleDecision
	skipState    = skipPrefix + ruleState
	skipIgnores  = skipPrefix + ruleReason

	auditSegment = repo.AuditDir + "/"

	msgBaselineUp  = "baseline entry added or raised against the base revision: "
	msgDecision    = "a removed comment recorded a decision and " + repo.DecisionsFile + " did not change: "
	msgStateLoose  = "state loosened against the base revision: "
	msgThreshold   = "threshold raised against the base revision: %s %d -> %d"
	fixThreshold   = "restore the base value; raising a limit is a maintainer's own commit"
	msgUnknownRule = "unknown rule id: "
	msgBadMode     = "invalid mode: "
	msgNoReason    = "ignore without a rule id and a reason"
	msgUnknownIgn  = "ignore names an unknown rule: "
)

var ruleIDs = []string{ruleBaselineGuard, ruleDecision, ruleReason, ruleCount, ruleState}

// sourceExts are the files whose comments hold ignores and decisions: Go, and the HTML
// comments of Markdown.
var sourceExts = []string{repo.GoExt, repo.MarkdownExt}

const docLineMark = "*"

var commentOpeners = []string{"//", "/*", "<!--"}

// ignoreMarkers are every other suppression syntax the pinned Go tools honour (golangci-lint,
// staticcheck, gosec, exhaustive, revive); ignore-count sums them with this audit's own.
var ignoreMarkers = []string{
	"//nolint", "//lint:ignore", "//lint:file-ignore", "#nosec", "//gosec:disable",
	"//exhaustive:ignore", "//revive:disable",
}

const (
	gitDiff          = "diff"
	gitUnifiedZero   = "--unified=0"
	gitNoColor       = "--no-color"
	gitRelative      = "--relative"
	gitStatus        = "status"
	gitPorcelain     = "--porcelain"
	gitMessageFormat = "--format=%B"
	diffFilePrefix   = "+++ b/"
	diffHunkPrefix   = "@@"
	diffRemoved      = "-"
	diffAdded        = "+"
	diffOldHeader    = "---"
	// decisionObsolete in a commit message since the base says the dropped decisions are void.
	decisionObsolete = "sovaudit:decision-obsolete"
	// decisionContextWords is how much of a decision's wording must come back to count as kept.
	decisionContextWords = 4
)

// reDecision is the wording of a comment that records a decision, not an explanation.
var reDecision = regexp.MustCompile(`(?i)\b((was|were|and) rejected|considered and|trade-?off|the trade\b|judg(e)?ment call|not an oversight|an interpretation|(louis|maintainer)['’-]?s? ?(call|decision|ruling|flagged)|we (chose|decided))`)

// reHunkOld is a hunk header's old-side start line.
var reHunkOld = regexp.MustCompile(`^@@ -(\d+)`)

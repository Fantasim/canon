package lane

import (
	"regexp"

	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

// reDirective is a comment opening on the marker, after code or alone on its line.
var reDirective = regexp.MustCompile(`(?:^|//|/\*|<!--|^\s*\*)\s*sovaudit:ignore(-file)?\s+(.*)$`)

// reMdDirective is the same in Markdown: an HTML comment opening its line, so prose that
// quotes the Go syntax is not a directive.
var reMdDirective = regexp.MustCompile(`^\s*<!--\s*sovaudit:ignore(-file)?\s+(.*)$`)

// reGoDirective is the same over Go comment text, where the comment itself must open on it.
var reGoDirective = regexp.MustCompile(`^\s*(?://|/\*)?\s*sovaudit:ignore(-file)?\s+(.*)$`)

const (
	reasonSep = "--"
	htmlClose = "-->"
	scanBuf   = 1 << 20

	directiveWholeGroup = 1
	directiveSpecGroup  = 2
)

const (
	SelfMemLimit = 1 << 30
	lockName     = "canon-audit.lock"
	lockPerm     = 0o600
)

// LogPrefix opens every diagnostic line lane and main write to stderr.
const LogPrefix = rules.ToolName + ": "

var toolLimits = []string{"GOMEMLIMIT=1GiB", "GOMAXPROCS=2", "GOFLAGS=-p=2"}

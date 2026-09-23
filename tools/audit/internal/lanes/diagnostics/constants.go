package diagnostics

import (
	"regexp"

	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

const laneName = "diagnostics"

const (
	ruleInline     = rules.IDDiagMessageInline
	ruleUntested   = rules.IDDiagCodeUntested
	ruleUnreported = rules.IDDiagCodeUnreported
)

var ruleIDs = []string{ruleInline, ruleUntested, ruleUnreported}

// Where the catalogue, the registry, the per-code tests and the runtime helper texts live.
const (
	catalogueFile   = "spec/ERRORS.md"
	diagDir         = "internal/diag"
	internalDir     = "internal/"
	genPackage      = "gen"
	anyGenerator    = "gen/*"
	findingsDir     = "/testdata/findings"
	runtimeTexts    = "internal/gen/*/runtime/*.txt"
	findingsSection = "-- findings.txt --"
	txtarGlob       = "_*.txtar"
	dotImport       = "."
	allMatches      = -1
	pathSep         = "/"
	lineBreak       = "\n"
)

// The header rows of the catalogue's tables whose rows this lane reads.
const (
	headerCodes    = "| Code | Severity | Package | Owner | Meaning |"
	headerMessages = "| Code | Variant | Args | Template |"
	headerRuntime  = "| Code | Text |"
	tableRow       = "|"
	tableRule      = "|---"
	codeSpan       = "`"
)

// Cell indexes: every table starts with the code; the codes table names the package third,
// a messages row holds the template fourth, a runtime row its text second.
const (
	colCode     = 0
	colText     = 1
	colPackage  = 2
	colTemplate = 3
	codeGroup   = 1
)

// Template grammar: escapes are literal text, a placeholder or a line break ends a segment.
const (
	escBraceOpen  = "{{"
	escBraceClose = "}}"
	escBackslash  = `\\`
	escLineBreak  = `\n`
	braceOpen     = '{'
	braceClose    = "}"
	backslash     = `\`
	segmentTrim   = " \t\"'.,:;"
)

var escapes = []struct{ seq, text string }{
	{escBraceOpen, string(braceOpen)}, {escBraceClose, braceClose}, {escBackslash, backslash},
}

var (
	reCode        = regexp.MustCompile(`^[EW][0-9]{4}$`)
	reCodeToken   = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])([EW][0-9]{4})(?:[^A-Za-z0-9_]|$)`)
	reTxtarName   = regexp.MustCompile(`^([EW][0-9]{4})_[1-9][0-9]*\.txtar$`)
	reTxtarMarker = regexp.MustCompile(`^-- .+ --$`)
)

// What diag-message-inline reports, as finding details and messages.
const (
	detailCode      = "code literal"
	detailText      = "catalogued text"
	detailFormatted = "formatted argument"
	detailPhrase    = "English argument"
	detailBuilt     = "hand-built "
	detailMessage   = "Message assignment"

	msgCode      = "string literal holds the code %s: only internal/diag names a code"
	msgText      = "string literal holds catalogued message text of %s: %q"
	msgFormatted = "fmt.%s formats an argument of a diag call: the message is a template of " + catalogueFile
	msgPhrase    = "string literal %q in a diag call: English is a template variant or a Kind, never an argument"
	msgBuilt     = "diag.%s built by hand outside " + diagDir
	msgMessage   = "diag field %s assigned outside " + diagDir

	msgUntested   = "%s is reported here, and no %s/%s_<n>.txtar holds it in its findings.txt"
	msgUnreported = "%s (package %s) is reported by no compiler code yet"
)

// handBuilt are the diag types only diag constructs; messageField is the rendered text.
var handBuilt = []string{"Finding", "Related", "Def", "Variant", "Arg"}

const messageField = "Message"

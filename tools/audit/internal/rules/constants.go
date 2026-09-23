package rules

import "regexp"

const (
	Enforce Mode = "enforce"
	Ratchet Mode = "ratchet"
	Observe Mode = "observe"
	Off     Mode = "off"
)

// Strictness levels, in ascending order: a state change is a loosening when it lowers this.
const (
	strictnessOff = iota
	strictnessObserve
	strictnessRatchet
	strictnessEnforce
)

const (
	famSize      = "size"
	famMagic     = "magic"
	famErrors    = "errors"
	famDiag      = "diag"
	famAPI       = "api"
	famDead      = "dead"
	famDup       = "dup"
	famComments  = "comments"
	famIdiom     = "idiom"
	famProject   = "project"
	famIntegrity = "integrity"
)

// Mechanism marks a rule that is a property of the audit itself and emits no finding.
const Mechanism = "mechanism"

// historyPattern is comment-history's one definition: only
// wording that can only narrate a change ("legacy" and "no longer" describe the present).
const historyPattern = `(?i)\b(previously|formerly|we changed|fixed in (v\d|commit)|as of v\d|since v\d)\b|` +
	`\b(this|the) (func|function|file|field|const|type|package|test|method) was (renamed|moved|removed)\b|` +
	`\b(before|after) the (fix|refactor|rewrite)\b`

// presentPrevious is "previously" as UI or cache state ("previously seen"), not history.
const presentPrevious = `(?i)\bpreviously[- ](expanded|edited|added|cached|watched|seen)\b`

// usedTo finds "used to" with the word before it: history unless that word makes it the
// passive "is used to <verb>" (purpose), or the phrase opens a sentence ("Used to resolve").
var usedTo = regexp.MustCompile(`(\S*)\s+used to\b|^\s*[Uu]sed to\b`)

const wordPunct = "\"'`([{,;:"

var passiveBefore = map[string]bool{
	"is": true, "are": true, "be": true, "been": true, "being": true,
	"was": true, "were": true, "get": true, "gets": true, "got": true, "and": true, "or": true,
}

// toolCustom marks a rule implemented by the audit itself.
const toolCustom = "custom"

// ToolName is how every report, log line and usage text names this tool.
const ToolName = "canon audit"

// Rule ids more than one package names, written once so a rename touches one place.
const (
	IDCommentDecl         = "comment-decl"
	IDCommentFileHeader   = "comment-file-header"
	IDCommentBlock        = "comment-block"
	IDCommentRatio        = "comment-ratio"
	IDCommentADRNarration = "comment-adr-narration"
	IDCommentHistory      = "comment-history"
	IDTodoInCode          = "todo-in-code"
	IDDiagMessageInline   = "diag-message-inline"
	IDDiagCodeUntested    = "diag-code-untested"
	IDDiagCodeUnreported  = "diag-code-unreported"
)

// Tool labels this table repeats across rules.
const (
	toolStaticcheck = "staticcheck"
	// ToolDeadcode is shared with the stock lane's own skip label for the same tool.
	ToolDeadcode = "deadcode"
)

// Fix text this table repeats across otherwise unrelated rules.
const (
	fixDeleteIt       = "delete it"
	fixDeleteGitHasIt = "delete it; git has it"
	fixFlaggedPattern = "fix the flagged pattern"
)

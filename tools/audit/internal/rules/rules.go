package rules

import (
	"sort"

	"github.com/fantasim/canonlang/tools/audit/internal/threshold"
)

type Mode string

type Rule struct {
	ID      string
	Family  string
	Mode    Mode
	Summary string
	Fix     string
	Tool    string
}

// All is the rulebook, in print order. An id never changes: baselines are keyed by it.
var All = []Rule{
	{
		ID: "fn-length", Family: famSize, Mode: Ratchet, Tool: toolCustom,
		Summary: "function body over {fn-lines} lines or {fn-statements} statements (tests: {test-fn-lines} lines)", Fix: "split it into named steps",
	},
	{
		ID: "fn-params", Family: famSize, Mode: Ratchet, Tool: toolCustom,
		Summary: "more than {fn-params} parameters", Fix: "group them into an options struct",
	},
	{
		ID: "fn-results", Family: famSize, Mode: Ratchet, Tool: toolCustom,
		Summary: "more than {fn-results} return values", Fix: "return a struct",
	},
	{
		ID: "fn-complexity", Family: famSize, Mode: Ratchet, Tool: "gocognit",
		Summary: "cognitive complexity over {fn-complexity}", Fix: "extract branches into functions, return early",
	},
	{
		ID: "fn-nesting", Family: famSize, Mode: Ratchet, Tool: toolCustom,
		Summary: "control flow nested deeper than {fn-nesting} levels", Fix: "invert conditions and return early",
	},
	{
		ID: "naked-return", Family: famSize, Mode: Ratchet, Tool: toolCustom,
		Summary: "bare return in a function longer than {naked-return-lines} lines", Fix: "return the values explicitly",
	},
	{
		ID: "file-length", Family: famSize, Mode: Ratchet, Tool: toolCustom,
		Summary: "file over {file-lines} lines", Fix: "split the file by concern",
	},
	{
		ID: "pkg-size", Family: famSize, Mode: Observe, Tool: toolCustom,
		Summary: "package over {pkg-files} files or {pkg-lines} lines", Fix: "split the package",
	},
	{
		ID: "magic-string", Family: famMagic, Mode: Ratchet, Tool: toolCustom,
		Summary: "string literal used {magic-string-uses}+ times", Fix: "name it in <pkg>/constants.go (in the lowest package both import when shared)",
	},
	{
		ID: "magic-number", Family: famMagic, Mode: Ratchet, Tool: toolCustom,
		Summary: "number other than 0 and 1 outside a const declaration", Fix: "name it in <pkg>/constants.go",
	},
	{
		ID: "const-placement", Family: famMagic, Mode: Ratchet, Tool: toolCustom,
		Summary: "const, or package-level literal var, outside <pkg>/constants.go or errors.go", Fix: "move it to <pkg>/constants.go",
	},
	{
		ID: "const-dup", Family: famMagic, Mode: Ratchet, Tool: toolCustom,
		Summary: "one string value declared under two constant names", Fix: "keep one constant",
	},
	{
		ID: "env-key", Family: famMagic, Mode: Ratchet, Tool: toolCustom,
		Summary: "environment read outside cmd/ or a config package, or with a literal key", Fix: "read it once in cmd/ and pass it down, key as a constant",
	},
	{
		ID: "err-inline", Family: famErrors, Mode: Ratchet, Tool: toolCustom,
		Summary: "errors.New inside a function, or fmt.Errorf without %w", Fix: "declare a sentinel in <pkg>/errors.go and wrap it",
	},
	{
		ID: "err-placement", Family: famErrors, Mode: Ratchet, Tool: toolCustom,
		Summary: "error sentinel declared outside <pkg>/errors.go", Fix: "move it to <pkg>/errors.go",
	},
	{
		ID: "err-compare", Family: famErrors, Mode: Ratchet, Tool: "errorlint",
		Summary: "== or type assertion on an error", Fix: "use errors.Is / errors.As",
	},
	{
		ID: "err-unchecked", Family: famErrors, Mode: Ratchet, Tool: "errcheck",
		Summary: "error result not checked", Fix: "handle or return it",
	},
	{
		ID: "err-unwrapped", Family: famErrors, Mode: Ratchet, Tool: "wrapcheck",
		Summary: "error from another module returned unwrapped, or formatted with %v", Fix: "wrap with fmt.Errorf(\"...: %w\", err)",
	},
	{
		ID: "nil-err", Family: famErrors, Mode: Ratchet, Tool: "nilerr",
		Summary: "returns nil inside an err != nil branch", Fix: "return the error",
	},
	{
		ID: "err-style", Family: famErrors, Mode: Enforce, Tool: "staticcheck ST1005",
		Summary: "error string capitalised or ending in punctuation", Fix: "lower-case it, drop the punctuation",
	},
	{
		ID: IDDiagMessageInline, Family: famDiag, Mode: Enforce, Tool: toolCustom,
		Summary: "diagnostic code or message text outside internal/diag: a code or catalogued text in a string, English or fmt in a diag call, a hand-built Finding",
		Fix:     "report through the registry: diag.<CODE>.At...(span, args...).Report(bag)",
	},
	{
		ID: IDDiagCodeUntested, Family: famDiag, Mode: Ratchet, Tool: toolCustom,
		Summary: "catalogued code the compiler reports with no internal/<pkg>/testdata/findings/<CODE>_<n>.txtar producing it",
		Fix:     "add the per-code txtar case to the owning package",
	},
	{
		ID: IDDiagCodeUnreported, Family: famDiag, Mode: Observe, Tool: toolCustom,
		Summary: "catalogued code no compiler code reports yet (target zero at v0.1)",
		Fix:     "implement the trigger its owning document defines",
	},
	{
		ID: "exported-but-local", Family: famAPI, Mode: Ratchet, Tool: toolCustom,
		Summary: "exported identifier used only inside its own package", Fix: "unexport it",
	},
	{
		ID: "unused-param", Family: famAPI, Mode: Ratchet, Tool: "unparam",
		Summary: "parameter always unused or always the same value", Fix: "drop it",
	},
	{
		ID: "pkg-doc", Family: famAPI, Mode: Ratchet, Tool: toolCustom,
		Summary: "package without a doc.go", Fix: "add doc.go holding the package comment",
	},
	{
		ID: "pkg-example", Family: famAPI, Mode: Ratchet, Tool: toolCustom,
		Summary: "package other than main without an Example test", Fix: "add an Example function in a _test.go file",
	},
	{
		ID: "dead-unexported", Family: famDead, Mode: Ratchet, Tool: "unused",
		Summary: "unused unexported identifier", Fix: fixDeleteIt,
	},
	{
		ID: "dead-unreachable", Family: famDead, Mode: Ratchet, Tool: ToolDeadcode,
		Summary: "function no main or test can reach", Fix: fixDeleteIt,
	},
	{
		ID: "dead-file", Family: famDead, Mode: Ratchet, Tool: ToolDeadcode,
		Summary: "file whose every function is unreachable", Fix: "delete the file",
	},
	{
		ID: "commented-code", Family: famDead, Mode: Ratchet, Tool: "gocritic",
		Summary: "commented-out code", Fix: fixDeleteGitHasIt,
	},
	{
		ID: "dead-assign", Family: famDead, Mode: Ratchet, Tool: "ineffassign, wastedassign",
		Summary: "value assigned and never read", Fix: "drop the assignment",
	},
	{
		ID: IDTodoInCode, Family: famDead, Mode: Ratchet, Tool: toolCustom,
		Summary: "TODO, FIXME, XXX or HACK in code", Fix: "track it in an issue and delete it",
	},
	{
		ID: "reimplements-stdlib", Family: famDup, Mode: Ratchet, Tool: toolCustom + ", modernize",
		Summary: "hand-written stdlib helper, or pre-modern Go (interface{}, int loops, loop var copies)", Fix: "use the standard library or current Go",
	},
	{
		ID: "dup-in-repo", Family: famDup, Mode: Ratchet, Tool: "dupl",
		Summary: "duplicated block of {dup-tokens}+ tokens in one repo", Fix: "extract one function",
	},
	{
		ID: IDCommentDecl, Family: famComments, Mode: Ratchet, Tool: toolCustom,
		Summary: "doc comment over {comment-decl-lines} lines on a declaration", Fix: "keep the one line a reader needs; a decision moves to DECISIONS.md first, history is in git",
	},
	{
		ID: IDCommentFileHeader, Family: famComments, Mode: Ratchet, Tool: toolCustom,
		Summary: "comment block above package/imports outside doc.go", Fix: "delete it, or one line in doc.go",
	},
	{
		ID: "comment-pkg", Family: famComments, Mode: Ratchet, Tool: toolCustom,
		Summary: "package doc over {comment-pkg-lines} lines", Fix: "cut it to what the package is",
	},
	{
		ID: IDCommentBlock, Family: famComments, Mode: Ratchet, Tool: toolCustom,
		Summary: "more than {comment-block-lines} consecutive comment lines inside a body", Fix: "make the code say it, or one line; a decision moves to DECISIONS.md first",
	},
	{
		ID: IDCommentRatio, Family: famComments, Mode: Ratchet, Tool: toolCustom,
		Summary: "comments over {comment-ratio-percent}% of a file's non-blank lines (files of {comment-ratio-min-lines}+ non-blank lines)", Fix: "cut comments",
	},
	{
		ID: "comment-field", Family: famComments, Mode: Ratchet, Tool: toolCustom,
		Summary: "comment over {comment-field-lines} line on a field or constant", Fix: "a better name, or one line",
	},
	{
		ID: IDCommentADRNarration, Family: famComments, Mode: Ratchet, Tool: toolCustom,
		Summary: "comment citing a SPEC §, decision, ADR or DOCTRINE that narrates it", Fix: "keep the bare reference: // SPEC §19, // decision 25",
	},
	{
		ID: IDCommentHistory, Family: famComments, Mode: Observe, Tool: toolCustom,
		Summary: "comment narrating what changed", Fix: "delete it; git has the history",
	},
	{
		ID: "fmt", Family: famIdiom, Mode: Enforce, Tool: "gofumpt, goimports",
		Summary: "file not gofumpt/goimports formatted", Fix: "golangci-lint fmt",
	},
	{
		ID: "log-direct", Family: famIdiom, Mode: Ratchet, Tool: toolCustom,
		Summary: "log.* or fmt.Print* outside cmd/ and package main", Fix: "return it to the caller; only cmd/ prints",
	},
	{
		ID: "bare-goroutine", Family: famIdiom, Mode: Ratchet, Tool: toolCustom,
		Summary: "bare go statement: a panic in it kills the host program", Fix: "recover inside the goroutine and return the panic as an error",
	},
	{
		ID: "ctx-first", Family: famIdiom, Mode: Ratchet, Tool: "revive, contextcheck",
		Summary: "ctx not first (revive), or a call chain that drops the caller's ctx (contextcheck, one per final callee)", Fix: "take ctx first and pass it down",
	},
	{
		ID: "http-no-ctx", Family: famIdiom, Mode: Ratchet, Tool: "noctx",
		Summary: "outgoing HTTP request without a context", Fix: "http.NewRequestWithContext",
	},
	{
		ID: "resource-close", Family: famIdiom, Mode: Ratchet, Tool: "sqlclosecheck, rowserrcheck, bodyclose",
		Summary: "rows, statement or body not closed, or rows.Err unchecked", Fix: "defer Close, check rows.Err",
	},
	{
		ID: "switch-exhaustive", Family: famIdiom, Mode: Ratchet, Tool: "exhaustive",
		Summary: "switch on an enum-like type missing a case", Fix: "add the case",
	},
	{
		ID: "naming", Family: famIdiom, Mode: Ratchet, Tool: "revive, staticcheck ST1003",
		Summary: "initialism, receiver or variable naming", Fix: "rename",
	},
	{
		ID: "import-boundary", Family: famIdiom, Mode: Ratchet, Tool: toolCustom,
		Summary: "import across a boundary declared in .sovaudit/imports.tsv", Fix: "go through the allowed layer",
	},
	{
		ID: "security", Family: famIdiom, Mode: Ratchet, Tool: "gosec",
		Summary: "gosec finding", Fix: fixFlaggedPattern,
	},
	{
		ID: "staticcheck", Family: famIdiom, Mode: Ratchet, Tool: toolStaticcheck,
		Summary: "staticcheck finding", Fix: fixFlaggedPattern,
	},
	{
		ID: "dead-link", Family: famProject, Mode: Ratchet, Tool: toolCustom,
		Summary: "Markdown link to a path that does not exist", Fix: "fix or drop the link",
	},
	{
		ID: "root-clutter", Family: famProject, Mode: Ratchet, Tool: toolCustom,
		Summary: "stray file or directory at the repo root", Fix: "move it, delete it, or list it in .sovaudit/root-allow.txt",
	},
	{
		ID: "ratchet", Family: famIntegrity, Mode: Enforce, Tool: Mechanism,
		Summary: "check fails on a finding that is new or grew against .sovaudit/baseline.tsv", Fix: "",
	},
	{
		ID: "baseline-guard", Family: famIntegrity, Mode: Enforce, Tool: toolCustom,
		Summary: "baseline, state or thresholds loosened against HEAD", Fix: "revert it; only `baseline --tighten` writes the baseline",
	},
	{
		ID: "decision-dropped", Family: famIntegrity, Mode: Enforce, Tool: toolCustom,
		Summary: "a removed comment recorded a decision and DECISIONS.md did not change",
		Fix:     "record it in DECISIONS.md, or say `sovaudit:decision-obsolete` in the commit",
	},
	{
		ID: "ignore-reason", Family: famIntegrity, Mode: Enforce, Tool: toolCustom + ", nolintlint",
		Summary: "ignore directive without a rule and a reason, or one that ignores nothing", Fix: "sovaudit:ignore <rule> -- <reason>; delete a stale one",
	},
	{
		ID: "ignore-count", Family: famIntegrity, Mode: Ratchet, Tool: toolCustom,
		Summary: "ignore directives in the repo: sovaudit, nolint, lint, nosec, gosec, exhaustive, revive", Fix: "fix the code instead of ignoring it",
	},
	{
		ID: "rule-state", Family: famIntegrity, Mode: Enforce, Tool: toolCustom,
		Summary: "unknown rule id or mode in .sovaudit/state.tsv", Fix: "fix the row",
	},
	{
		ID: "census-complete", Family: famIntegrity, Mode: Enforce, Tool: Mechanism,
		Summary: "audit prints every rule with its count, zeros included", Fix: "",
	},
}

var byID = func() map[string]*Rule {
	m := make(map[string]*Rule, len(All))
	for i := range All {
		m[All[i].ID] = &All[i]
	}
	return m
}()

// Describe is the rule's summary with every `{key}` replaced by its threshold; a summary
// naming an unknown key (TestEverySummaryExpands) stays as written.
func (rl Rule) Describe(limits threshold.Set) string {
	text, err := limits.Expand(rl.Summary)
	if err != nil {
		return rl.Summary
	}
	return text
}

func Lookup(id string) (*Rule, bool) {
	rl, ok := byID[id]
	return rl, ok
}

func ValidMode(m Mode) bool {
	return m == Enforce || m == Ratchet || m == Observe || m == Off
}

// Strictness orders modes so a state change can be classified as loosening.
func Strictness(m Mode) int {
	return map[Mode]int{Off: strictnessOff, Observe: strictnessObserve, Ratchet: strictnessRatchet, Enforce: strictnessEnforce}[m]
}

func IDs() []string {
	out := make([]string, len(All))
	for i, rl := range All {
		out[i] = rl.ID
	}
	sort.Strings(out)
	return out
}

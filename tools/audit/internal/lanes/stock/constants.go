package stock

import (
	"regexp"

	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

const laneName = "stock"

// Rule ids this lane produces (see internal/rules/rules.go for their definitions).
const (
	ruleFnComplexity     = "fn-complexity"
	ruleErrCompare       = "err-compare"
	ruleErrUnchecked     = "err-unchecked"
	ruleErrUnwrapped     = "err-unwrapped"
	ruleNilErr           = "nil-err"
	ruleErrStyle         = "err-style"
	ruleUnusedParam      = "unused-param"
	ruleDeadUnexported   = "dead-unexported"
	ruleDeadUnreachable  = "dead-unreachable"
	ruleDeadFile         = "dead-file"
	ruleCommentedCode    = "commented-code"
	ruleDeadAssign       = "dead-assign"
	ruleReimplStdlib     = "reimplements-stdlib"
	ruleDupInRepo        = "dup-in-repo"
	ruleFmt              = "fmt"
	ruleCtxFirst         = "ctx-first"
	ruleHTTPNoCtx        = "http-no-ctx"
	ruleResourceClose    = "resource-close"
	ruleSwitchExhaustive = "switch-exhaustive"
	ruleNaming           = "naming"
	ruleSecurity         = "security"
	ruleStaticcheck      = "staticcheck"
	ruleIgnoreReason     = "ignore-reason"
)

var ruleIDs = []string{
	ruleFnComplexity, ruleErrCompare, ruleErrUnchecked, ruleErrUnwrapped,
	ruleNilErr, ruleErrStyle, ruleUnusedParam, ruleDeadUnexported, ruleDeadUnreachable,
	ruleDeadFile, ruleCommentedCode, ruleDeadAssign, ruleReimplStdlib, ruleDupInRepo, ruleFmt,
	ruleCtxFirst, ruleHTTPNoCtx, ruleResourceClose, ruleSwitchExhaustive, ruleNaming,
	ruleSecurity, ruleStaticcheck, ruleIgnoreReason,
}

// golangci-lint linter names this lane enables.
const (
	linterGocognit      = "gocognit"
	linterGosec         = "gosec"
	linterErrorlint     = "errorlint"
	linterErrcheck      = "errcheck"
	linterWrapcheck     = "wrapcheck"
	linterNilerr        = "nilerr"
	linterStaticcheck   = "staticcheck"
	linterUnparam       = "unparam"
	linterUnused        = "unused"
	linterGocritic      = "gocritic"
	linterIneffassign   = "ineffassign"
	linterWastedassign  = "wastedassign"
	linterModernize     = "modernize"
	linterDupl          = "dupl"
	linterRevive        = "revive"
	linterContextcheck  = "contextcheck"
	linterNoctx         = "noctx"
	linterSqlclosecheck = "sqlclosecheck"
	linterRowserrcheck  = "rowserrcheck"
	linterBodyclose     = "bodyclose"
	linterExhaustive    = "exhaustive"
	linterNolintlint    = "nolintlint"
	linterGofumpt       = "gofumpt"
	linterGoimports     = "goimports"
)

// allLinters mirrors toolchain/golangci.yml's linters.enable list.
var allLinters = []string{
	linterGocognit, linterGosec, linterErrorlint, linterErrcheck, linterWrapcheck, linterNilerr,
	linterStaticcheck, linterUnparam, linterUnused, linterGocritic, linterIneffassign,
	linterWastedassign, linterModernize, linterDupl, linterRevive, linterContextcheck,
	linterNoctx, linterSqlclosecheck, linterRowserrcheck, linterBodyclose, linterExhaustive,
	linterNolintlint,
}

// ruleLinters lists, for every golangci-backed rule but fmt, the linter(s) that feed it.
var ruleLinters = map[string][]string{
	ruleFnComplexity:     {linterGocognit},
	ruleSecurity:         {linterGosec},
	ruleErrCompare:       {linterErrorlint},
	ruleErrUnchecked:     {linterErrcheck},
	ruleErrUnwrapped:     {linterWrapcheck, linterErrorlint},
	ruleNilErr:           {linterNilerr},
	ruleErrStyle:         {linterStaticcheck},
	ruleNaming:           {linterStaticcheck, linterRevive},
	ruleStaticcheck:      {linterStaticcheck},
	ruleUnusedParam:      {linterUnparam},
	ruleDeadUnexported:   {linterUnused},
	ruleCommentedCode:    {linterGocritic},
	ruleDeadAssign:       {linterIneffassign, linterWastedassign},
	ruleReimplStdlib:     {linterModernize},
	ruleDupInRepo:        {linterDupl},
	ruleCtxFirst:         {linterRevive, linterContextcheck},
	ruleHTTPNoCtx:        {linterNoctx},
	ruleResourceClose:    {linterSqlclosecheck, linterRowserrcheck, linterBodyclose},
	ruleSwitchExhaustive: {linterExhaustive},
	ruleIgnoreReason:     {linterNolintlint},
}

// directRuleByLinter is the rule for a linter whose whole output means one thing, no text
// parsing needed. gosec, staticcheck, revive and the formatters are mapped in mapping.go.
var directRuleByLinter = map[string]string{
	linterGocognit:      ruleFnComplexity,
	linterErrcheck:      ruleErrUnchecked,
	linterWrapcheck:     ruleErrUnwrapped,
	linterNilerr:        ruleNilErr,
	linterUnparam:       ruleUnusedParam,
	linterUnused:        ruleDeadUnexported,
	linterGocritic:      ruleCommentedCode,
	linterIneffassign:   ruleDeadAssign,
	linterWastedassign:  ruleDeadAssign,
	linterModernize:     ruleReimplStdlib,
	linterDupl:          ruleDupInRepo,
	linterContextcheck:  ruleCtxFirst,
	linterNoctx:         ruleHTTPNoCtx,
	linterSqlclosecheck: ruleResourceClose,
	linterRowserrcheck:  ruleResourceClose,
	linterBodyclose:     ruleResourceClose,
	linterExhaustive:    ruleSwitchExhaustive,
	linterNolintlint:    ruleIgnoreReason,
}

// A whole-function contains helper is gorules' finding; modernize's twin is dropped.
const (
	modernizeContains = "slicescontains"
	helperBodyLen     = 2
)

// contextcheck's chain text, "Function `a->b` should pass the context parameter", and the
// names filters resolve syntactically.
const (
	ctxChainOpen  = "Function `"
	ctxChainClose = "`"
	ctxChainSep   = "->"
	ctxPassMarker = "should pass the context parameter"
	closureMark   = "$"
	msgCtxPass    = ctxChainOpen + "%s" + ctxChainClose + " " + ctxPassMarker
	pkgNetHTTP    = gosrc.PkgNetHTTP
	pkgSQL        = "database/sql"
	typeRequest   = "Request"
	typeRows      = "Rows"
	typeError     = "error"
	scanResults   = 2
)

// errorlint's errorf check ("non-wrapping format verb") is about wrapping, not comparing.
const errorlintVerbMarker = "format verb"

// Messages and fixes stock rewrites because the tool's own text does not say what to do.
const (
	msgGofumpt     = "file not gofumpt-formatted"
	msgGoimports   = "imports not goimports-formatted"
	nolintUnused   = "is unused"
	fixStaleNolint = "delete the stale //nolint directive"
	fixGolangciFmt = "golangci-lint fmt (gofumpt + goimports)"
)

// Sub-codes that split one linter's output across more than one rule.
const (
	staticcheckErrStyle   = "ST1005"
	staticcheckNaming     = "ST1003"
	reviveContextArgument = "context-as-argument"
	reviveVarNaming       = "var-naming"
	reviveReceiverNaming  = "receiver-naming"
)

const (
	goCmd          = "go"
	toolGolangci   = "golangci-lint"
	toolDeadcode   = rules.ToolDeadcode
	golangciConfig = "golangci.yml"
	// renderedConfigPattern names the temporary config golangci-lint actually reads.
	renderedConfigPattern = "canon-audit-golangci-*.yml"
	targetAll             = "./..."
	detailSep             = ": "

	skipGolangci = "stock: golangci-lint"
	skipDeadcode = "stock: deadcode"

	argRun              = "run"
	argConcurrency      = "--concurrency=2"
	argConfig           = "--config"
	argOutputJSONPath   = "--output.json.path"
	argShowStatsFalse   = "--show-stats=false"
	argIssuesExitCodeOK = "--issues-exit-code=0"
	argDisable          = "--disable"
	valStdout           = "stdout"
	argDeadcodeTest     = "-test"
	unreachableFuncPfx  = "unreachable func: "
	deadFileDetail      = "deadcode: file contains only unreachable functions"
)

var (
	reCognitive       = regexp.MustCompile(`cognitive complexity (\d+)`)
	reUnreachableFunc = regexp.MustCompile(`^(.+\.go):(\d+):\d+: unreachable func: (.+)$`)
)

// reUnreachableFunc's own submatch indexes: 1 is the file, 2 the line, 3 the func name.
const (
	unreachableLineGroup = 2
	unreachableNameGroup = 3
)

// cognitiveGroups is len(reCognitive submatches): the whole match plus one capture group.
const cognitiveGroups = 2

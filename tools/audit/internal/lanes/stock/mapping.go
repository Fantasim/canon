package stock

import (
	"strconv"
	"strings"
)

// mapIssueRule finds the rule a golangci-lint issue belongs to. Most linters mean
// one rule; gosec, staticcheck, revive and the formatters split by the issue's own text.
func mapIssueRule(linter, text string) (string, bool) {
	switch linter {
	case linterGosec:
		return ruleSecurity, true
	case linterErrorlint:
		return mapErrorlint(text), true
	case linterStaticcheck:
		return mapStaticcheck(text), true
	case linterRevive:
		return mapRevive(text)
	case linterGofumpt, linterGoimports:
		return ruleFmt, true
	default:
		rule, ok := directRuleByLinter[linter]
		return rule, ok
	}
}

func mapErrorlint(text string) string {
	if strings.Contains(text, errorlintVerbMarker) {
		return ruleErrUnwrapped
	}
	return ruleErrCompare
}

// describeIssue rewrites the text of issues whose own wording does not say what to do.
func describeIssue(linter, text string) (msg, fix string) {
	switch {
	case linter == linterGofumpt:
		return msgGofumpt, fixGolangciFmt
	case linter == linterGoimports:
		return msgGoimports, fixGolangciFmt
	case linter == linterNolintlint && strings.Contains(text, nolintUnused):
		return text, fixStaleNolint
	}
	return text, ""
}

func mapStaticcheck(text string) string {
	switch codePrefix(text) {
	case staticcheckErrStyle:
		return ruleErrStyle
	case staticcheckNaming:
		return ruleNaming
	default:
		return ruleStaticcheck
	}
}

func mapRevive(text string) (string, bool) {
	switch codePrefix(text) {
	case reviveContextArgument:
		return ruleCtxFirst, true
	case reviveVarNaming, reviveReceiverNaming:
		return ruleNaming, true
	default:
		return "", false
	}
}

// codePrefix pulls the "CODE" out of a "CODE: message" issue text (gosec, staticcheck and
// revive all format their text this way).
func codePrefix(text string) string {
	code, _, ok := strings.Cut(text, ":")
	if !ok {
		return ""
	}
	return code
}

// valueFor is the measured number a finding carries, where the linter reports one.
func valueFor(linter string, iss golangciIssue) int {
	switch linter {
	case linterGocognit:
		return cognitiveValue(iss.Text)
	case linterDupl:
		return duplValue(iss.LineRange)
	case linterContextcheck:
		if len(ctxChain(iss.Text)) > 0 {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func cognitiveValue(text string) int {
	m := reCognitive.FindStringSubmatch(text)
	if len(m) < cognitiveGroups {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func duplValue(lr *lineRange) int {
	if lr == nil {
		return 0
	}
	return lr.To - lr.From + 1
}

package main

import "github.com/fantasim/canonlang/tools/audit/internal/rules"

const (
	cmdAudit    = "audit"
	cmdCheck    = "check"
	cmdBaseline = "baseline"
	cmdRules    = "rules"

	exitOK    = 0
	exitFail  = 1
	exitUsage = 2

	defaultPerRule = 5
	toolchainName  = "toolchain"

	// errPrefix opens every one-line diagnostic this command prints to stderr.
	errPrefix = rules.ToolName + ":"
)

const usage = `canon audit: the project's strict code audit (tools/audit)

  audit    [--rule id | --family f | --lane l] [--raw] [-n N]   census: every rule, zeros included
  check    [--changed [--base REV]]                             gate: new or grown findings fail (exit 1)
  baseline --init | --tighten                                   write / shrink .sovaudit/baseline.tsv
  rules    [--family f]                                         the rulebook

Run from tools/audit: go run . <command> --repo ../..
Common: --repo DIR (default .), --toolchain DIR, --thresholds FILE, --quiet.
Ignore one finding: // sovaudit:ignore <rule> -- <reason>   (a directive without a reason ignores nothing)
`

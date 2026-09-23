package main

import "os"

const (
	flagErrors  = "errors"
	flagPlan    = "plan"
	flagOut     = "out"
	flagRuntime = "runtime"

	defaultErrors = "spec/ERRORS.md"
	defaultPlan   = "spec/IMPLEMENTATION-PLAN.md"
	defaultOut    = "internal/diag"

	usageErrors  = "the diagnostic catalogue to read"
	usagePlan    = "the implementation plan holding the package table"
	usageOut     = "the directory the generated files are written to"
	usageRuntime = "a directory whose runtime helper texts are checked (optional)"

	exitOK    = 0
	exitFail  = 1
	exitUsage = 2

	filePerm os.FileMode = 0o600
	fmtFail              = "diaggen: %v\n"
)

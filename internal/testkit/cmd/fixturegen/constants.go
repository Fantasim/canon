package main

import (
	"os"
	"regexp"
)

const (
	flagReal    = "real"
	flagOut     = "fixtures"
	defaultReal = "testdata-real"
	defaultOut  = "examples/_fixtures"
	usageReal   = "the copy of the real roots, one directory per root"
	usageOut    = "the fixture tree to write"

	exitOK    = 0
	exitFail  = 1
	exitUsage = 2
	fmtFail   = "fixturegen: %v\n"

	filePerm os.FileMode = 0o600
	dirPerm  os.FileMode = 0o750

	// maxFixtureBytes is the size budget of the whole fixture tree (IMPLEMENTATION-PLAN.md §7.3).
	maxFixtureBytes = 300_000
)

// Manifest syntax: comment lines, then <path>\t<method>\t<space-separated names>.
const (
	commentPrefix  = "#"
	fieldSep       = "\t"
	manifestFields = 3
	fieldPath      = 0
	fieldMethod    = 1
	fieldNames     = 2
	lineBreak      = "\n"
	crlf           = "\r\n"
	methodDefines  = "defines"
	guardOpen      = "#ifndef"
	guardClose     = "#endif"
	defineWord     = "#define"
	guardFields    = 2
)

// methods are the extraction methods a manifest row may name.
var methods = map[string]func(src []byte, names []string) ([]byte, error){
	methodDefines: defines,
}

var reDefine = regexp.MustCompile(`^#define\s+([A-Za-z_][A-Za-z0-9_]*)`)

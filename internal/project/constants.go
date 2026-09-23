package project

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/syntax"
)

// DefaultLanguage is `languages` when project.canon does not declare it (GRAMMAR.md §7.1).
const DefaultLanguage = "en"

// FileName is the name of the project file at the project root (SPEC §3.1).
const FileName = "project.canon"

// SourceExt is the extension of every Canon file (SPEC §3.3).
const SourceExt = ".canon"

// dot separates version numbers and package names, and is the current-directory segment.
const dot = "."

const versionSep = dot

// supported are the language versions this compiler reads (NFR-03).
var supported = [...]Version{{Major: 0, Minor: 1}}

// The keys of project.canon (GRAMMAR.md §7.1).
const (
	keyCanon     = "canon"
	keyRoots     = "roots"
	keyLanguages = "languages"
	keyStudio    = "studio"
	keyBudget    = "budget"
	keyGoModule  = "go_module"
)

// keyRules reads each key; go_module waits for the roots, keys coming in any order (GRAMMAR.md §7.1).
var keyRules = map[string]func(*schema, *syntax.ProjectEntry){
	keyCanon:     (*schema).canon,
	keyRoots:     (*schema).roots,
	keyLanguages: (*schema).languages,
	keyStudio:    (*schema).studio,
	keyBudget:    (*schema).budget,
	keyGoModule:  (*schema).deferGoModule,
}

var (
	versionPattern  = regexp.MustCompile(`^([0-9]+)\.([0-9]+)$`)
	languagePattern = regexp.MustCompile(`^[a-z]{2,3}(_[A-Z][a-z]{3})?(_[A-Z]{2})?$`)
	drivePattern    = regexp.MustCompile(`^[A-Za-z]:`)
)

// The submatches of versionPattern.
const (
	majorGroup = 1
	minorGroup = 2
)

// Paths (WIRE.md §2) and the file set scan (API.md O2).
const (
	sep          = "/"
	backslash    = `\`
	parentSeg    = ".."
	currentSeg   = dot
	rootMark     = "@"
	hiddenPrefix = dot
	nameSep      = dot
	allBelow     = "..."
	relPrefix    = "./"
)

// Texts of Go errors.
const (
	fmtWrap    = "%w"
	unknownSep = ": "
)

// identPattern is the shape of an IDENT, reserved words aside (GRAMMAR.md §2.3).
var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// segmentPattern is the lowerCamel convention of a package segment (GRAMMAR.md §9.2).
var segmentPattern = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)

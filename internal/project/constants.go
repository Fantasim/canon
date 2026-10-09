package project

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// DefaultLanguage is `languages` when project.canon does not declare it (GRAMMAR.md §7.1).
const DefaultLanguage = "en"

// FileName is the name of the project file at the project root (SPEC §3.1).
const FileName = "project.canon"

// LocalFileName is the file beside FileName that places declared roots on one machine (GRAMMAR.md §7.2).
const LocalFileName = syntax.LocalProjectFile

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
	keyOptional  = "optional_roots"
	keyConsumer  = "consumer_roots" // DECISIONS 343
)

// keyRules reads each key; go_module, optional_roots and consumer_roots wait for the roots, keys coming in any order (GRAMMAR.md §7.1).
var keyRules = map[string]func(*schema, *syntax.ProjectEntry){
	keyCanon:     (*schema).canon,
	keyRoots:     (*schema).roots,
	keyLanguages: (*schema).languages,
	keyStudio:    (*schema).studio,
	keyBudget:    (*schema).budget,
	keyGoModule:  (*schema).deferGoModule,
	keyOptional:  (*schema).deferOptional,
	keyConsumer:  (*schema).deferConsumer,
}

// The origins of a root's directory (origin).
const (
	fromDeclared origin = iota
	fromLocal
	fromOption
)

// absentRoot is E1013's variant by where the absent root's directory was placed (SPEC §3.1).
var absentRoot = [...]func(source.Span, string, string) *diag.Builder{
	fromDeclared: diag.E1013.AtDeclared,
	fromLocal:    diag.E1013.AtLocal,
	fromOption:   diag.E1013.AtOption,
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

// uncPrefix starts a UNC share; driveLen is a drive letter volume's length, both volumeOf parses as text, never through the OS.
const (
	uncPrefix = "//"
	driveLen  = 2
	queryHost = "?" // `//?/` and `//./` start a device path, which is no volume
)

// Texts of Go errors.
const (
	fmtWrap    = "%w"
	unknownSep = ": "
)

// MaxSymlinkHops bounds evalSymlinksOS's own walk, an acyclic but excessively long chain reported as a loop too, as the OS itself does.
const MaxSymlinkHops = 40

// identPattern is the shape of an IDENT, reserved words aside (GRAMMAR.md §2.3).
var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// segmentPattern is the lowerCamel convention of a package segment (GRAMMAR.md §9.2).
var segmentPattern = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)

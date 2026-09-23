package project

import (
	"regexp"

	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

const laneName = "project"

const (
	ruleRootClutter = "root-clutter"
	ruleDeadLink    = "dead-link"
)

var ruleIDs = []string{ruleRootClutter, ruleDeadLink}

const (
	mdExt          = repo.MarkdownExt
	goExt          = repo.GoExt
	pathSep        = "/"
	commentMark    = "#"
	fragmentPrefix = "#"
	fragmentByte   = '#'
	lineBreak      = "\n"
)

const (
	rootClutterMessage = "not a standard repo-root entry (intentional? add it to " + repo.RootAllowFile + ")"
	deadLinkMessage    = "link target %q does not exist (resolved %s)"
)

// rootAllowedDirs and rootAllowedFiles are the root-clutter allowlist of a Go project; a
// project's own entries go in .sovaudit/root-allow.txt.
var rootAllowedDirs = []string{
	"cmd", "internal", "pkg", "tools", "docs", "examples", "testdata", "scripts",
	".claude", ".github", ".sovaudit",
}

var rootAllowedFiles = []string{
	"go.mod", "go.sum", "go.work", "go.work.sum", "Makefile", "README.md", "CLAUDE.md",
	"CHANGELOG.md", "CONTRIBUTING.md", "SECURITY.md", "CODE_OF_CONDUCT.md", "LICENSE",
	".gitignore", ".gitattributes", ".editorconfig",
}

var (
	linkRe       = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	schemeRe     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+-]*:`)
	lineSuffixRe = regexp.MustCompile(`:\d+$`)
	fenceRe      = regexp.MustCompile("^\\s*(```|~~~)")
	codeSpanRe   = regexp.MustCompile("`[^`]*`")
)

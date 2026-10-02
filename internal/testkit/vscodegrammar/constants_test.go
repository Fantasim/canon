package vscodegrammar

import "os"

// Paths, relative to the package directory.
const (
	manifestGrammar = "./syntaxes/canon.tmLanguage.json"
	grammarPath     = "../../../editors/vscode/syntaxes/canon.tmLanguage.json"
	packagePath     = "../../../editors/vscode/package.json"
	extensionPath   = "../../../editors/vscode/extension.js"
	examplesRoot    = "../../../examples"
	specGrammarPath = "../../../spec/GRAMMAR.md"
	goldenRoot      = "testdata/scopes"
	goldenExt       = ".scopes"
	expectedDir     = "expected"
)

// File modes of a regenerated golden.
const (
	dirMode  os.FileMode = 0o755
	fileMode os.FileMode = 0o644
)

// Spec extraction: the reserved words are the first fenced block under this heading.
const (
	reservedHeading = "### 4.1 Reserved words"
	fenceMarker     = "```"
	watcherGlob     = "**/*.{canon,json,csv,h,txt}"
	languageID      = "canon"
)

// keywordScopePrefixes are the scope families a reserved word may carry.
var keywordScopePrefixes = []string{"keyword.", "storage.", "constant.language", "variable.language"}

// The approved npm packages (IMPLEMENTATION-PLAN.md 11) and the one argument of the server.
const (
	languageClient      = "vscode-languageclient"
	languageClientRange = "^9.0.1"
	vsce                = "@vscode/vsce"
	vsceRange           = "^3"
	serverArgs          = "args: ['lsp']"
	scopeSeparator      = "\" "
	keywordsRule        = "keywords"
	keywordPrefix       = "\\b(?:"
	keywordSuffix       = ")\\b"
)

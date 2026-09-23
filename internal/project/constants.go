package project

// DefaultLanguage is `languages` when project.canon does not declare it (GRAMMAR.md §7.1).
const DefaultLanguage = "en"

const versionSep = "."

// supported are the language versions this compiler reads (NFR-03).
var supported = [...]Version{{Major: 0, Minor: 1}}

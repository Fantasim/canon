package tsgen

// DeclPattern is the pattern that splits the helper texts into units, for the external tests.
const DeclPattern = declPattern

// DocCommentText is the doc comment of a doc text at no indent, as gen/ts writes it.
func DocCommentText(doc string) string { return docComment("", doc) }

// PureCall is the bundler directive that precedes a top-level construction (CODEGEN.md §2.5 T22).
const PureCall = pureCall

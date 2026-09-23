package threshold

import "regexp"

// FileName is the thresholds file, next to the tool's own source.
const FileName = "thresholds.tsv"

const (
	commentMark      = "#"
	fieldSep         = "\t"
	rowFields        = 2
	lineBreak        = "\n"
	noMax            = 0
	percentMax       = 100
	placeholderGroup = 1
)

// reValue is a positive decimal integer without sign or leading zero.
var reValue = regexp.MustCompile(`^[1-9][0-9]*$`)

// rePlaceholder is a `{key}` reference in a rule summary or a tool configuration.
var rePlaceholder = regexp.MustCompile(`\{([a-z][a-z0-9-]*)\}`)

// Keys of thresholds.tsv, in file order.
const (
	keyFnLines              = "fn-lines"
	keyFnStatements         = "fn-statements"
	keyTestFnLines          = "test-fn-lines"
	keyFnParams             = "fn-params"
	keyFnResults            = "fn-results"
	keyFnComplexity         = "fn-complexity"
	keyFnNesting            = "fn-nesting"
	keyNakedReturnLines     = "naked-return-lines"
	keyFileLines            = "file-lines"
	keyPkgFiles             = "pkg-files"
	keyPkgLines             = "pkg-lines"
	keyMagicStringUses      = "magic-string-uses"
	keyCommentDeclLines     = "comment-decl-lines"
	keyCommentPkgLines      = "comment-pkg-lines"
	keyCommentBlockLines    = "comment-block-lines"
	keyCommentRatioPercent  = "comment-ratio-percent"
	keyCommentRatioMinLines = "comment-ratio-min-lines"
	keyCommentFieldLines    = "comment-field-lines"
	keyDupTokens            = "dup-tokens"
	keyDiagTextWords        = "diag-text-words"
)

// specs maps every key to its field of Set and its upper bound (noMax: none).
var specs = []spec{
	{keyFnLines, func(s *Set) *int { return &s.FnLines }, noMax},
	{keyFnStatements, func(s *Set) *int { return &s.FnStatements }, noMax},
	{keyTestFnLines, func(s *Set) *int { return &s.TestFnLines }, noMax},
	{keyFnParams, func(s *Set) *int { return &s.FnParams }, noMax},
	{keyFnResults, func(s *Set) *int { return &s.FnResults }, noMax},
	{keyFnComplexity, func(s *Set) *int { return &s.FnComplexity }, noMax},
	{keyFnNesting, func(s *Set) *int { return &s.FnNesting }, noMax},
	{keyNakedReturnLines, func(s *Set) *int { return &s.NakedReturnLines }, noMax},
	{keyFileLines, func(s *Set) *int { return &s.FileLines }, noMax},
	{keyPkgFiles, func(s *Set) *int { return &s.PkgFiles }, noMax},
	{keyPkgLines, func(s *Set) *int { return &s.PkgLines }, noMax},
	{keyMagicStringUses, func(s *Set) *int { return &s.MagicStringUses }, noMax},
	{keyCommentDeclLines, func(s *Set) *int { return &s.CommentDeclLines }, noMax},
	{keyCommentPkgLines, func(s *Set) *int { return &s.CommentPkgLines }, noMax},
	{keyCommentBlockLines, func(s *Set) *int { return &s.CommentBlockLines }, noMax},
	{keyCommentRatioPercent, func(s *Set) *int { return &s.CommentRatioPercent }, percentMax},
	{keyCommentRatioMinLines, func(s *Set) *int { return &s.CommentRatioMinLines }, noMax},
	{keyCommentFieldLines, func(s *Set) *int { return &s.CommentFieldLines }, noMax},
	{keyDupTokens, func(s *Set) *int { return &s.DupTokens }, noMax},
	{keyDiagTextWords, func(s *Set) *int { return &s.DiagTextWords }, noMax},
}

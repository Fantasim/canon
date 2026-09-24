package syntax

import "github.com/fantasim/canonlang/internal/diag"

// tokenSet is the set of the given token kinds, as a table indexed by kind.
func tokenSet(ks ...TokenKind) [TokenKindCount]bool {
	var s [TokenKindCount]bool
	for _, k := range ks {
		s[k] = true
	}
	return s
}

// The token sets of the separator pass: cannotEnd (rule 2), continuesLine (rule 3) and
// declKeyword (rule 4), with the closing brackets' openers.
var (
	cannotEnd = tokenSet(TokPlus, TokMinus, TokStar, TokSlash, TokPercent, TokEq, TokNe, TokLt, TokLe,
		TokGt, TokGe, TokCoalesce, TokAssign, TokAddAssign, TokSubAssign, TokMulAssign, TokDivAssign,
		TokFatArrow, TokArrow, TokDot, TokOptDot, TokComma, TokColon, TokPipe, TokLParen, TokLBrack,
		TokLBrace, TokEllipsis, KwAnd, KwOr, KwNot, KwIn, KwIs, KwElse, KwWhere, KwAs)
	continuesLine = tokenSet(TokDot, TokOptDot, TokCoalesce, TokPlus, TokStar, TokSlash, TokPercent,
		TokEq, TokNe, TokLt, TokLe, TokGt, TokGe, TokAssign, TokAddAssign, TokSubAssign, TokMulAssign,
		TokDivAssign, TokFatArrow, TokArrow, TokPipe, KwAnd, KwOr, KwIn, KwIs, KwElse, KwWhere)
	declKeyword = tokenSet(KwLet, KwLocal, KwConst, KwType, KwEnum, KwRecord, KwVariant, KwFn, KwExport,
		KwEntry, KwView, KwWidget, KwTest, KwEmit, KwCheck, KwWarn)
	openerOf = [TokenKindCount]TokenKind{TokRParen: TokLParen, TokRBrack: TokLBrack, TokRBrace: TokLBrace}
)

// nameable are the reserved words read as identifiers in value position (GRAMMAR.md §4.3 (c)).
var nameable = tokenSet(KwAmend, KwAsset, KwCheck, KwConst, KwEmit, KwEntry, KwEnum, KwExpect,
	KwExport, KwImport, KwInput, KwLayer, KwLocal, KwPackage, KwProject, KwRecord, KwRef, KwRetired,
	KwStable, KwTable, KwTest, KwTranslation, KwType, KwVar, KwVariant, KwView, KwWarn, KwWidget)

// The token sets the parser decides on. stopToken are the tokens a missing operand never
// consumes; topSyncKind the tokens that, first on their line, resume a broken file.
var (
	isNumber      = tokenSet(TokInt, TokFloat, TokDuration)
	startsString  = tokenSet(TokString, TokStringHead, TokMLString, TokMLStringHead, TokRaw, TokRawML)
	isPiece       = tokenSet(TokStringMid, TokStringTail, TokMLStringMid, TokMLStringTail)
	startsPostfix = tokenSet(TokDot, TokOptDot, TokLBrack, TokLParen, TokBang)
	isAssign      = tokenSet(TokAssign, TokAddAssign, TokSubAssign, TokMulAssign, TokDivAssign)
	isMember      = tokenSet(KwFn, KwCheck, KwWarn)
	isModifier    = tokenSet(KwLocal, KwExport, KwRetired)
	stopToken     = tokenSet(TokNL, TokEOF, TokComma, TokRParen, TokRBrack, TokRBrace, TokLBrace,
		TokColon, TokAssign, TokFatArrow, TokArrow, KwElse, TokAt)
	startsItemTok = tokenSet(TokIdent, TokInt, TokFloat, TokDuration, TokString, TokStringHead,
		TokMLString, TokMLStringHead, TokRaw, TokRawML, TokAt, TokEllipsis, TokLParen, TokLBrack,
		TokMinus, TokDot, TokUnderscore)
	topSyncKind = tokenSet(KwLet, KwLocal, KwConst, KwType, KwEnum, KwRecord, KwVariant, KwFn,
		KwExport, KwEntry, KwView, KwWidget, KwTest, KwEmit, KwCheck, KwWarn, KwRetired, KwImport,
		KwPackage, KwAmend, KwProject, TokAt)
)

// widgetParams are the parameter words of a widget, in order (GRAMMAR.md §5.3).
var widgetParams = [...]string{WordValue, wordSiblings}

// siteKinds name each annotation site in E1118, by position (GRAMMAR.md §8.1).
var siteKinds = [...]struct {
	sites annSite
	kind  diag.Kind
}{
	{siteTop, diag.KindTopLevelDeclaration},
	{siteHeader, diag.KindTypeHeader},
	{siteField, diag.KindField},
	{siteMember | siteCodedMember, diag.KindEnumMember},
	{siteCase, diag.KindVariantCase},
	{siteTableEntry, diag.KindTableEntry},
	{siteMethod, diag.KindMethodOrCheck},
}

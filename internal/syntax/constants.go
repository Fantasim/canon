package syntax

import "unicode/utf8"

// NoTok is the Tok of an absent token: File.Tokens[0] is the BOF sentinel, never a node's.
const NoTok Tok = 0

// NoDecimals is FormatSpec.Decimals when the spec has no ".N" part.
const NoDecimals = -1

// Blank is the name of the "_" binder.
const Blank = "_"

// Token kinds: GRAMMAR.md §2.1 classes, the §2.8 punctuation and the §4.1 reserved words.
const (
	TokInvalid TokenKind = iota
	TokBOF
	TokEOF
	TokNL
	TokIllegal
	TokIdent
	TokInt
	TokFloat
	TokDuration
	TokString
	TokStringHead
	TokStringMid
	TokStringTail
	TokMLString
	TokMLStringHead
	TokMLStringMid
	TokMLStringTail
	TokRaw
	TokRawML
	TokRegex
	TokFormatSpec
	TokEllipsis
	TokRangeIncl
	TokRange
	TokCoalesce
	TokOptDot
	TokFatArrow
	TokArrow
	TokEq
	TokNe
	TokLe
	TokGe
	TokAddAssign
	TokSubAssign
	TokMulAssign
	TokDivAssign
	TokPlus
	TokMinus
	TokStar
	TokSlash
	TokPercent
	TokLt
	TokGt
	TokAssign
	TokBang
	TokQuestion
	TokDot
	TokComma
	TokColon
	TokLParen
	TokRParen
	TokLBrack
	TokRBrack
	TokLBrace
	TokRBrace
	TokAt
	TokPipe
	TokUnderscore
	TokHash
	KwAnd
	KwAmend
	KwAs
	KwAsset
	KwBreak
	KwCheck
	KwConst
	KwContinue
	KwElse
	KwEmit
	KwEntry
	KwEnum
	KwExport
	KwExpect
	KwFalse
	KwFn
	KwFor
	KwIf
	KwImport
	KwIn
	KwInput
	KwIs
	KwLayer
	KwLet
	KwLoad
	KwLocal
	KwMatch
	KwNone
	KwNot
	KwOr
	KwPackage
	KwProject
	KwRecord
	KwRef
	KwRetired
	KwReturn
	KwSelf
	KwStable
	KwTable
	KwTest
	KwTranslation
	KwTrue
	KwType
	KwVar
	KwVariant
	KwView
	KwWarn
	KwWhere
	KwWhile
	KwWidget
	TokenKindCount
)

// tokenNames spells each token kind: the class name, or the exact text of a fixed token.
var tokenNames = [TokenKindCount]string{
	TokInvalid: "INVALID", TokBOF: "BOF", TokEOF: "EOF", TokNL: "NL", TokIllegal: "ILLEGAL", TokIdent: "IDENT",
	TokInt: "INT", TokFloat: "FLOAT", TokDuration: "DURATION", TokString: "STRING",
	TokStringHead: "STRING_HEAD", TokStringMid: "STRING_MID", TokStringTail: "STRING_TAIL",
	TokMLString: "MLSTRING", TokMLStringHead: "MLSTRING_HEAD", TokMLStringMid: "MLSTRING_MID",
	TokMLStringTail: "MLSTRING_TAIL", TokRaw: "RAW", TokRawML: "RAWML", TokRegex: "REGEX",
	TokFormatSpec: "FORMAT_SPEC",
	TokEllipsis:   "...", TokRangeIncl: "..=", TokRange: "..", TokCoalesce: "??", TokOptDot: "?.",
	TokFatArrow: "=>", TokArrow: "->", TokEq: "==", TokNe: "!=", TokLe: "<=", TokGe: ">=",
	TokAddAssign: "+=", TokSubAssign: "-=", TokMulAssign: "*=", TokDivAssign: "/=", TokPlus: "+",
	TokMinus: "-", TokStar: "*", TokSlash: "/", TokPercent: "%", TokLt: "<", TokGt: ">",
	TokAssign: "=", TokBang: "!", TokQuestion: "?", TokDot: ".", TokComma: ",", TokColon: ":",
	TokLParen: "(", TokRParen: ")", TokLBrack: "[", TokRBrack: "]", TokLBrace: "{", TokRBrace: "}",
	TokAt: "@", TokPipe: "|", TokUnderscore: Blank, TokHash: "#",
	KwAnd: "and", KwAmend: "amend", KwAs: "as", KwAsset: "asset", KwBreak: "break",
	KwCheck: "check", KwConst: "const", KwContinue: "continue", KwElse: "else", KwEmit: "emit",
	KwEntry: "entry", KwEnum: "enum", KwExport: "export", KwExpect: "expect", KwFalse: "false",
	KwFn: "fn", KwFor: "for", KwIf: "if", KwImport: "import", KwIn: "in", KwInput: "input",
	KwIs: "is", KwLayer: "layer", KwLet: "let", KwLoad: "load", KwLocal: "local",
	KwMatch: "match", KwNone: "none", KwNot: "not", KwOr: "or", KwPackage: "package",
	KwProject: "project", KwRecord: "record", KwRef: "ref", KwRetired: "retired",
	KwReturn: "return", KwSelf: "self", KwStable: "stable", KwTable: "table", KwTest: "test",
	KwTranslation: "translation", KwTrue: "true", KwType: "type", KwVar: "var",
	KwVariant: "variant", KwView: "view", KwWarn: "warn", KwWhere: "where", KwWhile: "while",
	KwWidget: "widget",
}

// Trivia kinds (GRAMMAR.md §2.1, §2.2); a leading byte order mark is kept as trivia (E1123).
const (
	TriviaInvalid TriviaKind = iota
	TriviaSpace
	TriviaNewline
	TriviaLineComment
	TriviaBlockComment
	TriviaDocComment
	TriviaBOM
)

// File kinds (GRAMMAR.md §5.2); FileInvalid is a file whose first tokens name no kind.
const (
	FileInvalid FileKind = iota
	FileSource
	FileLayer
	FileTranslation
	FileProject
)

// Node kinds, one per node type; each kind's String is its type's name.
const (
	KindInvalid NodeKind = iota
	KindFile
	KindIdent
	KindQualifiedName
	KindDocComment
	KindAnnotation
	KindAnnotationArg
	KindAnnotationList
	KindModifiers
	KindImport
	KindConstDecl
	KindLetDecl
	KindTypeDecl
	KindParam
	KindFnDecl
	KindEntryDecl
	KindCheckDecl
	KindViewDecl
	KindWidgetDecl
	KindTestDecl
	KindEmitDecl
	KindRecordDecl
	KindRecordBody
	KindFieldDecl
	KindEnumDecl
	KindEnumMember
	KindVariantDecl
	KindVariantCase
	KindAmendBlock
	KindAmendment
	KindAmendSegment
	KindTranslationEntry
	KindProjectDecl
	KindProjectEntry
	KindProjectList
	KindProjectMap
	KindViewTitle
	KindViewSubtitle
	KindViewSingular
	KindViewPlural
	KindViewMenu
	KindViewColumns
	KindViewColumn
	KindViewSearch
	KindViewFilters
	KindViewFilter
	KindViewPreview
	KindViewShow
	KindViewGroup
	KindViewField
	KindNamedType
	KindTypeArgs
	KindListType
	KindMapType
	KindDepMapType
	KindTableType
	KindRefType
	KindOptionalType
	KindKeyedType
	KindWhereType
	KindUnionType
	KindLiteralType
	KindMatchType
	KindTypeArm
	KindAssetType
	KindAnyType
	KindFnType
	KindParenType
	KindIdentExpr
	KindIntLit
	KindFloatLit
	KindDurationLit
	KindStringLit
	KindInterp
	KindRawStringLit
	KindRegexLit
	KindBoolLit
	KindNoneLit
	KindSelfExpr
	KindParenExpr
	KindUnaryExpr
	KindBinaryExpr
	KindIsExpr
	KindRangeExpr
	KindSelectorExpr
	KindIndexExpr
	KindCallExpr
	KindArg
	KindForceExpr
	KindLambdaExpr
	KindShorthandLambda
	KindListLit
	KindListComp
	KindBraceLit
	KindFieldItem
	KindMapItem
	KindEntryItem
	KindSpreadItem
	KindCompClause
	KindTypedLit
	KindIfExpr
	KindExprBody
	KindMatchExpr
	KindMatchArm
	KindPattern
	KindLoadExpr
	KindBlock
	KindLetStmt
	KindVarStmt
	KindAssignStmt
	KindIfStmt
	KindForStmt
	KindWhileStmt
	KindBreakStmt
	KindContinueStmt
	KindReturnStmt
	KindExpectStmt
	KindMatchStmt
	KindStmtArm
	KindExprStmt
	KindBadExpr
	KindBadType
	KindBadStmt
	KindBadDecl
	NodeKindCount
)

var kindNames = [NodeKindCount]string{
	KindInvalid: "Invalid", KindFile: "File", KindIdent: "Ident", KindQualifiedName: "QualifiedName",
	KindDocComment: "DocComment", KindAnnotation: "Annotation", KindAnnotationArg: "AnnotationArg",
	KindAnnotationList: "AnnotationList", KindModifiers: "Modifiers", KindImport: "Import",
	KindConstDecl: "ConstDecl", KindLetDecl: "LetDecl", KindTypeDecl: "TypeDecl", KindParam: "Param",
	KindFnDecl: "FnDecl", KindEntryDecl: "EntryDecl", KindCheckDecl: "CheckDecl",
	KindViewDecl: "ViewDecl", KindWidgetDecl: "WidgetDecl", KindTestDecl: "TestDecl",
	KindEmitDecl: "EmitDecl", KindRecordDecl: "RecordDecl", KindRecordBody: "RecordBody",
	KindFieldDecl: "FieldDecl", KindEnumDecl: "EnumDecl", KindEnumMember: "EnumMember",
	KindVariantDecl: "VariantDecl", KindVariantCase: "VariantCase", KindAmendBlock: "AmendBlock",
	KindAmendment: "Amendment", KindAmendSegment: "AmendSegment",
	KindTranslationEntry: "TranslationEntry", KindProjectDecl: "ProjectDecl",
	KindProjectEntry: "ProjectEntry", KindProjectList: "ProjectList", KindProjectMap: "ProjectMap",
	KindViewTitle: "ViewTitle", KindViewSubtitle: "ViewSubtitle", KindViewSingular: "ViewSingular",
	KindViewPlural: "ViewPlural", KindViewMenu: "ViewMenu", KindViewColumns: "ViewColumns",
	KindViewColumn: "ViewColumn", KindViewSearch: "ViewSearch", KindViewFilters: "ViewFilters",
	KindViewFilter: "ViewFilter", KindViewPreview: "ViewPreview", KindViewShow: "ViewShow",
	KindViewGroup: "ViewGroup", KindViewField: "ViewField", KindNamedType: "NamedType",
	KindTypeArgs: "TypeArgs", KindListType: "ListType", KindMapType: "MapType",
	KindDepMapType: "DepMapType", KindTableType: "TableType", KindRefType: "RefType",
	KindOptionalType: "OptionalType", KindKeyedType: "KeyedType", KindWhereType: "WhereType",
	KindUnionType: "UnionType", KindLiteralType: "LiteralType", KindMatchType: "MatchType",
	KindTypeArm: "TypeArm", KindAssetType: "AssetType", KindAnyType: "AnyType",
	KindFnType: "FnType", KindParenType: "ParenType", KindIdentExpr: "IdentExpr",
	KindIntLit: "IntLit", KindFloatLit: "FloatLit", KindDurationLit: "DurationLit",
	KindStringLit: "StringLit", KindInterp: "Interp", KindRawStringLit: "RawStringLit",
	KindRegexLit: "RegexLit", KindBoolLit: "BoolLit", KindNoneLit: "NoneLit",
	KindSelfExpr: "SelfExpr", KindParenExpr: "ParenExpr", KindUnaryExpr: "UnaryExpr",
	KindBinaryExpr: "BinaryExpr", KindIsExpr: "IsExpr", KindRangeExpr: "RangeExpr",
	KindSelectorExpr: "SelectorExpr", KindIndexExpr: "IndexExpr", KindCallExpr: "CallExpr",
	KindArg: "Arg", KindForceExpr: "ForceExpr", KindLambdaExpr: "LambdaExpr",
	KindShorthandLambda: "ShorthandLambda", KindListLit: "ListLit", KindListComp: "ListComp",
	KindBraceLit: "BraceLit", KindFieldItem: "FieldItem", KindMapItem: "MapItem",
	KindEntryItem: "EntryItem", KindSpreadItem: "SpreadItem", KindCompClause: "CompClause",
	KindTypedLit: "TypedLit", KindIfExpr: "IfExpr", KindExprBody: "ExprBody",
	KindMatchExpr: "MatchExpr", KindMatchArm: "MatchArm", KindPattern: "Pattern",
	KindLoadExpr: "LoadExpr", KindBlock: "Block", KindLetStmt: "LetStmt", KindVarStmt: "VarStmt",
	KindAssignStmt: "AssignStmt", KindIfStmt: "IfStmt", KindForStmt: "ForStmt",
	KindWhileStmt: "WhileStmt", KindBreakStmt: "BreakStmt", KindContinueStmt: "ContinueStmt",
	KindReturnStmt: "ReturnStmt", KindExpectStmt: "ExpectStmt", KindMatchStmt: "MatchStmt",
	KindStmtArm: "StmtArm", KindExprStmt: "ExprStmt", KindBadExpr: "BadExpr", KindBadType: "BadType",
	KindBadStmt: "BadStmt", KindBadDecl: "BadDecl",
}

// Lexer texts and widths (GRAMMAR.md §2).
const (
	utf8Self, asciiLowerBit, pairWidth, maxHexDigits, runeBits, boolCount, nlShare = utf8.RuneSelf, 0x20, 2, 6, 32, 2, 4
	bomText, tripleQuote, quoteText, rawPrefix, blankChars, spaceText, lf          = "\xEF\xBB\xBF", `"""`, `"`, "r", " \t", " ", "\n"
	docPrefix, ordinaryDoc, lineCommentText, blockOpen, blockClose                 = "///", "////", "//", "/*", "*/"
	slashText, openBraceText, closeBraceText                                       = "/", "{", "}"
	unicodeOpen                                                                    = 3
)

// Classes of a byte of string text, the index of stringByte.
const strPlain, strQuote, strNewline, strEscape, strOpen, strClose, strClassCount = 0, 1, 2, 3, 4, 5, 6

// Numbers and durations (GRAMMAR.md §2.4, §2.5): problems, bases, texts and limits.
const (
	numOK, numBad, durOrder, durRepeated, durFraction, durTrailing, durOverflow numProblem = 0, 1, 2, 3, 4, 5, 6

	decimalBase, hexBase, binBase, minExpLen, maxMillisBits, maxSpecDigits, maxDecimals = 10, 16, 2, 2, 63, 2, 20
	hexPrefix, binPrefix, digitSep, doubleSep, fractionDot, expLetters, unitLetters     = "0x", "0b", "_", "__", ".", "eE", "mshd"
	specPlus, specComma                                                                 = "+", ","
)

// unitsLongestFirst are the duration units as matched, with their rank (d h m s ms) in
// unitRank; unitMillis is each rank's length in milliseconds.
var (
	unitsLongestFirst = [...]string{"ms", "m", "s", "h", "d"}
	unitRank          = [...]int{4, 2, 3, 1, 0}
	unitMillis        = [...]int64{86_400_000, 3_600_000, 60_000, 1_000, 1}
	simpleEscapes     = map[byte]byte{'n': '\n', 't': '\t', 'r': '\r', '\\': '\\', '"': '"', '{': '{', '}': '}'}
)

// Names E1116 prints for what it expected: GRAMMAR.md symbols (§5.1) where no token is meant.
const (
	identName, wordName, exprName, typeName, stringName, numberName = "IDENT", "WORD", "expr", "type", "stringLit", "number"
	itemName, declName, annValueName, projectValueName, nestingName = "item", "topDecl", "annValue", "pValue", "nesting"
)

// Contextual keywords (GRAMMAR.md §4.2), predeclared names and the parser's limits.
const (
	wordAt, wordFrom, wordEnv, wordKeyed, wordBy, wordOrdered, wordExt = "at", "from", "env", "keyed", "by", "ordered", "ext"
	wordPasses, wordFails, wordWarns, wordDefault                      = "passes", "fails", "warns", "default"
	WordValue, wordSiblings, wordMulti, wordAdvanced, wordWhen         = "value", "siblings", "multi", "advanced", "when"
	wordTitle, wordSubtitle, wordSingular, wordPlural, wordMenu        = "title", "subtitle", "singular", "plural", "menu"
	wordIcon, wordPreview, wordSearch, wordFilters, wordColumns        = "icon", "preview", "search", "filters", "columns"
	wordGroup, wordShow, WordField, matchesName, failName, pairIndex   = "group", "show", "field", "matches", "fail", "i"
	maxNesting                                                         = 1000
)

// Modifier bits, in the order of Modifiers' fields (GRAMMAR.md §5.3, E1133).
const modLocal, modExport, modRetired uint8 = 1, 2, 4

// Precedence levels of GRAMMAR.md §5.11, lowest first; binaryLevel is each operator's.
const (
	levelNone, levelCoalesce, levelOr, levelAnd, levelNot, levelCompare      uint8 = 0, 1, 2, 3, 4, 5
	levelRange, levelAdditive, levelMultiplicative, levelUnary, levelPostfix uint8 = 6, 7, 8, 9, 10
	levelCount                                                                     = 11
)

var binaryLevel = [TokenKindCount]uint8{
	TokCoalesce: levelCoalesce, KwOr: levelOr, KwAnd: levelAnd, TokEq: levelCompare,
	TokNe: levelCompare, TokLt: levelCompare, TokLe: levelCompare, TokGt: levelCompare,
	TokGe: levelCompare, KwIn: levelCompare, TokRange: levelRange, TokRangeIncl: levelRange,
	TokPlus: levelAdditive, TokMinus: levelAdditive, TokStar: levelMultiplicative,
	TokSlash: levelMultiplicative, TokPercent: levelMultiplicative,
}

// Annotation sites (GRAMMAR.md §8.1): TL per declaration, TH per type, FD, EM, VC, TE, MB.
const (
	siteLet, siteConst, siteType, siteFn, siteEntry, siteOther                 annSite = 1 << 0, 1 << 1, 1 << 2, 1 << 3, 1 << 4, 1 << 5
	siteRecordHeader, siteEnumHeader, siteVariantHeader, siteField, siteMember annSite = 1 << 6, 1 << 7, 1 << 8, 1 << 9, 1 << 10
	siteCodedMember, siteCase, siteTableEntry, siteMethod                      annSite = 1 << 11, 1 << 12, 1 << 13, 1 << 14

	siteAll         = siteMethod<<1 - 1
	siteTop         = siteLet | siteConst | siteType | siteFn | siteEntry | siteOther
	siteHeader      = siteRecordHeader | siteEnumHeader | siteVariantHeader
	deprecatedSites = siteField | siteMember | siteCodedMember | siteCase | siteTableEntry | siteEntry
	nameSites       = siteHeader | siteField | siteMember | siteCodedMember | siteCase | siteMethod | siteLet | siteConst | siteType | siteFn
)

// Kinds of annotation argument values (GRAMMAR.md §8.2), and the exclusion groups of @json.
const (
	valString, valTemplate, valInteger, valSince, valLiteral, valPairs, valSymbol, valStudio, valFlag argKind = 0, 1, 2, 3, 4, 5, 6, 7, 8
	valKindCount                                                                                              = 9
	groupWire, groupShape                                                                             uint8   = 1, 2
)

// The annotation catalogue's names (GRAMMAR.md §8.3).
const (
	annJSON, annStable, annCodes, annDeprecated, annSince, AnnReload = "json", "stable", "codes", "deprecated", "since", "reload"
	AnnFiles, annMenu, AnnCpp, AnnGo, AnnTS                          = "files", "menu", "cpp", "go", "ts"
	argWire, argPath, argCase, argTag, argInline, ArgUnit, argInt    = "wire", "path", "case", "tag", "inline", "unit", "int"
	argBits, argPairs, argT, argWhy, argN, argTpl, argLabel          = "bits", "pairs", "T", "why", "n", "tpl", "label"
	ArgDefines, ArgStruct, ArgHeader, ArgAccess, ArgName, ArgBigint  = "defines", "struct", "header", "access", "name", "bigint"
)

// Closed sets of annotation symbols (GRAMMAR.md §8.3).
var (
	caseStyles  = []string{"snake", "camel", "kebab", "upper_snake"}
	codeTypes   = []string{"Int8", "Int16", "Int32", "Int", "UInt8", "UInt16", "UInt32", "UInt64"}
	accessModes = []string{"fields", "both", "getters"}
)

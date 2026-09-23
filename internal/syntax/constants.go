package syntax

// NoTok is the Tok of an absent token.
const NoTok Tok = -1

// NoDecimals is FormatSpec.Decimals when the spec has no ".N" part.
const NoDecimals = -1

// Blank is the name of the "_" binder.
const Blank = "_"

// Token kinds: GRAMMAR.md §2.1 classes, the §2.8 punctuation and the §4.1 reserved words.
const (
	TokInvalid TokenKind = iota
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
	TokInvalid: "INVALID", TokEOF: "EOF", TokNL: "NL", TokIllegal: "ILLEGAL", TokIdent: "IDENT",
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
	KindStmtArm: "StmtArm", KindExprStmt: "ExprStmt",
}

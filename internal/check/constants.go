package check

// NoneIndex is the index MatchInfo.Covers gives the `none` pattern (TYPES.md §12.6).
const NoneIndex = -1

// The object kinds; ObjParam is any parameter, lambdas' included, ObjLocal any other binder.
const (
	ObjConst ObjKind = iota
	ObjLet
	ObjFn
	ObjMethod
	ObjParam
	ObjLocal
	ObjField
	ObjMember
	ObjCase
	ObjEntry
	ObjBuiltin
	ObjTypeName
	ObjPackage
	ObjLayer
	ObjCheck
	ObjTest
	ObjWidget
)

// What `.name` selects (TYPES.md §3.5).
const (
	SelField SelKind = iota
	SelEntry
	SelBuiltinMember
	SelMethod
)

// The conversions of TYPES.md §6.2.
const (
	ConvWrap          ConvKind = iota // T to T?; Inner, if any, first
	ConvDeref                         // ref T to T; E3501 deferred
	ConvEntryToRef                    // T to ref T; E3503 deferred
	ConvIntLitToFloat                 // an integer literal to Float
	ConvCaseToVariant                 // V.c to V
	ConvToList                        // table T or a keyed list to [T], identities kept
	ConvElements                      // per element of a list, map or pair: Key and Inner
	ConvPresent                       // S? to T?: Inner on a present value
)

// What a call calls; CalleeConvert is `Int(f)` (STDLIB.md §2.1), CalleeLambda a function value.
const (
	CalleeFn CalleeKind = iota
	CalleeMethod
	CalleeBuiltin
	CalleeConvert
	CalleeLambda
)

// The classifications of TYPES.md §5.2; LitError is a literal no row classifies.
const (
	LitRecord LitKind = iota
	LitTable
	LitMap
	LitMapComp
	LitError
)

// Names shared by several kinds.
const (
	nameField   = "Field"
	nameMethod  = "Method"
	nameFn      = "Fn"
	nameBuiltin = "Builtin"
	nameEntry   = "Entry"
)

var objNames = [...]string{
	ObjConst: "Const", ObjLet: "Let", ObjFn: nameFn, ObjMethod: nameMethod, ObjParam: "Param",
	ObjLocal: "Local", ObjField: nameField, ObjMember: "Member", ObjCase: "Case", ObjEntry: nameEntry,
	ObjBuiltin: nameBuiltin, ObjTypeName: "TypeName", ObjPackage: "Package", ObjLayer: "Layer",
	ObjCheck: "Check", ObjTest: "Test", ObjWidget: "Widget",
}

var selNames = [...]string{
	SelField: nameField, SelEntry: nameEntry, SelBuiltinMember: "BuiltinMember", SelMethod: nameMethod,
}

var convNames = [...]string{
	ConvWrap: "Wrap", ConvDeref: "Deref", ConvEntryToRef: "EntryToRef", ConvIntLitToFloat: "IntLitToFloat",
	ConvCaseToVariant: "CaseToVariant", ConvToList: "ToList", ConvElements: "Elements", ConvPresent: "Present",
}

var calleeNames = [...]string{
	CalleeFn: nameFn, CalleeMethod: nameMethod, CalleeBuiltin: nameBuiltin, CalleeConvert: "Convert",
	CalleeLambda: "Lambda",
}

var litNames = [...]string{
	LitRecord: "RecordLit", LitTable: "TableLit", LitMap: "MapLit", LitMapComp: "MapComp", LitError: "ErrorLit",
}

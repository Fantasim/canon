package syntax

import "slices"

// annSite is where an annotation stands: one bit per position, and per declaration kind at the
// top level and on type headers.
type annSite uint16

// argKind is the kind of value an annotation argument takes (GRAMMAR.md §8.2).
type argKind uint8

// argSpec is one argument of an annotation: named or positional (a flag is a positional symbol),
// its kind, its closed set of symbols, where it may stand, the exclusion groups it belongs to,
// and whether it is required.
type argSpec struct {
	name     string
	named    bool
	kind     argKind
	values   []string
	sites    annSite
	groups   uint8
	required bool
}

// annSpec is one annotation of the catalogue: where it may stand without arguments, and its
// arguments.
type annSpec struct {
	bare annSite
	args []argSpec
}

// annCatalog is the annotation catalogue of GRAMMAR.md §8.3 (GRM-18).
var annCatalog = map[string]*annSpec{
	annJSON: {args: []argSpec{
		{name: argWire, kind: valString, sites: siteField | siteCase | siteCodedMember, groups: groupWire},
		{name: argPath, named: true, kind: valString, sites: siteField, groups: groupWire},
		{name: ArgCase, named: true, kind: valSymbol, values: caseStyles, sites: siteRecordHeader | siteVariantHeader | siteCase},
		{name: argTag, named: true, kind: valString, sites: siteVariantHeader},
		{name: argInline, kind: valFlag, sites: siteField, groups: groupShape},
		{name: tokenNames[KwNone], named: true, kind: valLiteral, sites: siteField},
		{name: ArgUnit, named: true, kind: valSymbol, values: unitsLongestFirst[:], sites: siteField, groups: groupShape},
		{name: argInt, kind: valFlag, sites: siteField, groups: groupShape},
		{name: argBits, kind: valFlag, sites: siteField, groups: groupShape},
		{name: annCodes, kind: valFlag, sites: siteEnumHeader},
		{name: argPairs, named: true, kind: valPairs, sites: siteField, groups: groupWire | groupShape},
	}},
	annStable: {bare: siteField},
	annCodes: {args: []argSpec{
		{name: argT, kind: valSymbol, values: codeTypes, sites: siteEnumHeader, required: true},
	}},
	AnnDeprecated: {bare: deprecatedSites, args: []argSpec{
		{name: argWhy, kind: valString, sites: deprecatedSites},
	}},
	annSince: {args: []argSpec{
		{name: argN, kind: valSince, sites: siteAll, required: true},
	}},
	AnnReload: {bare: siteLet},
	AnnFiles: {args: []argSpec{
		{name: argTpl, kind: valTemplate, sites: siteLet, required: true},
	}},
	AnnMenu: {args: []argSpec{
		{name: AnnMenu, kind: valStudio, sites: siteLet, required: true},
		{name: wordIcon, named: true, kind: valStudio, sites: siteLet},
		{name: ArgLabel, named: true, kind: valString, sites: siteLet},
	}},
	AnnCpp: {args: []argSpec{
		{name: ArgDefines, named: true, kind: valString, sites: siteEnumHeader},
		{name: ArgStruct, named: true, kind: valString, sites: siteRecordHeader},
		{name: ArgHeader, named: true, kind: valString, sites: siteRecordHeader},
		{name: ArgAccess, named: true, kind: valSymbol, values: accessModes, sites: siteRecordHeader},
		{name: WordField, named: true, kind: valString, sites: siteField},
		{name: tokenNames[KwType], named: true, kind: valString, sites: siteField},
		{name: WordValue, named: true, kind: valInteger, sites: siteCase},
		{name: ArgUnit, named: true, kind: valSymbol, values: unitsLongestFirst[:], sites: siteField},
		nameArg,
	}},
	AnnGo: {args: []argSpec{nameArg}},
	AnnTS: {args: []argSpec{nameArg, {name: ArgBigint, kind: valFlag, sites: siteField}}},
}

// AccessModes are the symbols of @cpp(access:), in order (GRAMMAR.md §8.3), a copy the caller may keep.
func AccessModes() []string { return slices.Clone(accessModes) }

// nameArg is the name: argument of @cpp, @go and @ts (CODEGEN.md CG-02).
var nameArg = argSpec{name: ArgName, named: true, kind: valString, sites: nameSites}

// argNeed is an argument that needs another one of the same annotation.
type argNeed struct {
	arg, needs string
}

// annNeeds are the arguments that need another one (GRAMMAR.md §8.3: header needs struct).
var annNeeds = map[string]argNeed{AnnCpp: {arg: ArgHeader, needs: ArgStruct}}

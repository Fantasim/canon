package check

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// tvar is a type parameter of a built-in signature (STDLIB.md §1.1), bound per call and never recorded in Info.
type tvar struct {
	name   string
	cons   constraint
	hidden bool // bound from the receiver's collection, not a parameter of STDLIB.md
}

func (v *tvar) Kind() types.Kind       { return types.Any }
func (v *tvar) String() string         { return v.name }
func (v *tvar) Underlying() types.Type { return v }
func (v *tvar) Base() types.Type       { return v }

// constraint is what a type parameter accepts (STDLIB.md §1.1).
type constraint uint8

// seqOf is the parameter type Seq(T): a list, a keyed list or a table of T.
type seqOf struct {
	elem types.Type
}

func (s *seqOf) Kind() types.Kind       { return types.List }
func (s *seqOf) String() string         { return seqText + s.elem.String() + closeParen }
func (s *seqOf) Underlying() types.Type { return s }
func (s *seqOf) Base() types.Type       { return s }

// The type parameters: T and the map's K and V come from the receiver; the others from the
// arguments. keyT is a keyed collection's key type, refT a ref into it.
var (
	tT    = &tvar{name: nameT}
	tU    = &tvar{name: nameU}
	tK    = &tvar{name: nameK}
	tV    = &tvar{name: nameV}
	tKOrd = &tvar{name: nameK, cons: consOrd}
	tKKey = &tvar{name: nameK, cons: consKey}
	tNum  = &tvar{name: nameT, cons: consNumD}
	tEq   = &tvar{name: nameT, cons: consEq}
	keyT  = &tvar{name: nameKT, hidden: true}
	refT  = &tvar{name: nameRefT, hidden: true}
)

// bsig is one row of a STDLIB.md signature table.
type bsig struct {
	name     string
	params   []bparam
	result   types.Type
	variadic bool // min and max: two or more arguments of the last parameter's type
	elemCons constraint
}

// bparam is a parameter of a built-in: its name and pattern type.
type bparam struct {
	name string
	t    types.Type
}

func fnOf(result types.Type, params ...types.Type) *types.FuncType {
	return &types.FuncType{Params: params, Result: result}
}

func listT(t types.Type) *types.ListType { return &types.ListType{Elem: t} }

func opt(t types.Type) *types.OptionalType { return &types.OptionalType{Elem: t} }

func bp(name string, t types.Type) bparam { return bparam{name: name, t: t} }

var (
	predT  = bp(paramPred, fnOf(types.BoolType, tT))
	predKV = bp(paramPred, fnOf(types.BoolType, tK, tV))
	otherT = bp(paramB, &seqOf{elem: tT})
	sepS   = bp(paramSep, types.StringType)
	strS   = bp(paramS, types.StringType)
)

// seqMethods are STDLIB.md §4, in its order: every method of Seq(T).
var seqMethods = []bsig{
	{name: methodLen, result: types.IntType},
	{name: methodIsEmpty, result: types.BoolType},
	{name: methodFirst, result: opt(tT)},
	{name: methodLast, result: opt(tT)},
	{name: methodFirst, params: []bparam{predT}, result: opt(tT)},
	{name: methodContains, params: []bparam{bp(paramX, tT)}, result: types.BoolType},
	{name: methodIndexOf, params: []bparam{bp(paramX, tT)}, result: opt(types.IntType)},
	{name: methodMap, params: []bparam{bp(paramF, fnOf(tU, tT))}, result: listT(tU)},
	{name: methodFilter, params: []bparam{predT}, result: listT(tT)},
	{name: methodFlatMap, params: []bparam{bp(paramF, fnOf(listT(tU), tT))}, result: listT(tU)},
	{name: methodFlatten, result: listT(tU)},
	{name: methodReverse, result: listT(tT)},
	{name: methodSortBy, params: []bparam{bp(paramF, fnOf(tKOrd, tT))}, result: listT(tT)},
	{name: methodUnique, result: listT(tT)},
	{name: methodEnumerate, result: listT(&types.PairType{A: types.IntType, B: tT})},
	{name: methodPairs, result: listT(&types.PairType{A: tT, B: tT})},
	{name: methodZip, params: []bparam{bp(paramOther, &seqOf{elem: tU})}, result: listT(&types.PairType{A: tT, B: tU})},
	{name: methodIntersect, params: []bparam{otherT}, result: listT(tT)},
	{name: methodUnion, params: []bparam{otherT}, result: listT(tT)},
	{name: methodDiff, params: []bparam{otherT}, result: listT(tT)},
	{name: methodGroupBy, params: []bparam{bp(paramF, fnOf(tKKey, tT))}, result: &types.MapType{Key: tKKey, Value: listT(tT)}},
	{name: methodToMap, params: []bparam{bp(paramKeyF, fnOf(tKKey, tT)), bp(paramValF, fnOf(tV, tT))}, result: &types.MapType{Key: tKKey, Value: tV}},
	{name: methodJoin, params: []bparam{sepS}, result: types.StringType, elemCons: consString},
	{name: methodAny, params: []bparam{predT}, result: types.BoolType},
	{name: methodAll, params: []bparam{predT}, result: types.BoolType},
	{name: methodCount, params: []bparam{predT}, result: types.IntType},
	{name: methodIsUnique, result: types.BoolType},
	{name: methodSum, result: tT, elemCons: consNumD},
	{name: methodMin, result: opt(tT), elemCons: consNumD},
	{name: methodMax, result: opt(tT), elemCons: consNumD},
	{name: methodMinBy, params: []bparam{bp(paramF, fnOf(tKOrd, tT))}, result: opt(tT)},
	{name: methodMaxBy, params: []bparam{bp(paramF, fnOf(tKOrd, tT))}, result: opt(tT)},
}

// listMethods are the methods of plain lists only (STDLIB.md §4.1).
var listMethods = []bsig{
	{name: methodGet, params: []bparam{bp(paramI, types.IntType)}, result: opt(tT)},
}

// keyedMethods are STDLIB.md §5: tables and keyed lists; active() on tables only.
var keyedMethods = []bsig{
	{name: methodGet, params: []bparam{bp(paramK, keyT)}, result: opt(tT)},
	{name: methodFind, params: []bparam{bp(paramK, keyT)}, result: opt(tT)},
	{name: methodAt, params: []bparam{bp(paramI, types.IntType)}, result: tT},
	{name: methodKeys, result: listT(refT)},
	{name: methodValues, result: listT(tT)},
	{name: methodActive, result: listT(tT)},
}

// mapMethods are STDLIB.md §6.
var mapMethods = []bsig{
	{name: methodLen, result: types.IntType},
	{name: methodIsEmpty, result: types.BoolType},
	{name: methodKeys, result: listT(tK)},
	{name: methodValues, result: listT(tV)},
	{name: methodGet, params: []bparam{bp(paramK, tK)}, result: opt(tV)},
	{name: methodContains, params: []bparam{bp(paramK, tK)}, result: types.BoolType},
	{name: methodMap, params: []bparam{bp(paramF, fnOf(tU, tV))}, result: &types.MapType{Key: tK, Value: tU}},
	{name: methodFilter, params: []bparam{predKV}, result: &types.MapType{Key: tK, Value: tV}},
	{name: methodAny, params: []bparam{predKV}, result: types.BoolType},
	{name: methodAll, params: []bparam{predKV}, result: types.BoolType},
	{name: methodCount, params: []bparam{predKV}, result: types.IntType},
}

// stringMethods are STDLIB.md §7; matches takes a regex literal.
var stringMethods = []bsig{
	{name: methodLen, result: types.IntType},
	{name: methodIsEmpty, result: types.BoolType},
	{name: methodContains, params: []bparam{strS}, result: types.BoolType},
	{name: methodStartsWith, params: []bparam{strS}, result: types.BoolType},
	{name: methodEndsWith, params: []bparam{strS}, result: types.BoolType},
	{name: methodFind, params: []bparam{strS}, result: opt(types.IntType)},
	{name: methodSplit, params: []bparam{sepS}, result: listT(types.StringType)},
	{name: methodTrim, result: types.StringType},
	{name: methodLower, result: types.StringType},
	{name: methodUpper, result: types.StringType},
	{name: methodReplace, params: []bparam{bp(paramA, types.StringType), bp(paramB, types.StringType)}, result: types.StringType},
	{name: methodMatches, params: []bparam{bp(paramRe, types.StringType)}, result: types.BoolType},
}

// rangeMethods are STDLIB.md §10.
var rangeMethods = []bsig{
	{name: methodLen, result: types.IntType},
	{name: methodIsEmpty, result: types.BoolType},
	{name: methodContains, params: []bparam{bp(paramX, types.IntType)}, result: types.BoolType},
}

// freeFunctions are STDLIB.md §2 and §10: conversions, math, graphs, fail and warn.
var freeFunctions = []bsig{
	{name: typeInt, params: []bparam{bp(paramX, types.FloatType)}, result: types.IntType},
	{name: typeInt, params: []bparam{bp(paramX, types.IntType)}, result: types.IntType},
	{name: typeFloat, params: []bparam{bp(paramX, types.IntType)}, result: types.FloatType},
	{name: typeFloat, params: []bparam{bp(paramX, types.FloatType)}, result: types.FloatType},
	{name: typeString, params: []bparam{bp(paramX, tEq)}, result: types.StringType},
	{name: fnAbs, params: []bparam{bp(paramX, tNum)}, result: tNum},
	{name: fnMin, params: []bparam{bp(paramA, tNum), bp(paramB, tNum)}, result: tNum, variadic: true},
	{name: fnMax, params: []bparam{bp(paramA, tNum), bp(paramB, tNum)}, result: tNum, variadic: true},
	{name: fnClamp, params: []bparam{bp(paramX, tNum), bp(paramLo, tNum), bp(paramHi, tNum)}, result: tNum},
	{name: fnFloor, params: []bparam{bp(paramX, types.FloatType)}, result: types.IntType},
	{name: fnCeil, params: []bparam{bp(paramX, types.FloatType)}, result: types.IntType},
	{name: fnRound, params: []bparam{bp(paramX, types.FloatType)}, result: types.IntType},
	{name: fnSqrt, params: []bparam{bp(paramX, types.FloatType)}, result: types.FloatType},
	{name: fnPow, params: []bparam{bp(paramX, types.FloatType), bp(paramY, types.FloatType)}, result: types.FloatType},
	{name: fnReachable, params: []bparam{bp(paramFrom, tEq), bp(paramNext, fnOf(listT(tEq), tEq))}, result: listT(tEq)},
	{name: fnReachable, params: []bparam{bp(paramFrom, tEq), bp(paramNext, fnOf(opt(tEq), tEq))}, result: listT(tEq)},
	{name: fnCycles, params: []bparam{bp(paramXs, &seqOf{elem: tEq}), bp(paramNext, fnOf(listT(tEq), tEq))}, result: listT(tEq)},
	{name: fnCycles, params: []bparam{bp(paramXs, &seqOf{elem: tEq}), bp(paramNext, fnOf(opt(tEq), tEq))}, result: listT(tEq)},
	{name: fnTopoSort, params: []bparam{bp(paramXs, &seqOf{elem: tEq}), bp(paramNext, fnOf(listT(tEq), tEq))}, result: listT(tEq)},
	{name: fnTopoSort, params: []bparam{bp(paramXs, &seqOf{elem: tEq}), bp(paramNext, fnOf(opt(tEq), tEq))}, result: listT(tEq)},
	{name: fnFail, params: []bparam{bp(paramAt, types.AnyType), bp(paramMessage, types.StringType)}, result: types.NoneType},
	{name: fnWarn, params: []bparam{bp(paramAt, types.AnyType), bp(paramMessage, types.StringType)}, result: types.NoneType},
}

// builtinTypes are the predeclared type names (TYPES.md §3.3 step 6).
var builtinTypes = map[string]types.Type{
	typeInt: types.IntType, typeInt8: types.Int8Type, typeInt16: types.Int16Type, typeInt32: types.Int32Type,
	typeUInt8: types.UInt8Type, typeUInt16: types.UInt16Type, typeUInt32: types.UInt32Type, typeUInt64: types.UInt64Type,
	typeFloat: types.FloatType, typeFloat32: types.Float32Type, typeString: types.StringType, typeBool: types.BoolType,
	typeDuration: types.DurationType, typeRange: types.RangeType, typeNever: types.NeverType, typeDefine: types.DefineType,
}

// builtinMembers are the members read after `.` (STDLIB.md §3); an object each, for NameUses.
var builtinMembers = []string{idMember, retiredMember, kindMember, nameMember, indexMember, wireMember, codeMember, startMember, endMember, membersMember}

// newUniverse makes step 6's objects: built-in types, free functions, fail and warn.
func (c *checker) newUniverse() map[string]*object {
	u := map[string]*object{}
	for _, name := range slices.Sorted(maps.Keys(builtinTypes)) {
		o := c.newObject(ObjBuiltin, name, nil, nil, nil)
		o.typ = builtinTypes[name]
		u[name] = o
	}
	for _, s := range freeFunctions {
		if _, ok := u[s.name]; !ok {
			u[s.name] = c.newObject(ObjBuiltin, s.name, nil, nil, nil)
		}
	}
	for _, name := range builtinMembers {
		c.builtins[name] = c.newObject(ObjBuiltin, name, nil, nil, nil)
	}
	for _, name := range []string{falseWord, trueWord} {
		c.boolObjs = append(c.boolObjs, c.newObject(ObjBuiltin, name, nil, nil, nil))
	}
	for _, table := range [][]bsig{seqMethods, listMethods, keyedMethods, mapMethods, stringMethods, rangeMethods} {
		for _, s := range table {
			if _, ok := c.builtins[s.name]; !ok {
				c.builtins[s.name] = c.newObject(ObjBuiltin, s.name, nil, nil, nil)
			}
		}
	}
	c.tableOf[types.DefineType] = true
	for _, f := range types.DefineType.Fields {
		fo := c.newObject(ObjField, f.Name, nil, nil, nil)
		fo.field, fo.owner, fo.typ = f, types.DefineType, f.Type
		c.fieldObjects[f] = fo
	}
	return u
}

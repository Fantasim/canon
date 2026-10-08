package std

import "maps"

// methods are the method tables by receiver family.
var methods = [famCount]map[string]builtin{
	famList: listMethods(), famKeyed: keyedMethods(), famMap: mapMethods(),
	famString: stringMethods(), famRange: rangeMethods(),
}

// paramNames are the parameter names of each built-in, by family (STDLIB.md §2 to §7).
var paramNames = [famCount]map[string][]string{
	famFree: {
		bInt: {pX}, bFloat: {pX}, bString: {pX}, bAbs: {pX}, bMin: {pA, pB}, bMax: {pA, pB},
		bClamp: {pX, pLo, pHi}, bFloor: {pX}, bCeil: {pX}, bRound: {pX}, bSqrt: {pX},
		bPow: {pX, pY}, bReachable: {pFrom, pNext}, bCycles: {pXs, pNext}, bTopoSort: {pXs, pNext},
	},
	famList:   seqParams(map[string][]string{bGet: {pI}}),
	famKeyed:  seqParams(map[string][]string{bGet: {pK}, bFind: {pK}, bHasKey: {pK}, bAt: {pI}}),
	famMap:    {bGet: {pK}, bContains: {pK}, bMap: {pF}, bFilter: {pPred}, bAny: {pPred}, bAll: {pPred}, bCount: {pPred}, bUnion: {pB}},
	famString: {bContains: {pS}, bStartsWith: {pS}, bEndsWith: {pS}, bFind: {pS}, bSplit: {pSep}, bReplace: {pA, pB}, bMatches: {pRe}},
	famRange:  {bContains: {pX}},
}

// seqParams are the named parameters of Seq(T) with extra's added.
func seqParams(extra map[string][]string) map[string][]string {
	m := map[string][]string{
		bFirst: {pPred}, bContains: {pX}, bIndexOf: {pX}, bMap: {pF}, bFilter: {pPred},
		bFlatMap: {pF}, bSortBy: {pF}, bZip: {pOther}, bIntersect: {pB}, bUnion: {pB},
		bDiff: {pB}, bGroupBy: {pF}, bToMap: {pKeyF, pValF}, bJoin: {pSep}, bAny: {pPred},
		bAll: {pPred}, bCount: {pPred}, bMinBy: {pF}, bMaxBy: {pF},
	}
	maps.Copy(m, extra)
	return m
}

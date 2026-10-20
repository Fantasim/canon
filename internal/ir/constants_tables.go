package ir

import (
	resyntax "regexp/syntax"
	"strings"
	"unicode/utf8"
)

// The code points of `.` (all but `\n`: the Perl flags lack DotNL) and of any, as rune pairs.
var (
	anyRuneNotNL = []rune{0, '\n' - 1, '\n' + 1, utf8.MaxRune}
	anyRune      = []rune{0, utf8.MaxRune}
)

// instStates convert each instruction a reachable state can be, indexed by its op; nil is refused.
var instStates = [...]func(*automatonBuilder, *resyntax.Inst) PatternState{
	resyntax.InstMatch: (*automatonBuilder).accept, resyntax.InstFail: (*automatonBuilder).fail,
	resyntax.InstAlt: (*automatonBuilder).split, resyntax.InstEmptyWidth: (*automatonBuilder).anchor,
	resyntax.InstRune: (*automatonBuilder).runes, resyntax.InstRune1: (*automatonBuilder).rune1,
	resyntax.InstRuneAny: (*automatonBuilder).any, resyntax.InstRuneAnyNotNL: (*automatonBuilder).anyNotNL,
}

// patternEmpty maps Go's assertions to the ones an automaton keeps; any other (multi-line anchors, word boundaries) has no state.
var patternEmpty = []struct {
	op uint32
	at PatternAt
}{{uint32(resyntax.EmptyBeginText), PatternAtBegin}, {uint32(resyntax.EmptyEndText), PatternAtEnd}}

// The names of a TypeScript module that are not the package's (CODEGEN.md §3.4, §8.2): reserved words, predefined type names, and the helper units the generator may write, runtime block and decoders.
var (
	tsReserved   = strings.Fields("await break case catch class const continue debugger default delete do else enum export extends false finally for function if import in instanceof new null return super switch this throw true try typeof var void while with yield implements interface let package private protected public static undefined NaN Infinity arguments eval")
	tsPredefined = strings.Fields("string number boolean symbol bigint object any unknown never void")
	tsHelpers    = strings.Fields("CanonEvalError canonFail canonInt canonAdd canonSub canonMul canonDiv canonMod canonNeg canonAbs canonClamp canonF canonMinF canonMaxF canonClampF canonToInt canonFloor canonCeil canonRound canonDivDuration canonCheckRange canonCheckWidth canonF32 CanonMap CanonTable canonTable CanonRow canonFreeze canonList canonIndex canonEnvelope decOrder decTokenKey decToken decText decDecimal decSame decKeys decFail decEntry decGet decIsObject decObject decArray decAt decEmptyObject decEmptyArray decBool decBit decInt decBig decFloat decCompare decHalf decF32 decString decDuration decEnum decCode decBits decKeyed decList decTable decIntKey decBigKey decMap decPairs decParse")
)

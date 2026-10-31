package ir

import "github.com/fantasim/canonlang/internal/types"

// The words of a bound constant's name after <F> (CODEGEN.md §5.15): each step, Len for a length, the side.
var (
	boundStepWords = [...]string{StepElem: boundElem, StepKey: boundKey, StepValue: boundValue}
	boundSideWords = [...]string{SideMin: boundMin, SideMax: boundMax, SideEnd: boundEnd}
)

// rangeFailures is the decoder's failure prefix by the kind a range refines, "" for another (CODEGEN.md §5.15).
var rangeFailures = [types.Error + 1]string{
	types.Int: failValue, types.Float: failValue, types.Duration: failValue, types.String: failBytes,
	types.List: failCount, types.Map: failCount, types.DepMap: failCount,
}

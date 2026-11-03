package ir_test

import "github.com/fantasim/canonlang/internal/diag"

// mxAllowance is a refusal of Canon's check the matrix accepts: why, the diagnostic code it must come with, and the positions it applies to. Any other refusal fails TestCppRefMatrix.
type mxAllowance struct {
	reason  string
	code    diag.Code
	applies func(c mxCase, p mxPos) bool
}

// mxAllowed are the refusals the matrix accepts, each pinned by its code, not by its words.
var mxAllowed = []mxAllowance{
	{"a data emit with @reload refuses a dependent value decided through a ref into a @reload list", diag.E8019.Def().Code, func(c mxCase, p mxPos) bool { return c.mode == mxData && p.code == "d" }},
	{"a literal union is a String, and no Int key is", diag.E3002.Def().Code, func(c mxCase, p mxPos) bool { return c.leaf.code == "in" && p.code == "u" }},
}

// mxPackages is the number of .gen.cpp the matrix compiles, the library's included.
const mxPackages = 118

// mxPinned are the positions each case keeps, by case: the others are refused by an allowance. A moved line names the case.
const mxPinned = `
mxew1bk flmud
mxew1da flmu
mxew1ds flmud
mxew1ty flmud
mxew1rd flmud
mxew2bk flmud
mxew2da flmu
mxew2ds flmud
mxew2ty flmud
mxew2rd flmud
mxew3bk flmud
mxew3da flmu
mxew3ds flmud
mxew3ty flmud
mxew3rd flmud
mxec1bk flmud
mxec1da flmu
mxec1ds flmud
mxec1ty flmud
mxec1rd flmud
mxec2bk flmud
mxec2da flmu
mxec2ds flmud
mxec2ty flmud
mxec2rd flmud
mxec3bk flmud
mxec3da flmu
mxec3ds flmud
mxec3ty flmud
mxec3rd flmud
mxst1bk flmud
mxst1da flmu
mxst1ds flmud
mxst1ty flmud
mxst1rd flmud
mxst2bk flmud
mxst2da flmu
mxst2ds flmud
mxst2ty flmud
mxst2rd flmud
mxst3bk flmud
mxst3da flmu
mxst3ds flmud
mxst3ty flmud
mxst3rd flmud
mxin1bk flmd
mxin1da flm
mxin1ds flmd
mxin1ty flmd
mxin1rd flmd
mxin2bk flmd
mxin2da flm
mxin2ds flmd
mxin2ty flmd
mxin2rd flmd
mxin3bk flmd
mxin3da flm
mxin3ds flmd
mxin3ty flmd
mxin3rd flmd
mxot1bk flmud
mxot1rd flmud
mxot2bk flmud
mxot2rd flmud
mxot3bk flmud
mxot3rd flmud
mxft1bk flmud
mxft1da flmud
mxft1ds flmud
mxft1ty flmud
mxft1rd flmud
mxft2bk flmud
mxft2da flmu
mxft2ds flmud
mxft2ty flmud
mxft2rd flmud
mxft3bk flmud
mxft3da flmu
mxft3ds flmud
mxft3ty flmud
mxft3rd flmud
`

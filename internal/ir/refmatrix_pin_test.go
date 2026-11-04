package ir_test

import "github.com/fantasim/canonlang/internal/diag"

// mxAllowance is a refusal of Canon's check the matrix accepts: why, the diagnostic code it must come with, the kind of the code (its word in the message; "" for a code without kinds) and the positions it applies to. Any other refusal fails TestCppRefMatrix.
type mxAllowance struct {
	reason  string
	code    diag.Code
	kind    string
	applies func(c mxCase, p mxPos) bool
}

// mxAllowed are the refusals the matrix accepts, each pinned by its code (and, for E8019, its kind DependentType), not by its words.
var mxAllowed = []mxAllowance{
	{"a data emit with @reload refuses a dependent value decided through a ref into a @reload list", diag.E8019.Def().Code, diag.KindDependentType.Word(), func(c mxCase, p mxPos) bool { return c.mode == mxData && p.code == "d" }},
	{"a literal union is a String, and no Int key is", diag.E3002.Def().Code, "", func(c mxCase, p mxPos) bool { return c.leaf.code == "in" && p.code == "u" }},
}

// mxPendingProbe are the positions a case holds back in packages of their own (tagged mxPendingTag), by case: each is a bug of gen/cpp's, so the C++ matrix expects the probe's packages on mxCppPending to fail and compiles every other position of the case strictly. The Go matrix has no pending position and compiles the probe like any other.
var mxPendingProbe = map[string]string{}

// mxPendingTag tells a pending probe's packages from the case's other packages.
const mxPendingTag = "p"

// mxCppPending are the packages of those probes that gen/cpp does not compile yet; the test fails when one compiles, so an entry goes with its fix. None is pending.
var mxCppPending = []string{}

// mxPackages is the number of .gen.cpp the matrix compiles, the libraries' included, a pending probe's among them.
const mxPackages = 149

// mxPinned are the positions each case keeps, by case: the others are refused by an allowance. A moved line names the case.
const mxPinned = `
mxew1bk flmudo
mxew1da flmuo
mxew1ds flmudo
mxew1ty flmudo
mxew1rd flmudo
mxew2bk flmudo
mxew2da flmuo
mxew2ds flmudo
mxew2ty flmudo
mxew2rd flmudo
mxew3bk flmudo
mxew3da flmuo
mxew3ds flmudo
mxew3ty flmudo
mxew3rd flmudo
mxec1bk flmudo
mxec1da flmuo
mxec1ds flmudo
mxec1ty flmudo
mxec1rd flmudo
mxec2bk flmudo
mxec2da flmuo
mxec2ds flmudo
mxec2ty flmudo
mxec2rd flmudo
mxec3bk flmudo
mxec3da flmuo
mxec3ds flmudo
mxec3ty flmudo
mxec3rd flmudo
mxst1bk flmudo
mxst1da flmuo
mxst1ds flmudo
mxst1ty flmudo
mxst1rd flmudo
mxst2bk flmudo
mxst2da flmuo
mxst2ds flmudo
mxst2ty flmudo
mxst2rd flmudo
mxst3bk flmudo
mxst3da flmuo
mxst3ds flmudo
mxst3ty flmudo
mxst3rd flmudo
mxin1bk flmdo
mxin1da flmo
mxin1ds flmdo
mxin1ty flmdo
mxin1rd flmdo
mxin2bk flmdo
mxin2da flmo
mxin2ds flmdo
mxin2ty flmdo
mxin2rd flmdo
mxin3bk flmdo
mxin3da flmo
mxin3ds flmdo
mxin3ty flmdo
mxin3rd flmdo
mxot1bk flmudo
mxot1rd flmudo
mxot2bk flmudo
mxot2rd flmudo
mxot3bk flmudo
mxot3rd flmudo
mxft1bk flmudo
mxft1da flmudo
mxft1ds flmudo
mxft1ty flmudo
mxft1rd flmudo
mxft2bk flmudo
mxft2da flmuo
mxft2ds flmudo
mxft2ty flmudo
mxft2rd flmudo
mxft3bk flmudo
mxft3da flmuo
mxft3ds flmudo
mxft3ty flmudo
mxft3rd flmudo
mxfd1bk flmudo
mxfd1da flmudo
mxfd1ds flmudo
mxfd1ty flmudo
mxfd1rd flmudo
mxfd2bk flmudo
mxfd2da flmuo
mxfd2ds flmudo
mxfd2ty flmudo
mxfd2rd flmudo
mxfd3bk flmudo
mxfd3da flmuo
mxfd3ds flmudo
mxfd3ty flmudo
mxfd3rd flmudo
mxos1da flmudo
mxos1ds flmudo
mxos1ty flmudo
mxos2da flmuo
mxos2ds flmudo
mxos2ty flmudo
mxos3da flmuo
mxos3ds flmudo
mxos3ty flmudo
`

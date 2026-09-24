package cppgen_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	// commaLocaleName is buildCommaLocale's locale, whose decimal point is ','.
	commaLocaleName = "canon_comma"
	// asciiCharmap is the charmap the comma locale is compiled with: every glibc install has it.
	asciiCharmap = "ANSI_X3.4-1968"
	weightKey    = `"weight": `
	weightOld    = weightKey + "3.25"
	// midpoint1 is 1 + 2^-24 written out exactly: halfway between the float32 values 1 and 1 + 2^-23.
	midpoint1 = "1.000000059604644775390625"
	// aboveMidpoint1 is the float32round case's token, just above midpoint1.
	aboveMidpoint1 = "1.000000059604644775400625"
	// f32One, f32Up and f32Max are strict_main's weight for 1, 1 + 2^-23 and FLT_MAX.
	f32One = "+1"
	f32Up  = "+1.00000012"
	f32Max = "+3.40282347e+38"
	// fitsFloat32 is the refusal of sword's weight.
	fitsFloat32 = "rows[0].weight: expected a number that fits Float32"
	// noToken is the internal error of a midpoint read with no token on record.
	noToken = "internal error: no token on record for a Float32 rounding midpoint"
	// sixtyDigitZeros pads midpoint1 (25 digits) to a 60-digit token with a final 1.
	sixtyDigitZeros = 34
	// subnormalMid is 2^-150 written out exactly: halfway between +0 and the smallest float32.
	subnormalMid = "7.00649232162408535461864791644958065640130970938257885878534141944895541342930300743319094181060791015625e-46"
	// subnormalAbove is just above subnormalMid (testdata/main/nest/good's $scale.small).
	subnormalAbove = "7.006492321624085354618647916449580656401309709382578858785341419448955413429303007433190941810607910156251e-46"
	// circleR is testdata/main/nest/good's shape.r: just above 2 + 2^-23, a midpoint.
	circleR = "2.000000119209289550781251"
)

// swordDump is strict_main's dump of the good items.json and config.json, sword's weight read as weight.
func swordDump(weight string) string {
	return "sword:code=300,price=+12.5,weight=" + weight + ",bonus=none,legacyMax=9 " + goodAnvilAir + " " + goodConfig
}

// weightCase writes tok as sword's weight: the load dumps weight, or fails with want when want is set.
func weightCase(name, rule, tok, weight, want string) strictCase {
	dump := ""
	if want == "" {
		dump = swordDump(weight)
	}
	return strictCase{name, rule, itemsFile, weightOld, weightKey + tok, want, dump, shopFiles}
}

// float32Cases are log-2026-09-24 A1 C++ Float32's rule (WIRE.md §3.3, §5.1): a Float32 is rounded once from the exact decimal, ties to even, overflow judged on the exact decimal, whatever the process locale.
func float32Cases() []strictCase {
	const rule = "WIRE.md §3.3, §5.1, log-2026-09-24 A1 C++ Float32"
	cases := []strictCase{
		weightCase("f32below", rule+" (just below a midpoint)", "1.000000059604644775380625", f32One, ""),
		weightCase("f32tie", rule+" (a midpoint exactly: ties to even)", midpoint1, f32One, ""),
		weightCase("f32sixty", rule+" (60 digits, just above a midpoint)", midpoint1+strings.Repeat("0", sixtyDigitZeros)+"1", f32Up, ""),
		weightCase("f32expabove", rule+" (exponent-heavy, above)", "0.0000000001000000059604644775390625000001e10", f32Up, ""),
		weightCase("f32expbelow", rule+" (exponent-heavy, below)", "1000000059604644775390624E-24", f32One, ""),
		weightCase("f32negabove", rule+" (negative, above in magnitude)", "-"+aboveMidpoint1, "-1.00000012", ""),
		weightCase("f32int", rule+" (an integer token rounds once)", "1152921573326323713", "+1.15292164e+18", ""),
		weightCase("f32maxdec", rule+" (below the overflow boundary by the exact decimal)", "3.4028235677973366e38", f32Max, ""),
		weightCase("f32maxint", rule+" (below the overflow boundary, integer digits)", "340282356779733661637539395458142568447", f32Max, ""),
		weightCase("f32negmax", rule+" (negative, below the overflow boundary)", "-3.4028235677973366e38", "-3.40282347e+38", ""),
		weightCase("f32overtie", rule+" (the overflow boundary exactly)", "340282356779733661637539395458142568448", "", fitsFloat32),
		weightCase("f32overexp", rule+" (just above the overflow boundary)", "3.4028235677973366163753939545814256845e38", "", fitsFloat32),
		weightCase("f32subabove", rule+" (just above the smallest midpoint)", subnormalAbove, "+1.40129846e-45", ""),
		weightCase("f32subtie", rule+" (the smallest midpoint: ties to +0)", subnormalMid, "+0", ""),
		{"commagood", rule + " (',' locale)", itemsFile, weightOld, weightOld, "", swordDump("+3.25"), commaLocale},
		{"commamid", rule + " (',' locale, a midpoint token)", itemsFile, weightOld, weightKey + aboveMidpoint1, "", swordDump(f32Up), commaLocale},
	}
	return append(cases, nestFloat32Cases(rule)...)
}

// nestFloat32Cases refuse a Float32 in a list element, a dotted wire name, a variant case field and a $fn cell of demo.nest.
func nestFloat32Cases(rule string) []strictCase {
	const fits = ": expected a number that fits Float32"
	return []strictCase{
		{"nestf32list", rule + " (list element)", nestValueFile, nestMaxOld, "3.4028235677973367e38]", "value.f32s[3]" + fits, "", nestFile},
		{"nestf32dotted", rule + " (dotted wire name)", nestValueFile, `"a.b": ` + aboveMidpoint1, `"a.b": 1e39`, "value.a.b" + fits, "", nestFile},
		{"nestf32case", rule + " (variant case field)", nestValueFile, `"r": ` + circleR, `"r": -1e39`, "value.shape.r" + fits, "", nestFile},
		{"nestf32cell", rule + " ($fn cell)", nestValueFile, `"small": ` + subnormalAbove, `"small": 1e39`, "value.$scale.small" + fits, "", nestFile},
	}
}

// withoutMode is cases less those loaded in mode m.
func withoutMode(cases []strictCase, m caseMode) []strictCase {
	var kept []strictCase
	for _, c := range cases {
		if c.mode != m {
			kept = append(kept, c)
		}
	}
	return kept
}

// buildCommaLocale compiles testdata/main/comma.locale into a temporary locale directory, the
// LOCPATH under which the C++ driver finds commaLocaleName; an error when localedef cannot.
func buildCommaLocale(t *testing.T) (string, error) {
	t.Helper()
	dir := t.TempDir()
	def, err := filepath.Abs(filepath.Join("testdata", "main", "comma.locale"))
	if err != nil {
		return "", err
	}
	// localedef -c exits non-zero over the categories the definition leaves out, yet writes them.
	out, runErr := exec.Command("localedef", "-c", "-i", def, "-f", asciiCharmap, filepath.Join(dir, commaLocaleName)).CombinedOutput()
	if _, err := os.Stat(filepath.Join(dir, commaLocaleName, "LC_NUMERIC")); err != nil {
		return "", errors.Join(runErr, errors.New(string(out)), err)
	}
	return dir, nil
}

// log-2026-09-24 A1 C++ Float32: a midpoint read with no token on record fails loudly; a Decoder takes no temporary token table.
func TestFloat32MidpointWithoutToken(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated C++")
	}
	dir := t.TempDir()
	writeTree(t, dir, generate(t, nestPackage()), "f32_main.cpp")
	want := "3f800001\ng.json: value.w: " + noToken + "\nf.json: value.w: " + noToken + "\n"
	for _, out := range buildAndRun(t, dir, []string{"main.cpp"}) {
		if out != want {
			t.Errorf("got\n%swant\n%s", out, want)
		}
	}
}

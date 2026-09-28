package viewgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// canonicalNumber refuses a vm.Number's text unless it is WIRE.md 7.2's one form: quoted, it
// must be VIEWMODEL.md J10's decimal-string integer beyond the safe range (a Float is never
// quoted); unquoted, canonicalUnquotedNumber decides.
func canonicalNumber(text string, quoted bool) error {
	if !quoted {
		return canonicalUnquotedNumber(text)
	}
	if hasFraction(text) {
		return fmt.Errorf("%w: quoted float %s", errNumberText, text)
	}
	if err := canonicalIntegerText(text); err != nil {
		return err
	}
	if withinSafeRange(text) {
		return fmt.Errorf("%w: quoted in range %s", errNumberText, text)
	}
	return nil
}

// canonicalUnquotedNumber refuses a bare number's text unless it is a canonical integer in
// the safe range, a canonical float, or — beyond the range, with neither `.` nor an exponent —
// a Float whose own ECMAScript text (WIRE.md 7.2 step 4) happens to be that digit string.
func canonicalUnquotedNumber(text string) error {
	if hasFraction(text) {
		return canonicalFloatText(text)
	}
	if err := canonicalIntegerText(text); err != nil {
		return err
	}
	if withinSafeRange(text) {
		return nil
	}
	return canonicalFloatText(text)
}

// canonicalIntegerText refuses text unless it is WIRE.md 7.2's one form for an integer.
func canonicalIntegerText(text string) error {
	if !isCanonicalInteger(text) {
		return fmt.Errorf("%w: %s", errNumberText, text)
	}
	return nil
}

// isCanonicalInteger is WIRE.md 7.2's integer grammar: an optional `-`, then `0` alone or a
// nonzero-leading digit run; `-0` refuses (only a float writes `0` for a negative zero).
func isCanonicalInteger(text string) bool {
	s, neg := text, false
	if after, ok := strings.CutPrefix(text, "-"); ok {
		s, neg = after, true
	}
	switch {
	case s == "":
		return false
	case s == "0":
		return !neg
	case s[0] < '1' || s[0] > '9':
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// canonicalFloatText refuses text unless it already is the ECMAScript shortest form Write
// would otherwise compute (WIRE.md 7.2); Write never rewrites it.
func canonicalFloatText(text string) error {
	x, err := strconv.ParseFloat(text, floatBits)
	if err != nil || types.FloatText(x, floatBits) != text {
		return fmt.Errorf("%w: %s", errNumberText, text)
	}
	return nil
}

// withinSafeRange reports a canonical integer's magnitude at or under maxSafeIntText.
func withinSafeRange(text string) bool {
	digits := strings.TrimPrefix(text, "-")
	switch {
	case len(digits) < len(maxSafeIntText):
		return true
	case len(digits) > len(maxSafeIntText):
		return false
	default:
		return digits <= maxSafeIntText
	}
}

// hasFraction reports a decimal point or exponent marker in a JSON number's text.
func hasFraction(text string) bool {
	for i := range len(text) {
		switch text[i] {
		case '.', 'e', 'E':
			return true
		}
	}
	return false
}

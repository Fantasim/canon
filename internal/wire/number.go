package wire

import (
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// NumberText is the canonical text of the number token tok that the decoder read into v, whose
// provenance is tok; false when v is no numeric reading of tok.
func NumberText(v value.Value, tok string) (string, bool) {
	switch x := v.(type) {
	case *value.Float:
		return floatText(x, tok)
	case *value.Dur:
		return countText(x.Ms, tok)
	case *value.Int, *value.Member, *value.Bool, *value.List, *value.Ref:
		return integerText(parseDecimal(tok))
	}
	return "", false
}

// floatText is the shortest round trip of tok at x's width; false when tok does not read as x.
func floatText(x *value.Float, tok string) (string, bool) {
	bits := floatBits(x.T)
	f, err := strconv.ParseFloat(tok, bits)
	if err != nil || f != x.V {
		return "", false
	}
	return types.FloatText(f, bits), true
}

// countText is tok's integer count of the unit a Duration of ms was read in, tok itself for a
// count that is not whole; false when tok is no count of ms in any unit.
func countText(ms int64, tok string) (string, bool) {
	d := parseDecimal(tok)
	for _, u := range types.DurationUnits() {
		if got, whole, inRange := d.times(u.Millis()); whole && inRange && got == ms {
			if text, ok := integerText(d); ok {
				return text, true
			}
			return tok, true
		}
	}
	return "", false
}

// integerText is d in decimal digits when it is an integer an int64 can hold.
func integerText(d decimal) (string, bool) {
	if d.digits == "" {
		return textZero, true
	}
	if d.exp < 0 || int64(len(d.digits))+d.exp > maxMillisDigits {
		return "", false
	}
	text := d.digits + strings.Repeat(textZero, int(d.exp))
	if d.neg {
		text = minusSign + text
	}
	return text, true
}

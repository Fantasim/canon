package wire

import (
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// boolean is true / false, or the tokens 0 / 1 under @json(int) (WIRE.md §5.1, §5.2).
func (r *run) boolean(sel Selection, t types.Type, sc wscope) value.Value {
	n := sel.Node
	switch {
	case sc.asInt && isContainer(n):
		return r.mismatch(n, diag.KindNumber, t)
	case sc.asInt && n.Kind == jsonsrc.Number && (n.Text == textZero || n.Text == textOne):
		return &value.Bool{V: n.Text == textOne, P: prov(n)}
	case sc.asInt:
		r.report(diag.E7110.AtInt(n.Span, jsonText(n)), n)
		return nil
	case n.Kind != jsonsrc.Bool:
		return r.mismatch(n, diag.KindBoolean, t)
	}
	return &value.Bool{V: n.Text == textTrue, P: prov(n)}
}

// integer is a number token without fraction or exponent, in t's range (WIRE.md §5.1).
func (r *run) integer(sel Selection, t types.Type, _ wscope) value.Value {
	n := sel.Node
	if n.Kind != jsonsrc.Number {
		return r.mismatch(n, diag.KindNumber, t)
	}
	i, ok := r.intToken(n, t)
	if !ok {
		return nil
	}
	return &value.Int{V: i, T: t, P: prov(n)}
}

// intToken is n's integer, E7103 when the token is not one, E3201 outside t's range.
func (r *run) intToken(n *jsonsrc.Node, t types.Type) (int64, bool) {
	if strings.ContainsAny(n.Text, fractionChars) {
		r.report(diag.E7103.AtNumber(n.Span, n.Text), n)
		return 0, false
	}
	i, err := strconv.ParseInt(n.Text, decimalBase, float64Bits)
	switch {
	case err != nil:
		r.report(diag.E3201.At(n.Span, literal(n.Text), t), n)
		return 0, false
	case !fits(i, t):
		return i, r.soft(diag.E3201.At(n.Span, literal(n.Text), t), n)
	}
	return i, true
}

// fits reports whether i is in the implicit range of t's integer or Duration base (TYPES §7.2).
func fits(i int64, t types.Type) bool {
	b, ok := t.Base().(types.Basic)
	if !ok {
		return true
	}
	lo, hi, bounded := b.Limits()
	return !bounded || lo <= i && i <= hi
}

// float is any number token rounded once to t's width; an overflow is E3202 (WIRE.md §5.1).
func (r *run) float(sel Selection, t types.Type, _ wscope) value.Value {
	n := sel.Node
	if n.Kind != jsonsrc.Number {
		return r.mismatch(n, diag.KindNumber, t)
	}
	x, ok := r.floatOf(at(n), n.Text, t)
	if !ok {
		return nil
	}
	return &value.Float{V: x, T: t, P: prov(n)}
}

// floatOf rounds the decimal text to t's width: ParseFloat rounds the exact decimal once.
func (r *run) floatOf(s site, text string, t types.Type) (float64, bool) {
	bits := floatBits(t)
	x, _ := strconv.ParseFloat(text, bits)
	if math.IsInf(x, 0) {
		if wide, _ := strconv.ParseFloat(text, float64Bits); r.d.Keep && !math.IsInf(wide, 0) {
			return wide, r.soft(overflowFinding(s, text, bits), s.node) // a Float32 overflow, kept whole
		}
		r.overflow(s, text, bits)
		return 0, false
	}
	if x == 0 {
		return 0, true
	}
	return x, true
}

// overflow is E3202 for a number beyond the largest finite Float or Float32.
func (r *run) overflow(s site, text string, bits int) {
	r.report(overflowFinding(s, text, bits), s.node)
}

// overflowFinding is E3202 for a number beyond the largest finite float of bits.
func overflowFinding(s site, text string, bits int) *diag.Builder {
	if bits == float32Bits {
		return diag.E3202.AtFloat32(s.span, literal(text))
	}
	return diag.E3202.AtNonFinite(s.span, literal(text))
}

func (r *run) str(sel Selection, t types.Type, _ wscope) value.Value {
	n := sel.Node
	if n.Kind != jsonsrc.String {
		return r.mismatch(n, diag.KindString, t)
	}
	return &value.Str{V: n.Text, T: t, P: prov(n)}
}

// duration is any number of the field's unit that makes whole milliseconds (WIRE.md §5.1).
func (r *run) duration(sel Selection, t types.Type, sc wscope) value.Value {
	n := sel.Node
	if n.Kind != jsonsrc.Number {
		return r.mismatch(n, diag.KindNumber, t)
	}
	ms, ok := r.millis(at(n), n.Text, parseDecimal(n.Text), sc.unit, t)
	if !ok {
		return nil
	}
	return &value.Dur{Ms: ms, P: prov(n)}
}

// millis is d units in ms: E3203 when not whole, E3201 past the Duration range (TYPES §7.2).
func (r *run) millis(s site, text string, d decimal, unit types.Unit, t types.Type) (int64, bool) {
	ms, whole, inRange := d.times(unit.Millis())
	switch {
	case !inRange:
		r.report(diag.E3201.At(s.span, literal(text), t), s.node)
	case !whole:
		r.report(diag.E3203.At(s.span, text, unit.String()), s.node)
	default:
		return ms, true
	}
	return 0, false
}

// enum is a member's wire string, or its code under @json(codes) (WIRE.md §5.3).
func (r *run) enum(sel Selection, t types.Type, _ wscope) value.Value {
	n, e := sel.Node, t.Base().(*types.EnumType)
	if e.WireCodes {
		if n.Kind != jsonsrc.Number {
			return r.mismatch(n, diag.KindNumber, t)
		}
		if strings.ContainsAny(n.Text, fractionChars) {
			r.report(diag.E7103.AtNumber(n.Span, n.Text), n)
			return nil
		}
		return r.memberByCode(at(n), n.Text, e)
	}
	if n.Kind != jsonsrc.String {
		return r.mismatch(n, diag.KindString, t)
	}
	return r.memberByWire(at(n), n.Text, e)
}

// memberByWire is the member whose wire value is s, byte for byte, else E7111.
func (r *run) memberByWire(s site, text string, e *types.EnumType) value.Value {
	for i, m := range e.Members {
		if m.Wire == text {
			return &value.Member{Enum: e, Index: i, P: s.prov()}
		}
	}
	name := r.name(e.Pkg, e.Name)
	if hint := memberHint(text, e); hint != "" {
		r.report(diag.E7111.AtHint(s.span, text, name, hint), s.node)
		return nil
	}
	r.report(diag.E7111.AtMember(s.span, text, name), s.node)
	return nil
}

// memberByCode is the member whose code is the integer text, else E7111.
func (r *run) memberByCode(s site, text string, e *types.EnumType) value.Value {
	code, err := strconv.ParseInt(text, decimalBase, float64Bits)
	if i := codeIndex(e, code); err == nil && i >= 0 {
		return &value.Member{Enum: e, Index: i, P: s.prov()}
	}
	if !codesUnique(e) {
		return nil
	}
	r.report(diag.E7111.AtMember(s.span, text, r.name(e.Pkg, e.Name)), s.node)
	return nil
}

// codesUnique is false when two members hold one code, E3102 already: no second finding (TYPES.md §9.1).
func codesUnique(e *types.EnumType) bool {
	seen := map[int64]bool{}
	for _, m := range e.Members {
		if !m.HasCode {
			continue
		}
		if seen[m.Code] {
			return false
		}
		seen[m.Code] = true
	}
	return true
}

// memberHint is the wire value of the member named s, else the wire value nearest to s
// within hintDistance edits and shorter than s; ties go to the smallest in byte order.
func memberHint(s string, e *types.EnumType) string {
	best, bestD := "", hintDistance+1
	for _, m := range e.Members {
		if m.Name == s {
			return m.Wire
		}
		d := editDistance(s, m.Wire)
		if d < len(s) && (d < bestD || d == bestD && m.Wire < best) {
			best, bestD = m.Wire, d
		}
	}
	return best
}

// editDistance is the Levenshtein distance of a and b, in bytes.
func editDistance(a, b string) int {
	row := make([]int, len(b)+1)
	for j := range row {
		row[j] = j
	}
	for i := range len(a) {
		corner, left := row[0], i+1
		row[0] = left
		for j := range len(b) {
			cost := 1
			if a[i] == b[j] {
				cost = 0
			}
			corner, row[j+1] = row[j+1], min(row[j+1]+1, left+1, corner+cost)
			left = row[j+1]
		}
	}
	return row[len(b)]
}

// bits is a @json(bits) list: an integer mask whose set bits are member codes (WIRE.md §5.3).
func (r *run) bits(sel Selection, t types.Type, e *types.EnumType) value.Value {
	n := sel.Node
	if n.Kind != jsonsrc.Number {
		return r.mismatch(n, diag.KindNumber, t)
	}
	mask, ok := r.maskToken(n, e)
	if !ok {
		return nil
	}
	p := prov(n)
	l := &value.List{T: t, P: p}
	for bit := int64(1); bit > 0 && bit <= mask; bit <<= 1 {
		if i := codeIndex(e, bit); i >= 0 && mask&bit != 0 {
			l.Elems = append(l.Elems, &value.Member{Enum: e, Index: i, P: p})
			mask &^= bit
		}
	}
	if mask != 0 {
		if codesUnique(e) {
			r.report(diag.E7111.AtBits(n.Span, mask, r.name(e.Pkg, e.Name)), n)
		}
		return nil
	}
	return l
}

// maskToken is a bitmask: an integer, not negative, within Int, where every code is (WIRE.md §4.1).
func (r *run) maskToken(n *jsonsrc.Node, e *types.EnumType) (int64, bool) {
	if strings.ContainsAny(n.Text, fractionChars) {
		r.report(diag.E7103.AtNumber(n.Span, n.Text), n)
		return 0, false
	}
	digits, negative := strings.CutPrefix(n.Text, minusSign)
	var mask int64
	var err error
	if len(digits) <= maxIntDigits {
		mask, err = strconv.ParseInt(digits, decimalBase, float64Bits)
	}
	switch {
	case negative && digits != textZero:
		r.report(diag.E7110.AtBits(n.Span, n.Text), n)
	case len(digits) > maxIntDigits || err != nil:
		r.report(diag.E7111.AtMember(n.Span, n.Text, r.name(e.Pkg, e.Name)), n)
	default:
		return mask, true
	}
	return 0, false
}

// codeIndex is the first member whose code is c, or -1.
func codeIndex(e *types.EnumType, c int64) int {
	for i, m := range e.Members {
		if m.HasCode && m.Code == c {
			return i
		}
	}
	return -1
}

// litUnion is one of the literals, which wins over Of's reading, else a value of Of (TYP-09).
func (r *run) litUnion(sel Selection, t types.Type, sc wscope) value.Value {
	u, n := t.Base().(*types.LitUnionType), sel.Node
	if n.Kind == jsonsrc.String && isLiteral(u, n.Text) {
		return &value.Str{V: n.Text, T: t, P: prov(n)}
	}
	return r.value(sel, u.Of, sc)
}

func isLiteral(u *types.LitUnionType, s string) bool { return slices.Contains(u.Literals, s) }

// ref is a key of its target, bound to the enclosing instance of a field target (WIRE.md §5.9).
func (r *run) ref(sel Selection, t types.Type, sc wscope) value.Value {
	rt := t.Base().(*types.RefType)
	key, ok := r.refKey(sel, rt, sc)
	if !ok {
		return nil
	}
	return &value.Ref{T: t, Key: key, Owner: r.owner(rt.Target), P: prov(sel.Node)}
}

// refKey reads a ref's key: a table's entry key is a string, a keyed list's is its key field.
func (r *run) refKey(sel Selection, rt *types.RefType, sc wscope) (value.Key, bool) {
	if rt.Target == nil || rt.Target.KeyedBy == nil {
		n := sel.Node
		if n.Kind != jsonsrc.String {
			r.mismatch(n, diag.KindString, rt)
			return value.Key{}, false
		}
		return value.Key{S: n.Text}, true
	}
	return keyOfValue(r.value(sel, rt.Target.KeyedBy.Type, wscope{fr: sc.fr}))
}

// keyOfValue is the entry key a key field's value makes (TYPES.md §9.1).
func keyOfValue(v value.Value) (value.Key, bool) {
	switch x := v.(type) {
	case *value.Str:
		return value.Key{S: x.V}, true
	case *value.Int:
		return value.Key{I: x.V, IsInt: true}, true
	case *value.Member:
		return value.Key{S: x.Enum.Members[x.Index].Name}, true
	case *value.Ref:
		return x.Key, true
	}
	return value.Key{}, false
}

// jsonText is a scalar's JSON text as a finding quotes it: a string written as JSON.
func jsonText(n *jsonsrc.Node) string {
	if n.Kind == jsonsrc.String {
		return string(diag.AppendJSONString(nil, n.Text))
	}
	return n.Text
}

func isContainer(n *jsonsrc.Node) bool {
	return n.Kind == jsonsrc.Array || n.Kind == jsonsrc.Object
}

// decimal is an exact number, ±digits × 10^exp, digits trimmed of zeros (WIRE.md §3.3).
type decimal struct {
	neg    bool
	digits string
	exp    int64
}

// parseDecimal reads a JSON number token, or a Canon literal stripped of its underscores.
func parseDecimal(tok string) decimal {
	var d decimal
	tok, d.neg = strings.CutPrefix(tok, minusSign)
	mant, exp := tok, ""
	if i := strings.IndexAny(tok, exponentChars); i >= 0 {
		mant, exp = tok[:i], tok[i+1:]
	}
	whole, frac, _ := strings.Cut(mant, pointSep)
	digits := strings.TrimLeft(whole+frac, textZero)
	d.digits = strings.TrimRight(digits, textZero)
	d.exp = exponent(exp) - int64(len(frac)) + int64(len(digits)-len(d.digits))
	if d.digits == "" {
		return decimal{}
	}
	return d
}

// exponent is a signed decimal exponent, saturated at maxExponent, far past any exact range.
func exponent(s string) int64 {
	s, neg := strings.CutPrefix(s, minusSign)
	s = strings.TrimPrefix(s, plusSign)
	var e int64
	for i := range len(s) {
		e = min(e*decimalBase+int64(s[i]-digitZero), maxExponent)
	}
	if neg {
		return -e
	}
	return e
}

// times is d × unit as an integer: whether it is whole, and whether it is in the Duration range.
func (d decimal) times(unit int64) (ms int64, whole, inRange bool) {
	size := int64(len(d.digits)) + d.exp
	switch {
	case d.digits == "":
		return 0, true, true
	case size > maxMillisDigits:
		return 0, false, false
	case -d.exp > maxScale:
		return 0, false, true
	}
	n, _ := new(big.Int).SetString(d.digits, decimalBase)
	n.Mul(n, big.NewInt(unit))
	scale := new(big.Int).Exp(big.NewInt(decimalBase), big.NewInt(max(d.exp, -d.exp)), nil)
	if d.exp >= 0 {
		n.Mul(n, scale)
	} else if _, rem := n.QuoRem(n, scale, new(big.Int)); rem.Sign() != 0 {
		return 0, false, true
	}
	if d.neg {
		n.Neg(n)
	}
	if !n.IsInt64() || n.Int64() < -types.DurationLimit || n.Int64() > types.DurationLimit {
		return 0, true, false
	}
	return n.Int64(), true, true
}

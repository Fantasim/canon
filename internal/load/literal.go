package load

import (
	"math/big"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// plainString is a string literal with no interpolation: load reads its path and a plain default straight off the syntax tree, without the evaluator (SPEC.md §5.4).
func plainString(s *syntax.StringLit) (string, bool) {
	var b strings.Builder
	for _, p := range s.Parts {
		if p.Interp != nil {
			return "", false
		}
		b.WriteString(p.Text)
	}
	return b.String(), true
}

// defaultLiteralOK is whether e is a plain literal wireHost can read as is, of the same base
// kind as k: a mismatch (`x: Float = 1`) needs the evaluator, so it is refused this milestone
// (DECISIONS 173, meta/decisions/log-2026-09-24.md "load.dir review (M2)").
func defaultLiteralOK(e syntax.Expr, k types.Kind) bool {
	switch n := unparen(e).(type) {
	case *syntax.NoneLit:
		return true
	case *syntax.BoolLit:
		return k == types.Bool
	case *syntax.IntLit:
		return k == types.Int
	case *syntax.DurationLit:
		return k == types.Duration
	case *syntax.StringLit:
		_, ok := plainString(n)
		return ok && k == types.String
	default:
		return false
	}
}

// unparen is e with every wrapping "(...)" removed: a redundant grouping around a literal
// default is still that literal (`nameKey: String =("szName")`).
func unparen(e syntax.Expr) syntax.Expr {
	for {
		p, ok := e.(*syntax.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

// defEval evaluates WIRE.md §6.8's #define grammar on explicit stacks, never the host's (DECISIONS 195).
type defEval struct {
	s        string
	i        int
	accepted map[string]int64
	vals     []*big.Int
	ops      []defOp
}

// evalDefineExpr is text's value, if it fully matches WIRE.md §6.8's grammar and fits Int.
func evalDefineExpr(text string, accepted map[string]int64) (int64, bool) {
	e := &defEval{s: text, accepted: accepted}
	for {
		if !e.operand() {
			return 0, false
		}
		done, ok := e.operator()
		switch {
		case !ok || (done && !e.vals[0].IsInt64()):
			return 0, false
		case done:
			return e.vals[0].Int64(), true
		}
	}
}

// operand reads any prefix "-" and "(" (pushed as pending), then one integer or accepted NAME.
func (e *defEval) operand() bool {
	for {
		e.skipSpace()
		if e.i >= len(e.s) {
			return false
		}
		c := e.s[e.i]
		switch {
		case c == '-':
			e.i++
			e.ops = append(e.ops, opNeg)
		case c == '(':
			e.i++
			e.ops = append(e.ops, opParen)
		case isDigit(c):
			return e.push(e.integer())
		case isNameStart(c):
			return e.push(e.nameValue())
		default:
			return false
		}
	}
}

// push adds v, an operand read, to the stack; nil is an operand that did not read.
func (e *defEval) push(v *big.Int) bool {
	if v == nil {
		return false
	}
	e.vals = append(e.vals, v)
	return true
}

// operator reads what follows an operand: any ")" (each closing its "("), then a binary
// operator, or the end; done once the whole text reduced to one value.
func (e *defEval) operator() (done, ok bool) {
	for {
		e.skipSpace()
		switch {
		case e.i >= len(e.s):
			return true, e.reduceAll()
		case e.s[e.i] == ')':
			e.i++
			if !e.closeParen() {
				return false, false
			}
		default:
			return false, e.binary()
		}
	}
}

// binary reads one binary operator, first applying every pending one binding at least as tight.
func (e *defEval) binary() bool {
	for _, b := range defBinaryOps {
		if strings.HasPrefix(e.s[e.i:], b.text) {
			e.i += len(b.text)
			if !e.reduceTo(defPrec[b.op]) {
				return false
			}
			e.ops = append(e.ops, b.op)
			return true
		}
	}
	return false
}

// reduceTo applies pending operators down to the innermost "(" or one binding looser than prec.
func (e *defEval) reduceTo(prec int) bool {
	for len(e.ops) > 0 {
		top := e.ops[len(e.ops)-1]
		if top == opParen || defPrec[top] < prec {
			return true
		}
		if !e.apply(top) {
			return false
		}
	}
	return true
}

// closeParen reduces to the innermost "(" and removes it; a ")" with none open fails.
func (e *defEval) closeParen() bool {
	if !e.reduceTo(0) || len(e.ops) == 0 {
		return false
	}
	e.ops = e.ops[:len(e.ops)-1]
	return true
}

// reduceAll applies every pending operator; an unclosed "(" fails.
func (e *defEval) reduceAll() bool {
	return e.reduceTo(0) && len(e.ops) == 0 && len(e.vals) == 1
}

// apply pops op and applies it to the operand stack's top (unary) or top two (binary).
func (e *defEval) apply(op defOp) bool {
	e.ops = e.ops[:len(e.ops)-1]
	if op == opNeg {
		v := e.top()
		v.Neg(v)
		return true
	}
	b := e.vals[len(e.vals)-1]
	e.vals = e.vals[:len(e.vals)-1]
	a := e.top()
	switch op {
	case opOr:
		a.Or(a, b)
	case opAnd:
		a.And(a, b)
	case opAdd:
		a.Add(a, b)
	case opSub:
		a.Sub(a, b)
	default:
		return shiftBy(a, b, op == opShl)
	}
	return true
}

// top is the operand stack's top value, which the next operator applied updates in place.
func (e *defEval) top() *big.Int { return e.vals[len(e.vals)-1] }

// shiftBy shifts v in place by n, left or right; n outside 0..63 refuses the whole expression.
func shiftBy(v, n *big.Int, left bool) bool {
	count := n.Int64()
	if !n.IsInt64() || count < 0 || count > maxShift {
		return false
	}
	u := uint(count)
	if left {
		v.Lsh(v, u)
	} else {
		v.Rsh(v, u) // arithmetic: big.Int.Rsh is two's-complement for negative v
	}
	return true
}

// nameValue is the NAME at i's accepted value; nil for a name not (yet) accepted in the file.
func (e *defEval) nameValue() *big.Int {
	start := e.i
	for e.i < len(e.s) && isNameCont(e.s[e.i]) {
		e.i++
	}
	v, ok := e.accepted[e.s[start:e.i]]
	if !ok {
		return nil
	}
	return big.NewInt(v)
}

// integer reads WIRE.md §6.8's hex, octal or decimal literal, suffixes ignored; nil when malformed.
func (e *defEval) integer() *big.Int {
	switch {
	case strings.HasPrefix(e.s[e.i:], hexPrefixLower) || strings.HasPrefix(e.s[e.i:], hexPrefixUpper):
		e.i += len(hexPrefixLower)
		return e.digits(isHexDigit, hexBase, false)
	case e.s[e.i] == '0':
		e.i++
		return e.digits(isOctDigit, octBase, true)
	default:
		return e.digits(isDigit, decBase, false)
	}
}

// digits reads a run of is-digit bytes in base, empty allowed only for octalZero's bare "0".
func (e *defEval) digits(is func(byte) bool, base int, octalZero bool) *big.Int {
	start := e.i
	for e.i < len(e.s) && is(e.s[e.i]) {
		e.i++
	}
	digits := e.s[start:e.i]
	for e.i < len(e.s) && strings.IndexByte(intSuffixes, e.s[e.i]) >= 0 {
		e.i++
	}
	if digits == "" {
		if octalZero {
			return new(big.Int)
		}
		return nil
	}
	v, ok := new(big.Int).SetString(digits, base)
	if !ok {
		return nil
	}
	return v
}

func (e *defEval) skipSpace() {
	for e.i < len(e.s) && (e.s[e.i] == ' ' || e.s[e.i] == '\t') {
		e.i++
	}
}

func isDigit(c byte) bool    { return '0' <= c && c <= '9' }
func isOctDigit(c byte) bool { return '0' <= c && c <= '7' }
func isHexDigit(c byte) bool { return isDigit(c) || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F') }
func isNameStart(c byte) bool {
	return c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}
func isNameCont(c byte) bool { return isNameStart(c) || isDigit(c) }

package check

import (
	"strconv"
	"strings"
)

// pairTemplate is a `pairs:` key template `pre{i}suf` over slots 0 to n-1, never expanded (WIRE.md §5.14).
type pairTemplate struct {
	pre, suf string
	n        int
}

func newPairTemplate(tpl string, n int) *pairTemplate {
	pre, suf, _ := strings.Cut(tpl, pairSlot)
	return &pairTemplate{pre: pre, suf: suf, n: n}
}

// text is the template as written.
func (t pairTemplate) text() string { return t.pre + pairSlot + t.suf }

// matches reports s among the expanded keys: pre, a slot in decimal, suf.
func (t pairTemplate) matches(s string) bool {
	d, ok := strings.CutPrefix(s, t.pre)
	if !ok {
		return false
	}
	d, ok = strings.CutSuffix(d, t.suf)
	return ok && slotBelow(d, t.n)
}

// slotBelow reports d as a slot's decimal text, without a leading zero, below n.
func slotBelow(d string, n int) bool {
	if d == "" || (len(d) > 1 && d[0] == zeroDigit) {
		return false
	}
	if _, err := strconv.ParseUint(d, decimalBase, bits64); err != nil || n < 1 {
		return false
	}
	last := strconv.Itoa(n - 1)
	return len(d) < len(last) || len(d) == len(last) && d <= last
}

// width is the number of digits of the largest slot.
func (t pairTemplate) width() int { return len(strconv.Itoa(t.n - 1)) }

// commonKey is a key both templates expand to, tried for each digit count of a's slot: at most
// 19 alignments of the two templates' texts.
func commonKey(a, b pairTemplate) (string, bool) {
	for la := 1; la <= a.width(); la++ {
		lb := len(a.pre) + la + len(a.suf) - len(b.pre) - len(b.suf)
		if lb < 1 || lb > b.width() {
			continue
		}
		if k, ok := aligned(a, b, la, lb); ok {
			return k, true
		}
	}
	return "", false
}

// aligned is the smallest key both templates spell with slots of la and lb digits: fixed text
// must agree, a digit facing text takes it, digits facing digits are shared and take the least
// value, which minimises both slots at once.
func aligned(a, b pairTemplate, la, lb int) (string, bool) {
	key := make([]byte, len(a.pre)+la+len(a.suf))
	for p := range key {
		ca, slotA := a.at(p, la)
		cb, slotB := b.at(p, lb)
		switch {
		case !slotA && !slotB && ca != cb:
			return "", false
		case slotA && slotB:
			key[p] = leastDigit(p == len(a.pre) && la > 1 || p == len(b.pre) && lb > 1)
		case slotA:
			key[p] = cb
		default:
			key[p] = ca
		}
	}
	s := string(key)
	return s, slotBelow(s[len(a.pre):len(a.pre)+la], a.n) && slotBelow(s[len(b.pre):len(b.pre)+lb], b.n)
}

// at is the byte of t's text at p with a slot of l digits, or a slot position.
func (t pairTemplate) at(p, l int) (byte, bool) {
	switch {
	case p < len(t.pre):
		return t.pre[p], false
	case p < len(t.pre)+l:
		return 0, true
	default:
		return t.suf[p-len(t.pre)-l], false
	}
}

// leastDigit is the smallest digit a shared slot position takes: 1 where a slot starts.
func leastDigit(leading bool) byte {
	if leading {
		return oneDigit
	}
	return zeroDigit
}

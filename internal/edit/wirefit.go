package edit

import (
	"errors"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// jsonFieldAt is the field the value at cursor k is, when its JSON source writes it; nil else.
func (x *opCtx) jsonFieldAt(k int) *types.Field {
	if k <= 0 || k >= len(x.j.cur) || x.j.cur[k].mode != ModeJSON {
		return nil
	}
	return x.fieldAt(k)
}

// bitsAt reports the value at cursor k a list its JSON source writes as one bitmask (WIRE.md 5.3).
func (x *opCtx) bitsAt(k int) bool {
	f := x.jsonFieldAt(k)
	return f != nil && f.Enc == types.EncBits
}

// withElem is the bits list at cursor k-1 with its element at cursor k, by its index, replaced by v.
func (x *opCtx) withElem(k int, v value.Value) value.Value {
	l, ok := x.valueAt(k - 1).(*value.List)
	i, at := elemIndex(x.res.Steps[k-1].Seg)
	if !ok || !at || i >= int64(len(l.Elems)) {
		return v
	}
	out := &value.List{T: l.T, Elems: slices.Clone(l.Elems), P: l.P}
	out.Elems[i] = v
	return out
}

// elemIndex is the index a plain list's element segment names: `[n]` or `[#n]`.
func elemIndex(seg Seg) (int64, bool) {
	switch {
	case seg.Kind == SegPos:
		return int64(seg.Pos), seg.Pos >= 0
	case seg.Kind == SegKey && seg.Key.Kind == KeyInt:
		return seg.Key.Int, seg.Key.Int >= 0
	}
	return 0, false
}

// bitsRefusal is why v, the value of field f, has no bitmask: a member it holds twice (WIRE.md
// 5.3, E8102); "" when f is no bits field or v has one.
func bitsRefusal(f *types.Field, v value.Value) string {
	l, ok := v.(*value.List)
	if !ok || f == nil || f.Enc != types.EncBits {
		return ""
	}
	seen := map[int]bool{}
	for _, e := range l.Elems {
		m, isMember := e.(*value.Member)
		if !isMember {
			continue
		}
		if seen[m.Index] {
			return detailBitsTwice + m.CanonText()
		}
		seen[m.Index] = true
	}
	return ""
}

// noneRefusal is why v, a present value of field f, has no wire form: it encodes as f's
// `@json(none:)` marker (WIRE.md 5.4, E8102); "" when it does not.
func noneRefusal(f *types.Field, v value.Value) string {
	if f == nil || f.NoneWire == nil || v == nil {
		return ""
	}
	if _, none := v.(*value.None); none {
		return ""
	}
	if _, err := encodeWire(v, f); errors.Is(err, wire.ErrNoneMarker) {
		return detailNoneMarker + string(f.NoneWire)
	}
	return ""
}

// wireRefusal is why v, the value of field f, has no wire form in a JSON source: a pairs list no
// slots write, a bits list holding a member twice, a value its none marker writes; "" when it has one.
func wireRefusal(f *types.Field, v value.Value) string {
	for _, why := range []func(*types.Field, value.Value) string{slotsRefusal, bitsRefusal, noneRefusal} {
		if text := why(f, v); text != "" {
			return text
		}
	}
	return ""
}

package control

import (
	"encoding/json"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// ReadOnly is why field f is shown read-only by its view entry (C43), "" when it is not, and
// the only value its type admits (C34). Of several reasons, the one the studio must name wins:
// input, then deprecated, then single, then the `readonly: true` property.
func (r *Resolver) ReadOnly(f *types.Field) (reason string, single json.RawMessage) {
	single = r.single(f)
	switch {
	case f.Input != nil:
		return ReadonlyInput, single
	case f.Deprecated != nil:
		return ReadonlyDeprecated, single
	case single != nil:
		return ReadonlySingle, single
	case r.index.Field(f).True(syntax.PropReadonly):
		return ReadonlyView, nil
	}
	return "", nil
}

// single is the only value a non-optional field's type admits (C34), nil for none: an
// integer, Float or Duration range with equal bounds, a `where` that is `it == c` or holds it
// as a conjunct (c folded), or an enum with exactly one active member.
func (r *Resolver) single(f *types.Field) json.RawMessage {
	t := f.Type
	if t.Base().Kind() == types.Optional {
		return nil
	}
	if c := equalsConstant(t); c != nil && r.env.Fold != nil {
		if v, ok := r.env.Fold(c); ok {
			return encode.Value(v)
		}
	}
	switch x := t.Base().(type) {
	case *types.EnumType:
		return onlyMember(x)
	case types.Basic:
		return equalBounds(t, x)
	}
	return nil
}

// onlyMember is the name of an enum's one active member, nil when it has another count.
func onlyMember(e *types.EnumType) json.RawMessage {
	if activeMembers(e) != 1 {
		return nil
	}
	for _, m := range e.Members {
		if !m.Retired {
			out, _ := json.Marshal(m.Name)
			return out
		}
	}
	return nil
}

// equalBounds is the value of a range whose bounds are equal, nil otherwise.
func equalBounds(t types.Type, b types.Basic) json.RawMessage {
	if b.K == types.Float {
		l := shape.LayersOf(t)
		if l.HasLo && l.HasHi && l.HiIncluded && l.Lo.F == l.Hi.F {
			return json.RawMessage(encode.Float(l.Lo.F, b.Bits).Text)
		}
		return nil
	}
	if b.K != types.Int && b.K != types.Duration {
		return nil
	}
	r := encode.IntRange(t)
	if !r.HasLo || !r.HasHi || r.Lo != r.Hi {
		return nil
	}
	n, _ := vm.Int(r.Lo).MarshalJSON()
	return n
}

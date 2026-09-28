package verify

import (
	"slices"

	"github.com/fantasim/canonlang/internal/value"
)

// part is sc for an element, a key, a map value or a pair half: not a field's own value.
func (sc scope) part() scope {
	sc.field, sc.direct = "", false
	return sc
}

// parts is a container's values as verification leaves them, copied at the first change so a
// container another record instance shares keeps its own values.
type parts struct {
	from []value.Value
	out  []value.Value
}

func (p *parts) set(i int, v value.Value) {
	if p.out == nil && v != p.from[i] {
		p.out = slices.Clone(p.from)
	}
	if p.out != nil {
		p.out[i] = v
	}
}

func (p *parts) changed() bool { return p.out != nil }

func (p *parts) at(i int) value.Value { return p.all()[i] }

func (p *parts) all() []value.Value {
	if p.out != nil {
		return p.out
	}
	return p.from
}

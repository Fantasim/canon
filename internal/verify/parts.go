package verify

import (
	"slices"

	"github.com/fantasim/canonlang/internal/value"
)

// part is sc for a field, an element, a key, a map value or a pair half: not its own slot's field or past (TYPES.md §8.4).
func (sc scope) part() scope {
	sc.field, sc.direct, sc.past = "", false, false
	return sc
}

// entered is sc inside the table entry at p, retired or not (LOCK.md §4.3).
func (sc scope) entered(p string, retired bool) scope {
	sc.entry = p
	if retired {
		sc.retired = p
	}
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

package eval

import (
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/value"
)

// pathAfter is the canonical path of the value n segments of the amendment's path lead to, as far
// as locate followed it; nil past that (API.md F1).
func (m *amending) pathAfter(n int) *vpath {
	switch {
	case n == 0:
		return rootPath(m.root.Name())
	case n <= len(m.trail):
		return m.trail[n-1]
	}
	return nil
}

// located is the path of the amended value, or of what locate reached of it.
func (m *amending) located() *vpath {
	return m.pathAfter(len(m.trail))
}

// step adds the path the segment just followed leads to.
func (m *amending) step(p *vpath) {
	m.trail = append(m.trail, p)
}

// elementPath is the path of the element j (-1: an absent key) of the list or table cur under p,
// named by key when it has one (API.md P8); key is what the segment wrote, nil for a position.
func elementPath(p *vpath, cur value.Value, j int, key value.Value) *vpath {
	elems := std.Elems(cur)
	if key == nil && j >= 0 && j < len(elems) {
		key = elems[j]
	}
	k, keyed := std.KeyOf(key)
	if !keyedColl(cur) || !keyed {
		if j >= 0 {
			return p.index(j)
		}
		return p
	}
	if _, isTable := cur.(*value.Table); isTable {
		return p.entry(k)
	}
	return p.key(k)
}

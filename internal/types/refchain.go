package types

import "slices"

// ChainEnd is how a walk down a chain of ref keys ends.
type ChainEnd uint8

// Chain is where a chain of ref keys ends, read at its leaf: the first key not a ref (DECISIONS 456).
type Chain struct {
	Field *Field // the last key field met; nil when none was
	Leaf  Type   // the leaf key; String at a table, Error at a broken target, nil on a loop
	End   ChainEnd
}

// Links reads a chain's links: a ref's target, nil when broken, and a key field's type (TYPES.md §10.2).
type Links interface {
	Target(r *RefType) *Collection
	KeyType(f *Field) Type
}

// resolved reads the links resolved types hold.
type resolved struct{}

func (resolved) Target(r *RefType) *Collection { return r.Target }

func (resolved) KeyType(f *Field) Type { return f.Type }

// KeyChain walks the chain of ref keys t starts; a t that is no ref is its own leaf (WIRE.md §5.9).
func KeyChain(t Type, l Links) Chain {
	r, ok := t.Base().(*RefType)
	if !ok {
		return Chain{Leaf: t, End: ChainLeaf}
	}
	return CollChain(l.Target(r), l)
}

// CollChain walks the chain of ref keys c's key starts, c met first; a target met twice is a loop (TYPES.md §9.1).
func CollChain(c *Collection, l Links) Chain {
	var f *Field
	var seen []*Collection
	for {
		switch {
		case c == nil:
			return Chain{Field: f, Leaf: ErrorType, End: ChainBroken}
		case slices.Contains(seen, c):
			return Chain{Field: f, End: ChainLoop}
		case c.KeyedBy == nil:
			return Chain{Field: f, Leaf: StringType, End: ChainTable}
		}
		seen = append(seen, c)
		f = c.KeyedBy
		t := l.KeyType(f)
		r, ok := t.Base().(*RefType)
		if !ok {
			return Chain{Field: f, Leaf: t, End: ChainLeaf}
		}
		c = l.Target(r)
	}
}

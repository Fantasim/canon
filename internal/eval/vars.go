package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/value"
)

// varSlot is a name bound to a value.
type varSlot struct {
	obj check.Object
	v   value.Value
}

// get is the value obj is bound to: in the frame first, then among its lambda's captures.
func (f *frame) get(obj check.Object) (value.Value, bool) {
	if v, ok := lookup(f.slots[:f.nslots], obj); ok {
		return v, true
	}
	if v, ok := f.vars[obj]; ok {
		return v, true
	}
	return lookup(f.caps, obj)
}

// set binds obj to v in the frame, over a capture of the same name; the captures are the
// closure's and never change. The map holds the names bound once the slots are full.
func (f *frame) set(obj check.Object, v value.Value) {
	for i := range f.nslots {
		if f.slots[i].obj == obj {
			f.slots[i].v = v
			return
		}
	}
	if f.nslots == frameSlots {
		if f.vars == nil {
			f.vars = map[check.Object]value.Value{}
		}
		f.vars[obj] = v
		return
	}
	f.slots[f.nslots] = varSlot{obj: obj, v: v}
	f.nslots++
}

// lookup is the value obj is bound to in bs.
func lookup(bs []varSlot, obj check.Object) (value.Value, bool) {
	for i := range bs {
		if bs[i].obj == obj {
			return bs[i].v, true
		}
	}
	return nil, false
}

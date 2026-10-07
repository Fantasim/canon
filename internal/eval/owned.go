package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// own records that the frame alone holds v, obj's collection, from the copy an element
// assignment made until a read other than a peek: the next element assignment may update it in
// place, and nothing can tell it from a rebuilt root.
func (f *frame) own(obj check.Object, v value.Value) {
	if f.owns == nil {
		f.owns = map[check.Object]value.Value{}
	}
	f.owns[obj] = v
}

// owner reports that obj is bound to the collection the frame owns.
func (f *frame) owner(obj check.Object) bool {
	v, ok := f.owns[obj]
	return ok && v == f.vars[obj]
}

// disown records that obj's value may be held elsewhere.
func (f *frame) disown(obj check.Object) {
	delete(f.owns, obj)
}

// peekRecv is recv(x), the receiver of an index read or of a std.Peeks method, read as a peek:
// it leaves a var's collection owned, unless the checker converts x, which may keep or mark it.
func (r *run) peekRecv(x syntax.Expr) value.Value {
	r.peek = nil
	if id := syntax.Unparen(x); r.ev.info.Conv[id] == nil {
		r.peek = id
	}
	v := r.recv(x)
	r.peek = nil
	return v
}

// readVar is the value obj is bound to, read at at: the frame keeps owning it only when at is
// the peek peekRecv marked, a mark the read consumes.
func (r *run) readVar(obj check.Object, at syntax.Expr) (value.Value, bool) {
	v, ok := r.fr.vars[obj]
	if at != nil && at == r.peek {
		r.peek = nil
		return v, ok
	}
	r.fr.disown(obj)
	return v, ok
}

// updated is v, a collection the frame owns, with slot i set to nv in place; slot -1 of a map
// appends key k. A keyed index built on v is dropped.
func (e *Evaluator) updated(v value.Value, i int, k, nv value.Value) value.Value {
	switch x := v.(type) {
	case *value.Map:
		x.Set(i, k, nv)
	case *value.List:
		x.Elems[i] = e.withIdent(nv, x.Elems[i])
		delete(e.keyed, v)
	case *value.Table:
		if rec, ok := e.withIdent(nv, x.Entries[i]).(*value.Record); ok {
			x.Entries[i] = rec
		}
		delete(e.keyed, v)
	}
	return v
}

// peeks reports a built-in method call on recv whose receiver read is a peek (std.Peeks).
func (r *run) peeks(callee *check.Callee, recv syntax.Expr) bool {
	return recv != nil && callee.Kind == check.CalleeBuiltin && std.Peeks(r.typeOf(recv), callee.Builtin)
}

package edit

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// keyedElement is the keyed-list element whose key field reference r states, false for any
// other reference: a rename rewrites that key and moves the element (API.md E11, DECISIONS 310).
func (a *applier) keyedElement(r Ref) (resolution, bool) {
	res, err := a.resolve(r.Package + packageMark + r.Path)
	field := len(res.Steps) - 1
	elem := field - 1
	if err != nil || elem < 0 || res.Steps[field].Seg.Kind != SegField {
		return resolution{}, false
	}
	lt, isList := baseOf(res.Steps[elem].Container).(*types.ListType)
	if !isList || lt.KeyedBy == nil || lt.KeyedBy.Name != res.Steps[field].Seg.Name {
		return resolution{}, false
	}
	p, err := Parse(res.Canonical)
	if err != nil {
		return resolution{}, false
	}
	p.Segs = p.Segs[:len(p.Segs)-1]
	el, err := a.snap.open(p)
	return el, err == nil
}

// cascade moves each keyed-list element whose key field held the renamed ref, once (E11).
func (r *renaming) cascade(elems []resolution) error {
	for _, el := range elems {
		if r.moved[el.Canonical] {
			continue
		}
		r.moved[el.Canonical] = true
		if err := r.moveElement(el); err != nil {
			return err
		}
	}
	return nil
}

// moveElement renames keyed-list element el (API.md E11, N8; DECISIONS 310): its `entry` line,
// @files file and path follow, every reference into it is rewritten, cascading in turn; its key
// field's token is a reference the enclosing rename rewrites.
func (r *renaming) moveElement(el resolution) error {
	l, isList := el.parent(len(el.Steps) - 1).(*value.List)
	lt, _ := baseOf(el.Steps[len(el.Steps)-1].Container).(*types.ListType)
	old, isRef := keyField(el.Target, lt.KeyedBy).(*value.Ref)
	key, isKey := renamedKey(r.to)
	if !isList || !isRef || !isKey {
		return &NotEditableError{Reason: ReasonKey}
	}
	to := &value.Ref{T: old.T, Key: key}
	if holdsKey(l, to) {
		return ErrKeyExists
	}
	x := &opCtx{a: r.x.a, op: r.x.op, res: el, j: r.x.a.snap.judge(el, OpRename, r.x.a.env.EditLayer), w: r.x.w}
	sub := &renaming{x: x, from: old, to: to, text: r.text, done: r.done, pkgs: r.pkgs, moved: r.moved}
	if err := sub.elementKey(l); err != nil {
		return err
	}
	x.w.rekey = append(x.w.rekey, pathMove{from: el.Canonical, to: childPath(x.parentPath(), keySeg(to, lt.KeyedBy.Type))})
	refs, cascades, err := x.renameRefs(l)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if err := sub.retoken(ref.Span, ref.Package); err != nil {
			return err
		}
	}
	return sub.cascade(cascades)
}

// elementKey changes what states a moved element's key apart from its key field: its `entry`
// line and @files file, or a load.dir element's file.
func (r *renaming) elementKey(l *value.List) error {
	c, pc := r.x.j.last(), r.x.j.holder()
	if pc.files && c.mode == ModeJSON {
		return r.fileKey(l)
	}
	if _, isDecl := c.node.(*syntax.EntryDecl); !isDecl {
		return nil
	}
	if err := r.canonKey(l, c, pc); err != nil {
		return err
	}
	return r.renameFile(c)
}

// renamedKey is the entry key a renamed key value names.
func renamedKey(v value.Value) (value.Key, bool) {
	switch x := v.(type) {
	case *value.Str:
		return value.Key{S: x.V}, true
	case *value.Int:
		return value.Key{I: x.V, IsInt: true}, true
	case *value.Ref:
		return x.Key, true
	}
	return value.Key{}, false
}

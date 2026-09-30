package edit

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// diffAt turns the value cursor k states, old, into nw, in its .canon literal or JSON source
// (API.md M1); a collection whose order its files give is never stated whole by one node.
func (x *opCtx) diffAt(k int, old, nw value.Value) error {
	c := x.j.cur[k]
	if c.files {
		return &NotEditableError{Reason: ReasonOrder}
	}
	if c.mode == ModeCanon {
		d := &canonDiff{a: x.a}
		if err := d.value(old, nw, c.node); err != nil {
			return err
		}
		x.w.addCanon(c.file.Src.Path, x.res.root.pkg.Path, d.out)
		return nil
	}
	n, display, err := x.jsonAt(old)
	if err != nil {
		return err
	}
	fr := x.frameAt(k)
	if nw, err = fr.symbolsIn(nw, old, x.declaredAt(k)); err != nil {
		return err
	}
	x.w.given = append(x.w.given, givenValue{path: x.pathAt(k), v: nw, held: fr.kept, changed: !sameValue(old, nw)})
	d := &jsonDiff{a: x.a}
	if err := d.value(old, nw, n, x.fieldAt(k)); err != nil {
		return err
	}
	x.w.addJSON(display, x.res.root.pkg.Path, d.out)
	return nil
}

// holder is the cursor of the value holding the path's target: the one before the last.
func (j *judge) holder() cursor {
	return j.cur[len(j.cur)-holderBack]
}

// jsonAt is the node of the JSON source stating v, and that file's display path.
func (x *opCtx) jsonAt(v value.Value) (*jsonsrc.Node, string, error) {
	p := provOf(v)
	if p == nil || p.Kind != value.ProvJSON {
		return nil, "", &NotEditableError{Reason: ReasonComputed}
	}
	display := x.a.snap.display(p.Span.File)
	root, err := x.a.jsonRoot(display)
	if err != nil {
		return nil, "", err
	}
	n := root.Find(p.Pointer)
	if n == nil {
		return nil, "", errNoTree
	}
	return n, display, nil
}

// jsonRoot is the document of the JSON source at display, as edited so far.
func (a *applier) jsonRoot(display string) (*jsonsrc.Node, error) {
	s, err := a.state(display)
	if err != nil {
		return nil, err
	}
	var fs source.FileSet
	src, err := fs.Add(display, display, s.cur)
	if err != nil {
		return nil, err
	}
	return jsonsrc.Parse(src, diag.NewBag(&fs, ""))
}

// fieldAt is the field the value at cursor k is, nil when it is no record's field.
func (x *opCtx) fieldAt(k int) *types.Field {
	if k == 0 {
		return nil
	}
	rec, ok := x.res.parent(k - 1).(*value.Record)
	if !ok {
		return nil
	}
	fields := fieldsOf(rec.T)
	if i := fieldIndex(fields, x.res.Steps[k-1].Seg.Name); i >= 0 {
		return fields[i]
	}
	return nil
}

// addCanon adds changes to a .canon file of package pkg.
func (w *work) addCanon(display, pkg string, changes []format.Change) {
	if len(changes) == 0 {
		return
	}
	w.canon[display] = append(w.canon[display], changes...)
	w.own(display, pkg)
}

// addJSON adds edits to a JSON source the package pkg loads.
func (w *work) addJSON(display, pkg string, edits []jsonEdit) {
	if len(edits) == 0 {
		return
	}
	w.json[display] = append(w.json[display], edits...)
	w.own(display, pkg)
}

// frameAt is the strict frame of the value at cursor k: the records and dependent map keys above it.
func (x *opCtx) frameAt(k int) depFrame {
	fr := depFrame{a: x.a, kept: map[*value.Symbol]bool{}}
	for j := 0; j < k && j < len(x.res.Steps); j++ {
		fr = fr.into(x.valueAt(j), x.declaredAt(j), x.res.Steps[j].Seg)
	}
	return fr
}

// declaredAt is the declared type of the value at cursor k, nil for the root.
func (x *opCtx) declaredAt(k int) types.Type {
	if k == 0 || k > len(x.res.Steps) {
		return nil
	}
	if f := x.fieldAt(k); f != nil {
		return f.Type
	}
	ct := x.res.Steps[k-1].Container
	if ct == nil {
		return nil
	}
	if _, vt, isMap := mapTypes(present(ct)); isMap {
		return vt
	}
	return elemType(present(ct))
}

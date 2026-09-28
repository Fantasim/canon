package check

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// keyReader reads a translation key left to right (I18N.md K5): a kind word is followed by a
// name, another reserved segment is a text part, a group or show id names nothing, anything
// else is a name, recorded in NameUses. A name that names nothing stops it: E1702 is i18n's.
type keyReader struct {
	c     *checker
	p     *pkgState
	f     *syntax.File
	e     *syntax.TranslationEntry
	parts []*syntax.Ident
	next  int
	step  bool // the key ends on a field's `step`
}

// readKey resolves the key of e and returns the scope its text is checked in when it names a
// template of the catalogue (I18N.md T1), else nil, and whether it names a field's step text.
func (c *checker) readKey(p *pkgState, f *syntax.File, e *syntax.TranslationEntry) (*env, bool) {
	r := &keyReader{c: c, p: p, f: f, e: e, parts: e.Key.Parts}
	return r.key(), r.step
}

// key reads the whole key.
func (r *keyReader) key() *env {
	first := r.take()
	if first == nil {
		return nil
	}
	if first.Name == checkSegment {
		return r.packageCheck()
	}
	o := r.p.names[first.Name]
	if o == nil {
		return nil
	}
	r.use(first, o)
	switch o.kind {
	case ObjTypeName:
		return r.typeKey(r.c.resolveTypeName(o))
	case ObjLet:
		return r.letKey(o)
	default:
	}
	return nil
}

// take is the next segment, nil past the last.
func (r *keyReader) take() *syntax.Ident {
	if r.next >= len(r.parts) {
		return nil
	}
	r.next++
	return r.parts[r.next-1]
}

// last reports that every segment has been read.
func (r *keyReader) last() bool { return r.next >= len(r.parts) }

// use records what id names, when it names something.
func (r *keyReader) use(id *syntax.Ident, o *object) *object {
	if o != nil {
		r.c.info.NameUses[id] = o
	}
	return o
}

// reservedSegment reports a reserved segment.
func reservedSegment(s string) bool { return reservedSet[s] }

// typeKey reads the rest of a key of a record, a variant or an enum (I18N.md §3.3).
func (r *keyReader) typeKey(t types.Type) *env {
	switch x := t.(type) {
	case *types.RecordType:
		r.c.completeRecord(x)
		return r.bodyKey(viewKey{typ: x}, r.c.bodyOf(x), x.Checks)
	case *types.VariantType:
		return r.variantKey(x)
	case *types.EnumType:
		r.enumKey(x)
	}
	return nil
}

// bodyKey reads the rest of a record's or case's key: view texts, checks, fields, methods (I18N.md K4).
func (r *keyReader) bodyKey(target viewKey, body *recordCtx, checks []*syntax.CheckDecl) *env {
	id := r.take()
	if id == nil || body == nil {
		return nil
	}
	switch id.Name {
	case titleWord, subtitleWord:
		return r.viewText(target, id.Name)
	case showWord:
		return r.showKey(target)
	case checkSegment:
		return r.checkKey(checks)
	case fieldSegment:
		return r.fieldKey(body, r.take())
	case methodWord:
		if m := r.take(); m != nil {
			r.use(m, body.methods[m.Name])
		}
		return nil
	}
	if reservedSegment(id.Name) {
		return nil
	}
	if fo := body.fields[id.Name]; fo != nil {
		return r.fieldKey(body, id)
	}
	r.use(id, body.methods[id.Name])
	return nil
}

// fieldKey reads a field's key; its `step` is the one template, checked apart (I18N.md T1).
func (r *keyReader) fieldKey(body *recordCtx, id *syntax.Ident) *env {
	if id == nil || r.use(id, body.fields[id.Name]) == nil {
		return nil
	}
	part := r.take()
	r.step = part != nil && part.Name == stepWord && r.last() && r.c.steps[body.fields[id.Name]]
	return nil
}

// viewText is the scope of the view of target when the key ends on its title or subtitle.
func (r *keyReader) viewText(target viewKey, part string) *env {
	vc := r.c.views[target]
	if vc == nil || !r.last() || !vc.texts[part] {
		return nil
	}
	return vc.env
}

// showKey reads `show.<id>` and ends on `text`, the show line's template.
func (r *keyReader) showKey(target viewKey) *env {
	id, part := r.take(), r.take()
	if id == nil || part == nil || part.Name != textWord {
		return nil
	}
	return r.viewText(target, showWord+dot+id.Name)
}

// checkKey reads a named check of checks; its message is a template in its scope.
func (r *keyReader) checkKey(checks []*syntax.CheckDecl) *env {
	id := r.take()
	if id == nil {
		return nil
	}
	for _, d := range checks {
		if d.Name != nil && d.Name.Name == id.Name {
			r.use(id, r.c.memberChecks[d])
			return r.messageScope(d)
		}
	}
	return nil
}

// packageCheck reads `check.<name>`, a named check of the package (CHK-03).
func (r *keyReader) packageCheck() *env {
	id := r.take()
	if id == nil {
		return nil
	}
	for _, o := range r.p.all {
		if o.kind == ObjCheck && o.owner == nil && o.name == id.Name {
			r.use(id, o)
			return r.messageScope(o.decl.(*syntax.CheckDecl))
		}
	}
	return nil
}

// messageScope is the scope of a one-line check's message, when the key ends there.
func (r *keyReader) messageScope(d *syntax.CheckDecl) *env {
	if !r.last() {
		return nil
	}
	return r.c.messageEnvs[d]
}

// variantKey reads a variant's key: its view's texts, then a case (I18N.md §3.3, K4).
func (r *keyReader) variantKey(v *types.VariantType) *env {
	id := r.take()
	if id == nil {
		return nil
	}
	switch {
	case id.Name == titleWord || id.Name == subtitleWord:
		return r.viewText(viewKey{typ: v}, id.Name)
	case id.Name == caseWord:
		id = r.take()
	case reservedSegment(id.Name):
		return nil
	}
	if id == nil {
		return nil
	}
	co := r.use(id, r.c.caseObject(v, id.Name))
	if co == nil {
		return nil
	}
	ct := co.typ.(*types.CaseType)
	return r.bodyKey(viewKey{typ: ct}, r.c.caseBodies[ct], ct.Checks)
}

// enumKey reads an enum member's key (K4); its texts are all plain.
func (r *keyReader) enumKey(e *types.EnumType) {
	id := r.take()
	if id != nil && id.Name == memberWord {
		id = r.take()
	} else if id != nil && reservedSegment(id.Name) {
		return
	}
	if id != nil {
		r.use(id, r.c.memberObject(e, id.Name))
	}
}

// letKey reads a let's key: a define-table view's texts, or an entry and its field (I18N.md K3).
func (r *keyReader) letKey(o *object) *env {
	id := r.take()
	switch {
	case id == nil:
		return nil
	case id.Name == titleWord || id.Name == subtitleWord:
		return r.viewText(viewKey{let: o}, id.Name)
	case reservedSegment(id.Name) || o.keys == nil:
		return nil
	}
	if r.use(id, o.keys.byName[id.Name]) == nil {
		return nil
	}
	elem, _, ok := collectionElem(r.c.letType(o))
	f := r.take()
	if !ok || f == nil {
		return nil
	}
	if rec, isRec := elem.Base().(*types.RecordType); isRec {
		if fd := fieldNamed(rec.Fields, f.Name); fd != nil {
			r.use(f, r.c.fieldObjects[fd])
		}
	}
	return nil
}

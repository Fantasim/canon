package views

import (
	"cmp"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/layout"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// shapeUse counts the instances of one shape of a type and the fields each sets (L7).
type shapeUse struct {
	decl  types.Type // the record, or the case of a variant instance
	count int
	set   map[string]int
}

// usage is the `usage` section (VIEWMODEL.md 12.7): the shapes of the record and variant
// instances reachable from the package's public values (L10), each record or case once.
func (b *builder) usage() {
	uses := map[string]map[string]*shapeUse{}
	seen := map[*value.Record]bool{}
	for _, o := range b.pkg.Decls {
		if b.ctx.Err() != nil {
			return // Build reports the cancellation
		}
		d, ok := o.Decl().(*syntax.LetDecl)
		if !ok || o.Kind() != check.ObjLet || shape.Local(d) || b.broken(o) {
			continue
		}
		if v, ok := b.colls.Let(b.pkg.Path, o.Name()); ok {
			walkRecords(v, seen, func(r *value.Record) { count(uses, r) })
		}
	}
	//canon:unordered each type's shapes written by key into a map
	for name, shapes := range uses {
		u := vm.Usage{Shapes: map[string]vm.Shape{}}
		//canon:unordered each shape written by key into a map
		for key, s := range shapes {
			u.Shapes[key] = b.shape(s)
		}
		b.m.Usage[name] = u
	}
}

// count adds the record or case value r to the uses of its shape (L19).
func count(uses map[string]map[string]*shapeUse, r *value.Record) {
	name, key := encode.Name(r.T), shapeKey(r)
	if uses[name] == nil {
		uses[name] = map[string]*shapeUse{}
	}
	s := uses[name][key]
	if s == nil {
		s = &shapeUse{decl: r.T.Base(), set: map[string]int{}}
		uses[name][key] = s
	}
	s.count++
	setFields(r, "", s.set)
}

// setFields counts each field key of r that its source sets (L7: present after spreads, not
// filled by its default), every key of the shape listed.
func setFields(r *value.Record, prefix string, set map[string]int) {
	for i, f := range encode.FieldsOf(r.T) {
		key, n := prefix+f.Name, set[prefix+f.Name]
		if i < len(r.Set) && r.Set[i] {
			n++
		}
		set[key] = n
		if c := inlineCase(f, r, i); c != nil {
			setFields(c, key+dot+caseName(c)+dot, set)
		}
	}
}

// inlineCase is the case value of r's @json(inline) variant field f at i, nil for another field.
func inlineCase(f *types.Field, r *value.Record, i int) *value.Record {
	if !f.Inline || i >= len(r.Fields) {
		return nil
	}
	c, _ := r.Fields[i].(*value.Record)
	return c
}

// caseName is the case of a case value, "" for a record.
func caseName(r *value.Record) string {
	if c, ok := r.T.Base().(*types.CaseType); ok {
		return c.Name
	}
	return ""
}

// shapeKey is a value's shape key (L19, 12.7): a case value's case name, then for each inline
// variant field in declaration order `<field>=<case>` followed by that case's own, joined by `/`.
func shapeKey(r *value.Record) string {
	var steps []string
	if c := caseName(r); c != "" {
		steps = append(steps, c)
	}
	return strings.Join(inlineSteps(r, steps), shapeSep)
}

func inlineSteps(r *value.Record, steps []string) []string {
	for i, f := range encode.FieldsOf(r.T) {
		if c := inlineCase(f, r, i); c != nil {
			steps = inlineSteps(c, append(steps, f.Name+shapeIs+caseName(c)))
		}
	}
	return steps
}

// shape is one shape's usage: its count, the fields set, and its `_other` fields split in `main`
// (set in at least 25 % of the instances) and `more`, by set count then view order (L7).
func (b *builder) shape(s *shapeUse) vm.Shape {
	out := vm.Shape{Count: s.count, Fields: s.set, Main: []string{}}
	other := slices.DeleteFunc(layout.Other(b.layoutInput(), s.decl), func(k string) bool {
		_, in := s.set[k]
		return !in
	})
	slices.SortStableFunc(other, func(x, y string) int { return cmp.Compare(s.set[y], s.set[x]) })
	for _, k := range other {
		if usageShare*s.set[k] >= s.count {
			out.Main = append(out.Main, k)
		} else {
			out.More = append(out.More, k)
		}
	}
	return out
}

// walkRecords calls fn on each record and case value v holds, itself included, once each; the
// case of an inline variant field belongs to its record (L17) and is not one of them.
func walkRecords(v value.Value, seen map[*value.Record]bool, fn func(*value.Record)) {
	switch x := v.(type) {
	case *value.Record:
		if seen[x] {
			return
		}
		seen[x] = true
		fn(x)
		walkFields(x, seen, fn)
	case *value.List:
		for _, e := range x.Elems {
			walkRecords(e, seen, fn)
		}
	case *value.Table:
		for _, e := range x.Entries {
			walkRecords(e, seen, fn)
		}
	case *value.Map:
		for _, e := range x.Vals {
			walkRecords(e, seen, fn)
		}
	}
}

// walkFields walks the fields of a record, and through its inline cases' fields.
func walkFields(r *value.Record, seen map[*value.Record]bool, fn func(*value.Record)) {
	for i, f := range encode.FieldsOf(r.T) {
		if i >= len(r.Fields) {
			return
		}
		if c := inlineCase(f, r, i); c != nil {
			walkFields(c, seen, fn)
			continue
		}
		walkRecords(r.Fields[i], seen, fn)
	}
}

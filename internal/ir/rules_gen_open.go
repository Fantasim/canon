package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
)

// checkForeignOpenEnums is E8019 `OpenEnum` once per enum of another package that its go types-mode emit opens and that a go emit of u in another mode references, at the first site holding it, through the types it reaches at any depth: only a types-mode decoder reads an opened enum's unknown members (DECISIONS 339).
func (s *stage) checkForeignOpenEnums(u *unit, es *emitSite) {
	reported := map[*Enum]bool{}
	for _, site := range s.openSites(u, es) {
		w := openWalk{p: u.p, e: es.e, seen: map[any]bool{}}
		w.typ(site.t)
		for _, en := range w.found {
			if !reported[en] {
				reported[en] = true
				u.reportGenConstruct(es, site.span, diag.KindOpenEnum)
			}
		}
	}
}

// openSites are the types a go emit of u writes (typeSites), then each dependent type's discriminant and branches.
func (s *stage) openSites(u *unit, es *emitSite) []typeSite {
	out := s.typeSites(u, es)
	for _, t := range u.p.Types {
		d, ok := t.(*Dependent)
		if !ok {
			continue
		}
		span := s.itemSpan(d, es.span())
		out = append(out, typeSite{t: d.Disc, span: span})
		for _, b := range d.Branches {
			out = append(out, typeSite{t: &b.Type, span: span})
		}
	}
	return out
}

// openWalk collects the opened enums of other packages a type holds, through the records, variants, cases and dependent types it reaches, each once.
type openWalk struct {
	p     *Package
	e     *Emit
	seen  map[any]bool
	found []*Enum
}

func (w *openWalk) typ(t *TypeRef) {
	if t == nil {
		return
	}
	w.typ(t.Elem)
	w.typ(t.Key)
	if t.Case != nil && !w.seen[t.Case] {
		w.seen[t.Case] = true
		w.fields(t.Case.Fields)
	}
	if t.Named == nil || w.seen[t.Named] {
		return
	}
	w.seen[t.Named] = true
	switch x := t.Named.(type) {
	case *Enum:
		if x.Pkg != w.p.Name && OpenEnum(w.p, w.e, x) {
			w.found = append(w.found, x)
		}
	case *Record:
		w.fields(x.Fields)
	case *Variant:
		for _, c := range x.Cases {
			w.fields(c.Fields)
		}
	case *Dependent:
		w.typ(x.Disc)
		for i := range x.Branches {
			w.typ(&x.Branches[i].Type)
		}
	}
}

// fields walks the types of a class's fields.
func (w *openWalk) fields(fields []*Field) {
	for _, f := range fields {
		w.typ(&f.Type)
	}
}

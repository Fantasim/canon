package ir

// walker visits TypeRefs depth first: a type, then its element and key; a named type's body
// once, when enter accepts it (a nil enter enters every one).
type walker struct {
	seen  map[Type]bool
	enter func(Type) bool
	visit func(*TypeRef)
}

func newWalker(enter func(Type) bool, visit func(*TypeRef)) *walker {
	return &walker{seen: map[Type]bool{}, enter: enter, visit: visit}
}

func (w *walker) ref(t *TypeRef) {
	if t == nil {
		return
	}
	w.visit(t)
	w.ref(t.Elem)
	w.ref(t.Key)
	if t.Named != nil {
		w.named(t.Named)
	}
}

// named enters a named type's body: fields and export fns of records and cases, the branches
// and discriminant of a dependent type, an enum's code type.
func (w *walker) named(n Type) {
	if n == nil || w.seen[n] || w.enter != nil && !w.enter(n) {
		return
	}
	w.seen[n] = true
	switch x := n.(type) {
	case *Record:
		w.body(x.Fields, x.Methods)
	case *Variant:
		for _, c := range x.Cases {
			w.body(c.Fields, c.Methods)
		}
	case *Dependent:
		w.ref(x.Disc)
		for _, b := range x.Branches {
			w.ref(&b.Type)
		}
	case *Enum:
		w.ref(x.Codes)
	}
}

func (w *walker) body(fields []*Field, fns []*ExportFn) {
	for _, f := range fields {
		w.ref(&f.Type)
	}
	for _, fn := range fns {
		w.fn(fn)
	}
}

func (w *walker) fn(fn *ExportFn) {
	for _, p := range fn.Params {
		w.ref(&p.Type)
	}
	w.ref(&fn.Result)
}

// pkgRefs walks everything a package's IR names: its types, constants, values and fns.
func (w *walker) pkgRefs(p *Package) {
	for _, t := range p.Types {
		w.named(t)
	}
	for _, c := range p.Consts {
		w.ref(&c.Type)
	}
	for _, v := range p.Values {
		w.ref(&v.Type)
	}
	for _, fn := range p.Fns {
		w.fn(fn)
	}
}

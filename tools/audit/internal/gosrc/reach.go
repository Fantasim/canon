package gosrc

import "go/types"

// Reacher collects the named types an API exposes: in a signature, an exported field, a
// variable's type. Within picks the types it records and walks through.
type Reacher struct {
	Within func(*types.TypeName) bool
	Seen   map[*types.TypeName]bool
}

func NewReacher(within func(*types.TypeName) bool) *Reacher {
	return &Reacher{Within: within, Seen: map[*types.TypeName]bool{}}
}

// Seed walks the types of pkg's package-level objects, and of its named types' methods,
// that keep selects.
func (r *Reacher) Seed(pkg *types.Package, keep func(types.Object) bool) {
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if keep(obj) {
			r.walk(obj.Type())
		}
		named, ok := obj.Type().(*types.Named)
		if _, isType := obj.(*types.TypeName); !isType || !ok {
			continue
		}
		for m := range named.Methods() {
			if keep(m) {
				r.walk(m.Type())
			}
		}
	}
}

func (r *Reacher) walk(t types.Type) {
	switch t := t.(type) {
	case *types.Named:
		r.named(t)
	case *types.Alias:
		r.walk(types.Unalias(t))
	case *types.Pointer:
		r.walk(t.Elem())
	case *types.Slice:
		r.walk(t.Elem())
	case *types.Array:
		r.walk(t.Elem())
	case *types.Chan:
		r.walk(t.Elem())
	case *types.Map:
		r.walk(t.Key())
		r.walk(t.Elem())
	case *types.Signature:
		r.walk(t.Params())
		r.walk(t.Results())
	case *types.Tuple:
		for v := range t.Variables() {
			r.walk(v.Type())
		}
	case *types.Struct:
		r.fields(t)
	case *types.Interface:
		for m := range t.Methods() {
			r.walk(m.Type())
		}
	}
}

func (r *Reacher) fields(t *types.Struct) {
	for f := range t.Fields() {
		if f.Exported() || f.Embedded() {
			r.walk(f.Type())
		}
	}
}

func (r *Reacher) named(t *types.Named) {
	for a := range t.TypeArgs().Types() {
		r.walk(a)
	}
	tn := t.Origin().Obj()
	if !r.Within(tn) || r.Seen[tn] {
		return
	}
	r.Seen[tn] = true
	r.walk(t.Underlying())
}

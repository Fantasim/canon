package ir

import (
	"math"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// goImportUse is what baked Go imports for one package and emit (CODEGEN.md §2.8): the standard packages it writes and the Canon packages whose names it qualifies.
type goImportUse struct {
	own  string
	std  map[string]bool
	pkgs map[string]bool
}

// importUse walks what gen/go writes (CODEGEN.md §2.8): enums need strconv, enums and containers iter, the baked data sync, a -0.0 literal math (decision 202), each type its kind's package, translated bodies their helpers; data mode's loaders and types mode's decoders rt, encoding/json, fmt, strings and slices, types mode's unicode/utf8 too, data mode's snapshot sync/atomic; another package's class built or forwarded, its package and what it names (DECISIONS 323).
func (pl *GoNamePlan) importUse() *goImportUse {
	u := &goImportUse{own: pl.p.Name, std: map[string]bool{}, pkgs: map[string]bool{}}
	u.std[goMath] = pl.writesNegZero()
	for _, c := range pl.p.Consts {
		u.ref(&c.Type)
	}
	for _, t := range pl.p.Types {
		u.named(t)
	}
	for _, v := range pl.p.Values {
		u.std[goStrconv] = u.std[goStrconv] || pl.data == nil && tableRecord(v) != nil
	}
	for _, v := range pl.emitted {
		u.ref(&v.Type)
		u.std[goIter] = u.std[goIter] || IsContainer(v)
		u.std[goSync] = pl.data == nil
		if pl.data != nil && goRootClass(v) != nil {
			u.mark(goDataImports...)
			u.std[goAtomic] = u.std[goAtomic] || v.Reload
		}
	}
	pl.typesImports(u)
	for _, fn := range pl.p.Fns {
		u.fn(fn, false)
	}
	for _, c := range pl.foreign.Built() {
		u.foreignClass(c, pl.foreign.classPkg(c))
	}
	for _, row := range pl.rows {
		u.foreignClass(row.Record, row.Record.Pkg)
	}
	return u
}

// typesImports marks what types mode's public decoders and their helpers write, when the package has a class to decode (CODEGEN.md §5.13).
func (pl *GoNamePlan) typesImports(u *goImportUse) {
	if pl.e.Mode == ModeTypes && len(pl.data.decoded) > 0 {
		u.mark(goDataImports...)
		u.mark(goUTF8)
	}
}

// foreignClass marks what building another package's class through its hook, or forwarding its methods from a row, writes: that package, and what its stored fields, fns and branches name, its lists, maps and keyed lists aside, which are that package's rt (CODEGEN.md §2.8, §5.9, §5.14).
func (u *goImportUse) foreignClass(c any, pkg string) {
	u.pkgs[pkg] = true
	if d, ok := c.(*Dependent); ok {
		for _, b := range d.Branches {
			u.foreignRef(&b.Type)
		}
		return
	}
	fields, fns := classBody(c)
	for _, f := range fields {
		if f.Input == nil && (!f.Optional || f.Type.Kind != types.Never) {
			u.foreignRef(&f.Type)
		}
	}
	for _, fn := range fns {
		for _, p := range fn.Params {
			u.foreignRef(&p.Type)
		}
		u.foreignRef(&fn.Result)
	}
}

// foreignRef is ref for a type of another package's class: its rt types are that package's.
func (u *goImportUse) foreignRef(t *TypeRef) {
	if t == nil {
		return
	}
	if std := goStdOfKind[t.Kind]; std != "" && std != goRT {
		u.std[std] = true
	}
	u.markPkg(t)
	u.foreignRef(t.Elem)
	u.foreignRef(t.Key)
}

// mark marks standard packages used.
func (u *goImportUse) mark(std ...string) {
	for _, s := range std {
		u.std[s] = true
	}
}

// named walks a type of the package: an enum's codes, a record's or case's fields and fns.
func (u *goImportUse) named(t Type) {
	switch x := t.(type) {
	case *Enum:
		u.std[goStrconv], u.std[goIter] = true, true
		u.ref(x.Codes)
	case *Record:
		u.body(x.Fields, x.Methods)
		u.inputImports(x.Fields)
	case *Variant:
		u.std[goStrconv] = true
		for _, c := range x.Cases {
			u.body(c.Fields, c.Methods)
		}
	}
}

func (u *goImportUse) body(fields []*Field, fns []*ExportFn) {
	for _, f := range fields {
		if !f.Optional || f.Type.Kind != types.Never {
			u.ref(&f.Type)
			walkTypeRef(f.Type, func(t TypeRef) { u.std[goRT] = u.std[goRT] || t.Kind == types.Table }) // a table field is an rt.KeyedList (CODEGEN.md §4.2)
		}
	}
	for _, fn := range fns {
		u.fn(fn, true)
	}
}

// fn walks an export fn's signature, and a translated one's checks and body (translatedUse); method tells a public method, whose Go types convert to the pure ones.
func (u *goImportUse) fn(fn *ExportFn, method bool) {
	if fn.Kind == FnTranslated {
		u.translated(fn, method)
		return
	}
	for _, p := range fn.Params {
		u.ref(&p.Type)
	}
	u.ref(&fn.Result)
}

// ref marks what writing t needs: its kind's standard package, the package of a named type of another package, and a table ref's, whose key is that table's id enum (§5.3, §5.8). Named types are not entered: another package's are only named, this one's walked above.
func (u *goImportUse) ref(t *TypeRef) {
	if t == nil {
		return
	}
	if std := goStdOfKind[t.Kind]; std != "" {
		u.std[std] = true
	}
	u.markPkg(t)
	u.ref(t.Elem)
	u.ref(t.Key)
}

// markPkg marks the package of t itself when it is another package's named type, or a table ref of another package, whose key is that table's id type (§5.3, §5.8).
func (u *goImportUse) markPkg(t *TypeRef) {
	if t.Named != nil && pkgOf(t.Named) != u.own {
		u.pkgs[pkgOf(t.Named)] = true
	}
	if r := t.Ref; r != nil && r.Coll == types.CollLet && !r.Local && !r.Keyed && r.Pkg != u.own {
		u.pkgs[r.Pkg] = true
	}
}

// writesNegZero reports a -0.0 that baked Go writes as math.Copysign (decision 181): in a list or map constant, a package fn's result or cells, or an emitted value with its records' stored results; refs are keys, not walked.
func (pl *GoNamePlan) writesNegZero() bool {
	z := &negZeroWalk{instances: ownInstances(pl.p), seen: map[*value.Record]bool{}}
	for _, c := range pl.p.Consts {
		if _, scalar := c.V.(*value.Float); !scalar { // a Float constant is a Go const, which gen/go refuses to be -0.0 (decision 181)
			z.value(c.V)
		}
	}
	for _, fn := range pl.p.Fns {
		z.value(fn.Value)
		z.table(fn.Table)
	}
	for _, v := range pl.emitted {
		z.value(v.V)
	}
	return z.found
}

// negZeroWalk looks for a -0.0 Float through records (once each), lists, maps and tables.
type negZeroWalk struct {
	instances map[*value.Record][]*Instance
	seen      map[*value.Record]bool
	found     bool
}

func (z *negZeroWalk) value(v value.Value) {
	switch x := v.(type) {
	case *value.Float:
		z.found = z.found || x.V == 0 && math.Signbit(x.V)
	case *value.Record:
		z.record(x)
	case *value.List:
		z.each(x.Elems)
	case *value.Map:
		z.each(x.Keys)
		z.each(x.Vals)
	case *value.Table:
		for _, e := range x.Entries {
			z.record(e)
		}
	}
}

// record walks a record's fields and the stored results gen/go writes beside them.
func (z *negZeroWalk) record(r *value.Record) {
	if z.seen[r] {
		return
	}
	z.seen[r] = true
	z.each(r.Fields)
	for _, in := range z.instances[r] {
		z.value(in.Result)
		z.table(in.Table)
	}
}

func (z *negZeroWalk) table(t *LookupTable) {
	if t != nil {
		z.each(t.Cells)
	}
}

func (z *negZeroWalk) each(vs []value.Value) {
	for _, v := range vs {
		z.value(v)
	}
}

// ownInstances are the stored results of the package's own record and case methods, by receiver: the ones gen/go writes.
func ownInstances(p *Package) map[*value.Record][]*Instance {
	out := map[*value.Record][]*Instance{}
	add := func(fns []*ExportFn) {
		for _, fn := range fns {
			for _, in := range fn.Instances {
				out[in.Recv] = append(out[in.Recv], in)
			}
		}
	}
	for _, t := range p.Types {
		switch x := t.(type) {
		case *Record:
			add(x.Methods)
		case *Variant:
			for _, c := range x.Cases {
				add(c.Methods)
			}
		}
	}
	return out
}

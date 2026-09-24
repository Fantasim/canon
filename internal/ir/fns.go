package ir

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// fnSite is an export fn with what stage E needs to compute and judge it.
type fnSite struct {
	fn    *ExportFn
	obj   check.Object
	decl  *syntax.FnDecl
	sig   *types.FuncType
	label string     // the fn in messages: `canTransition`, `Potion.healFor`
	pkg   string     // the declaring package
	recv  types.Type // the record or case of a method; nil for a package fn
}

// methods are the export methods of a record or case body, in declaration order; owner is
// the record or case as messages name it (`Potion`, `Reward.item`).
func (s *stage) methods(ms []*types.Method, items []syntax.RecordItem, owner string, recv types.Type) []*ExportFn {
	var out []*ExportFn
	for _, m := range ms {
		if !m.Export || m.Type == nil {
			continue
		}
		d := methodDecl(items, m.Name)
		if d == nil {
			continue
		}
		obj := s.info.Defs[d.Name]
		if obj == nil {
			continue
		}
		site := s.exportFn(obj, d, m.Type)
		site.label, site.recv = owner+qnameSep+m.Name, recv
		out = append(out, site.fn)
	}
	return out
}

func methodDecl(items []syntax.RecordItem, name string) *syntax.FnDecl {
	for _, it := range items {
		if d, ok := it.(*syntax.FnDecl); ok && d.Name != nil && d.Name.Name == name {
			return d
		}
	}
	return nil
}

// exportFn is the static IR of an export fn: signature, kind and name overrides (SPEC §9.4).
func (s *stage) exportFn(obj check.Object, d *syntax.FnDecl, sig *types.FuncType) *fnSite {
	fn := &ExportFn{Name: obj.Name(), Result: s.ref(sig.Result), Kind: kindOf(sig)}
	if d.Doc != nil {
		fn.Doc = d.Doc.Text
	}
	fn.ResultRange, _ = ownRefinements(sig.Result)
	for i, p := range sig.Params {
		param := &Param{Type: s.ref(p)}
		param.Range, _ = ownRefinements(p)
		if i < len(d.Params) && d.Params[i].Name != nil {
			param.Name = d.Params[i].Name.Name
		}
		fn.Params = append(fn.Params, param)
	}
	n := nameOverrides(d.Annotations)
	fn.Go, fn.Cpp, fn.TS = n.goName, n.cpp, n.ts
	site := &fnSite{fn: fn, obj: obj, decl: d, sig: sig, label: obj.Name(), pkg: obj.Pkg()}
	s.fnObjs[fn] = site
	return site
}

// kindOf is how an export fn is emitted (SPEC §9.4): no parameter, only finite ones, else translated. Finite is as check's `$` keys read it (WIRE.md §5.11): Bool, an enum, a ref into a collection that is not a keyed list.
func kindOf(sig *types.FuncType) FnKind {
	if len(sig.Params) == 0 {
		return FnPrecomputed
	}
	for _, p := range sig.Params {
		if !finite(p) {
			return FnTranslated
		}
	}
	return FnLookup
}

func finite(t types.Type) bool {
	switch x := t.Base().(type) {
	case *types.RefType:
		return x.Target != nil && x.Target.KeyedBy == nil
	default:
		k := t.Base().Kind()
		return k == types.Bool || k == types.Enum
	}
}

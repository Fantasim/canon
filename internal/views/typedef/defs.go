package typedef

import (
	"slices"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
)

// record is a record definition: its help, parameters, fields in declaration order and the
// parameterless methods a view names (VIEWMODEL.md 12.3).
func (s *Types) record(r *types.RecordType) vm.TypeDef {
	def := vm.TypeDef{Kind: defRecord, Name: r.Name, Help: s.help(s.keyed(), r.Doc, nil, r.Name)}
	for _, p := range r.Params {
		def.Params = append(def.Params, vm.Param{Name: p.Name, Type: s.Expr(nil, p.Type)})
	}
	def.Fields = s.fields(r, r.Fields, r.Name)
	def.Methods = s.methods(r, r.Methods)
	return def
}

// variant is a variant definition: its wire tag and every case, retired ones included, with
// the label, help, icon and tone `view V` gives it (VIEWMODEL.md 12.3, D5).
func (s *Types) variant(v *types.VariantType) vm.TypeDef {
	def := vm.TypeDef{Kind: defVariant, Name: v.Name, Help: s.help(s.keyed(), v.Doc, nil, v.Name), Tag: v.Tag}
	for _, c := range v.Cases {
		p := s.in.Index.Item(v, c.Name)
		seg := encode.CaseSeg(c.Name)
		def.Cases = append(def.Cases, vm.Case{
			Name: c.Name, Wire: c.Wire, Retired: c.Retired,
			Label: s.label(c.Name, s.keyed(), v.Name, seg), Help: s.help(s.keyed(), c.Doc, p, v.Name, seg),
			Icon: p.Name(syntax.PropIcon), Tone: p.Name(syntax.PropTone),
			Fields: s.fields(c, c.Fields, v.Name, seg), Methods: s.methods(c, c.Methods),
		})
	}
	return def
}

// enum is an enum definition: every member, retired ones included, with its wire value, index,
// code, and the label, help, icon and tone the enum's view gives it (VIEWMODEL.md 12.3).
func (s *Types) enum(e *types.EnumType) vm.TypeDef {
	def := vm.TypeDef{Kind: defEnum, Name: e.Name, Help: s.help(s.keyed(), e.Doc, nil, e.Name), Ordered: e.Ordered}
	keyed := s.keyed() || e.Name == syntax.StudioMenu
	if e.Codes != nil {
		def.Codes = e.Codes.String()
	}
	for _, m := range e.Members {
		p := s.in.Index.Item(e, m.Name)
		seg := encode.MemberSeg(m.Name)
		mem := vm.Member{
			Name: m.Name, Wire: vm.Scalar{Text: m.Wire, Quoted: true}, Index: m.Index, Retired: m.Retired,
			Label: s.label(m.Name, keyed, e.Name, seg), Help: s.help(keyed, m.Doc, p, e.Name, seg),
			Icon: p.Name(syntax.PropIcon), Tone: p.Name(syntax.PropTone),
		}
		if m.HasCode {
			mem.Code = vm.Int(m.Code)
		}
		if e.WireCodes {
			mem.Wire = vm.Scalar(vm.Int(m.Code))
		}
		def.Members = append(def.Members, mem)
	}
	return def
}

// fields are the field definitions of a record or case, keyed under key (I18N.md §3.3).
func (s *Types) fields(decl types.Type, fields []*types.Field, key ...string) []vm.Field {
	out := make([]vm.Field, 0, len(fields))
	for _, f := range fields {
		if f.Type == nil || f.Type.Kind() == types.Error {
			continue
		}
		out = append(out, s.field(decl, f, append(slices.Clip(key), encode.FieldSeg(f.Name))))
	}
	return out
}

// methods are the parameterless methods of a record or case that a view names (§12.3).
func (s *Types) methods(owner types.Type, methods []*types.Method) []vm.Method {
	var out []vm.Method
	for _, m := range s.named(owner, methods) {
		out = append(out, vm.Method{Name: m.Name, Returns: s.Expr(owner, m.Type.Result)})
	}
	return out
}

// named are the methods of owner a view names that take no parameter besides self (§3.3).
func (s *Types) named(owner types.Type, methods []*types.Method) []*types.Method {
	var out []*types.Method
	for _, m := range methods {
		if m.Type != nil && len(m.Type.Params) == 0 && s.in.Index.Named(owner, m.Name) {
			out = append(out, m)
		}
	}
	return out
}

// keyed reports texts that are keys of the package's catalogue: every package's but the
// studio's, whose catalogue holds only its Menu members and unit suffixes (I18N.md K3, J9).
func (s *Types) keyed() bool { return s.pkg != s.in.Studio }

// label is an item's label: its key, or, outside the catalogue, its Canon name (X1) as a
// language-neutral text.
func (s *Types) label(name string, keyed bool, key ...string) vm.TextRef {
	if !keyed {
		return vm.TextRef{Text: &name}
	}
	return encode.Key(s.pkg, key...)
}

// help is the help of an item keyed key: its `help` property, else its doc comment (I18N.md
// 3.3); absent without either, or outside the catalogue.
func (s *Types) help(keyed bool, doc string, p control.Props, key ...string) vm.TextRef {
	if !keyed {
		return vm.TextRef{}
	}
	if text, ok := p.TextIn(syntax.PropHelp, s.pkg); ok {
		doc = text
	}
	return encode.Plain(doc, s.pkg, append(slices.Clip(key), syntax.PropHelp)...)
}

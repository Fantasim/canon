package typedef

import (
	"slices"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
)

// record is a record definition: its help, parameters, fields in declaration order and the
// parameterless methods a view names (VIEWMODEL.md 12.3).
func (s *Types) record(r *types.RecordType) vm.TypeDef {
	def := vm.TypeDef{Kind: defRecord, Name: r.Name, Help: s.help(r.Doc, nil, r.Name)}
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
	def := vm.TypeDef{Kind: defVariant, Name: v.Name, Help: s.help(v.Doc, nil, v.Name), Tag: v.Tag}
	for _, c := range v.Cases {
		p := s.in.Index.Item(v, c.Name)
		seg := i18n.CaseSeg(c.Name)
		def.Cases = append(def.Cases, vm.Case{
			Name: c.Name, Wire: c.Wire, Retired: c.Retired,
			Label: s.label(v, c.Name, v.Name, seg), Help: s.help(c.Doc, p, v.Name, seg),
			Icon: p.Member(syntax.PropIcon), Tone: p.Member(syntax.PropTone),
			Fields: s.fields(c, c.Fields, v.Name, seg), Methods: s.methods(c, c.Methods),
		})
	}
	return def
}

// enum is an enum definition: every member, retired ones included, with its wire value, index,
// code, and the label, help, icon and tone the enum's view gives it (VIEWMODEL.md 12.3).
func (s *Types) enum(e *types.EnumType) vm.TypeDef {
	def := vm.TypeDef{Kind: defEnum, Name: e.Name, Help: s.help(e.Doc, nil, e.Name), Ordered: e.Ordered}
	if e.Codes != nil {
		def.Codes = e.Codes.String()
	}
	for _, m := range e.Members {
		p := s.in.Index.Item(e, m.Name)
		seg := i18n.MemberSeg(m.Name)
		mem := vm.Member{
			Name: m.Name, Wire: vm.Scalar{Text: m.Wire, Quoted: true}, Index: m.Index, Retired: m.Retired,
			Label: s.label(e, m.Name, e.Name, seg), Help: s.help(m.Doc, p, e.Name, seg),
			Icon: p.Member(syntax.PropIcon), Tone: p.Member(syntax.PropTone),
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
		out = append(out, s.field(decl, f, append(slices.Clip(key), i18n.FieldSeg(f.Name))))
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

// label is the label of the member or case name of owner, keyed key: its view label, else its
// Canon name (VIEWMODEL.md X1), by the catalogue (J9).
func (s *Types) label(owner types.Type, name string, key ...string) vm.TextRef {
	text, ok := s.in.Index.ItemLabel(owner, name)
	if !ok {
		text = name
	}
	return s.in.Texts.Label(s.pkg, text, name, key...)
}

// help is the help of an item keyed key: its `help` property, else its doc comment (I18N.md
// 3.3), by the catalogue.
func (s *Types) help(doc string, p control.Props, key ...string) vm.TextRef {
	if text, ok := p.TextIn(syntax.PropHelp, s.pkg); ok {
		doc = text
	}
	return s.in.Texts.Text(s.pkg, doc, append(slices.Clip(key), syntax.PropHelp)...)
}

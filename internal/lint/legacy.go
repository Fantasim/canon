package lint

import (
	"slices"
	"strings"
	"unicode"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// holder is a record or a case of a variant: the declaration whose fields are judged, with the types
// the checker made of it.
type holder struct {
	pkg     string
	path    string // `Rec`, or `Variant.case`
	file    *syntax.File
	body    *syntax.RecordBody
	typ     types.Type
	fields  []*types.Field
	methods []*types.Method
}

// legacyPrefixes reports W0001 (API.md §5.7, rule L2) for every field of pkg's records and cases.
func legacyPrefixes(p *pass, pkg *check.Package, bag *diag.Bag) {
	for _, f := range sources(pkg) {
		for _, d := range f.Decls {
			for _, h := range p.holders(pkg.Path, f, d) {
				p.judgeFields(h, bag)
			}
		}
	}
}

// holders are the records and cases d declares; none for a declaration the checker found broken.
func (p *pass) holders(pkg string, f *syntax.File, d syntax.Decl) []holder {
	switch d := d.(type) {
	case *syntax.RecordDecl:
		rt, ok := p.declared(d.Name).(*types.RecordType)
		if !ok || d.Body == nil {
			return nil
		}
		return []holder{{pkg: pkg, path: rt.Name, file: f, body: d.Body, typ: rt, fields: rt.Fields, methods: rt.Methods}}
	case *syntax.VariantDecl:
		vt, ok := p.declared(d.Name).(*types.VariantType)
		if !ok {
			return nil
		}
		return caseHolders(pkg, f, d, vt)
	}
	return nil
}

// declared is the type name declares, nil when it is unknown or broken (TYPES.md §1).
func (p *pass) declared(name *syntax.Ident) types.Type {
	info := p.prog.Info
	obj := info.Defs[name]
	if obj == nil || info.Broken[obj] || obj.Type() == nil {
		return nil
	}
	return obj.Type()
}

// caseHolders are the cases of vt that d writes with a body, in source order.
func caseHolders(pkg string, f *syntax.File, d *syntax.VariantDecl, vt *types.VariantType) []holder {
	var out []holder
	for _, item := range d.Items {
		vc, ok := item.(*syntax.VariantCase)
		if !ok || vc.Body == nil {
			continue
		}
		i := slices.IndexFunc(vt.Cases, func(c *types.CaseType) bool { return c.Name == vc.Name.Name })
		if i < 0 {
			continue
		}
		ct := vt.Cases[i]
		out = append(out, holder{pkg: pkg, path: vt.Name + pathDot + ct.Name, file: f, body: vc.Body, typ: ct, fields: ct.Fields, methods: ct.Methods})
	}
	return out
}

// judgeFields reports the fields of h that keep a legacy prefix, and offers the fix of each that has one.
func (p *pass) judgeFields(h holder, bag *diag.Bag) {
	for _, item := range h.body.Items {
		fd, ok := item.(*syntax.FieldDecl)
		if !ok || fd.Name == nil {
			continue
		}
		i := slices.IndexFunc(h.fields, func(f *types.Field) bool { return f.Name == fd.Name.Name })
		if i < 0 || h.fields[i].Type == nil {
			continue
		}
		field := h.fields[i]
		prefix, rest, ok := matchPrefix(field.Name, kindOf(field.Type))
		if !ok {
			continue
		}
		name := suggestName(rest)
		span := h.file.Span(fd.Name)
		diag.W0001.At(span, field.Name, prefix, name).Report(bag)
		if p.fixable(h, field, name) {
			code := diag.W0001.Def().Code
			p.fixes[fixKey{code, span}] = Fix{
				Code: string(code), Span: span,
				Ops: []edit.Operation{{Kind: edit.OpRenameName, Path: renamePath(h, field.Name), Name: name}},
			}
		}
	}
}

// kindOf is the kind of t under `?`, aliases and refinements (API.md §5.7, rule L2).
func kindOf(t types.Type) types.Kind {
	if o, ok := t.Base().(*types.OptionalType); ok {
		t = o.Elem
	}
	return t.Kind()
}

// matchPrefix is the legacy prefix name keeps for a field of the given kind, and the rest of the name.
func matchPrefix(name string, kind types.Kind) (prefix, rest string, ok bool) {
	for _, r := range prefixRules {
		rest, found := strings.CutPrefix(name, r.prefix)
		if !found || rest == "" || !endsPrefix(rest, r) {
			continue
		}
		if len(r.kinds) > 0 && !slices.Contains(r.kinds, kind) {
			continue
		}
		return r.prefix, rest, true
	}
	return "", "", false
}

// endsPrefix reports that rest begins as a name does after r: with a capital, or with any letter for `m_`.
func endsPrefix(rest string, r prefixRule) bool {
	first := []rune(rest)[0]
	if r.anyCase {
		return unicode.IsLetter(first)
	}
	return unicode.IsUpper(first)
}

// suggestName is rest with its leading capitals lowered, but, in a run of two or more capitals, the
// last one when a lower-case letter follows it (`ItemCountMax`, `ID`, `URLPath`).
func suggestName(rest string) string {
	r := []rune(rest)
	n := 0
	for n < len(r) && unicode.IsUpper(r[n]) {
		n++
	}
	if n > 1 && n < len(r) && unicode.IsLower(r[n]) {
		n--
	}
	return strings.ToLower(string(r[:n])) + string(r[n:])
}

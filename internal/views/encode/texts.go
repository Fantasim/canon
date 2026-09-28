package encode

import (
	"strings"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// Texts writes text references from the key catalogues of the program's packages (VIEWMODEL.md
// J9, I18N.md 3): a key is written only when its package's catalogue holds it, so the source
// table always has it.
type Texts struct {
	cats map[string]*i18n.Catalogue
}

// NewTexts reads text references from cats, by package; a package without one has no key.
func NewTexts(cats map[string]*i18n.Catalogue) *Texts { return &Texts{cats: cats} }

// Text is the reference of the text keyed segs in pkg: its key when catalogued; the neutral
// text when the catalogue holds it without a letter (I18N.md L7); absent otherwise.
func (t *Texts) Text(pkg, text string, segs ...string) vm.TextRef {
	cat := t.catalogue(pkg)
	if cat == nil || text == "" {
		return vm.TextRef{}
	}
	key := strings.Join(segs, dot)
	switch res := cat.Resolve(key); {
	case res.Found:
		return vm.TextRef{Key: pkg + colon + key}
	case res.NoLetter:
		return vm.TextRef{Text: &text}
	}
	return vm.TextRef{}
}

// Key is the key segs of pkg when catalogued, absent otherwise (a template's `text`, 12.4).
func (t *Texts) Key(pkg string, segs ...string) vm.TextRef {
	if _, ok := t.Source(pkg, segs...); !ok {
		return vm.TextRef{}
	}
	return vm.TextRef{Key: pkg + colon + strings.Join(segs, dot)}
}

// Label is a text that always exists (a label, I18N.md L8), text its source: its key when
// catalogued, text when the catalogue holds it without a letter, else the Canon name as a
// neutral text (a studio symbol without a key, J9).
func (t *Texts) Label(pkg, text, name string, segs ...string) vm.TextRef {
	if ref := t.Text(pkg, text, segs...); ref != (vm.TextRef{}) {
		return ref
	}
	return vm.TextRef{Text: &name}
}

// Source is the source text of the key segs of pkg (I18N.md 3.3); false when not catalogued.
func (t *Texts) Source(pkg string, segs ...string) (string, bool) {
	cat := t.catalogue(pkg)
	if cat == nil {
		return "", false
	}
	e, ok := cat.Lookup(strings.Join(segs, dot))
	return e.Text, ok
}

func (t *Texts) catalogue(pkg string) *i18n.Catalogue {
	if t == nil {
		return nil
	}
	return t.cats[pkg]
}

// FieldSeg is a field's name as a key segment, behind `field.` when reserved (I18N.md K4).
func FieldSeg(name string) string { return segment(syntax.WordField, name) }

// MethodSeg is a method's name as a key segment, behind `method.` when reserved (I18N.md K4).
func MethodSeg(name string) string { return segment(syntax.WordMethod, name) }

// CaseSeg is a case's name as a key segment, behind `case.` when reserved (I18N.md K4).
func CaseSeg(name string) string { return segment(syntax.ArgCase, name) }

// MemberSeg is a member's name as a key segment, behind `member.` when reserved (I18N.md K4).
func MemberSeg(name string) string { return segment(syntax.WordMember, name) }

// PlainText is a string literal without interpolation, unescaped; false for another expression.
func PlainText(e syntax.Expr) (string, bool) {
	switch x := e.(type) {
	case *syntax.RawStringLit:
		return x.Value, true
	case *syntax.StringLit:
		var b strings.Builder
		for _, p := range x.Parts {
			if p.Interp != nil {
				return "", false
			}
			b.WriteString(p.Text)
		}
		return b.String(), true
	}
	return "", false
}

// TypeKey is the package and key prefix of a record, enum, variant or case (I18N.md 3.3): `T`,
// or `V.c` for a case; none for another type.
func TypeKey(t types.Type) (pkg string, segs []string) {
	switch x := t.Base().(type) {
	case *types.RecordType:
		return x.Pkg, []string{x.Name}
	case *types.AppliedRecord:
		return x.Rec.Pkg, []string{x.Rec.Name}
	case *types.VariantType:
		return x.Pkg, []string{x.Name}
	case *types.EnumType:
		return x.Pkg, []string{x.Name}
	case *types.CaseType:
		return x.Variant.Pkg, []string{x.Variant.Name, CaseSeg(x.Name)}
	}
	return "", nil
}

// FieldKey is the package and key of field f of the record or case decl (I18N.md 3.3): `T.f`
// or `V.c.f`.
func FieldKey(decl types.Type, f *types.Field) (pkg string, segs []string) {
	pkg, segs = TypeKey(decl)
	return pkg, append(segs, FieldSeg(f.Name))
}

func segment(kind, name string) string {
	if syntax.IsReservedSegment(name) {
		return kind + dot + name
	}
	return name
}

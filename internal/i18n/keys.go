package i18n

import (
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// namedForm is the two ways a name could end a key (I18N.md K4): bare is valid unless name is
// a reserved segment, in which case kind (kindWord.name) is the only valid one and bare is the
// alternate, wrong, form a translator might write by mistake.
func namedForm(prefix, name, kindWord string) (key, altKey string) {
	bare := join(prefix, name)
	kind := join(prefix, kindWord, name)
	if syntax.IsReservedSegment(name) {
		return kind, bare
	}
	return bare, kind
}

// segment is name as a key segment, behind kindWord + "." when it is a reserved one (I18N.md
// K4); FieldSeg, MethodSeg, CaseSeg and MemberSeg are its four kinds, for a caller (`views`)
// composing prefixes of its own (a qualified type from another package).
func segment(kindWord, name string) string {
	if syntax.IsReservedSegment(name) {
		return join(kindWord, name)
	}
	return name
}

// FieldSeg is a field's name as a key segment (I18N.md K4 "field").
func FieldSeg(name string) string { return segment(syntax.WordField, name) }

// MethodSeg is a method's name as a key segment (I18N.md K4 "method").
func MethodSeg(name string) string { return segment(syntax.WordMethod, name) }

// CaseSeg is a variant case's name as a key segment (I18N.md K4 "case").
func CaseSeg(name string) string { return segment(syntax.ArgCase, name) }

// MemberSeg is an enum member's name as a key segment (I18N.md K4 "member").
func MemberSeg(name string) string { return segment(syntax.WordMember, name) }

// TypeKey is a record, enum, variant or case's package and key prefix segments (I18N.md K "T").
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

// FieldKey is TypeKey(decl) with field f's own segment appended (I18N.md K "T.f").
func FieldKey(decl types.Type, f *types.Field) (pkg string, segs []string) {
	pkg, segs = TypeKey(decl)
	return pkg, append(segs, FieldSeg(f.Name))
}

// join concatenates key segments with dots, empty ones skipped.
func join(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, dot)
}

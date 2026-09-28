package encode

import (
	"strings"
	"unicode"

	"github.com/fantasim/canonlang/api/vm"
)

// Key is the text reference of the key segs of pkg's catalogue (VIEWMODEL.md J9, I18N.md K5).
func Key(pkg string, segs ...string) vm.TextRef {
	return vm.TextRef{Key: pkg + colon + strings.Join(segs, dot)}
}

// Plain is a plain text's reference: its key when translatable (I18N.md L6), else the text
// itself (L7); absent for an empty text.
func Plain(text, pkg string, segs ...string) vm.TextRef {
	switch {
	case text == "":
		return vm.TextRef{}
	case strings.ContainsFunc(text, unicode.IsLetter):
		return Key(pkg, segs...)
	}
	return vm.TextRef{Text: &text}
}

// FieldSeg is a field's name as a key segment, behind `field.` when reserved (I18N.md K4).
func FieldSeg(name string) string { return segment(segField, name) }

// CaseSeg is a case's name as a key segment, behind `case.` when reserved (I18N.md K4).
func CaseSeg(name string) string { return segment(segCase, name) }

// MemberSeg is a member's name as a key segment, behind `member.` when reserved (I18N.md K4).
func MemberSeg(name string) string { return segment(segMember, name) }

func segment(kind, name string) string {
	if reserved[name] {
		return kind + dot + name
	}
	return name
}

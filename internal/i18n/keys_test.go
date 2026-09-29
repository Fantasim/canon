package i18n_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/types"
)

// I18N.md K4: a reserved segment (a field, method, case or member named like one of "help",
// "check", … ) takes its kind word; another name is written bare.
func TestSegmentFunctionsK4(t *testing.T) {
	tests := []struct {
		fn         func(string) string
		name, want string
	}{
		{i18n.FieldSeg, "title", "field.title"},
		{i18n.FieldSeg, "cooldown", "cooldown"},
		{i18n.MethodSeg, "check", "method.check"},
		{i18n.MethodSeg, "heal", "heal"},
		{i18n.CaseSeg, "member", "case.member"},
		{i18n.CaseSeg, "dead", "dead"},
		{i18n.MemberSeg, "field", "member.field"},
		{i18n.MemberSeg, "red", "red"},
	}
	for _, tt := range tests {
		if got := tt.fn(tt.name); got != tt.want {
			t.Errorf("Seg(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// I18N.md K "T", "T.f": TypeKey and FieldKey compose a record, variant, enum or case's prefix.
func TestTypeKeyAndFieldKey(t *testing.T) {
	variant := &types.VariantType{Pkg: "a", Name: "Event"}
	dead := &types.CaseType{Variant: variant, Name: "check"} // a reserved case name (K4)
	variant.Cases = []*types.CaseType{dead}
	quest := &types.RecordType{Pkg: "a", Name: "Quest", Fields: []*types.Field{{Name: "heal"}}}

	if pkg, segs := i18n.TypeKey(quest); pkg != "a" || join(segs) != "Quest" {
		t.Errorf("TypeKey(Quest) = %q %v, want a [Quest]", pkg, segs)
	}
	if pkg, segs := i18n.TypeKey(dead); pkg != "a" || join(segs) != "Event.case.check" {
		t.Errorf("TypeKey(Event.check) = %q %v, want a [Event case.check]", pkg, segs)
	}
	if pkg, segs := i18n.FieldKey(quest, quest.Fields[0]); pkg != "a" || join(segs) != "Quest.heal" {
		t.Errorf("FieldKey(Quest.heal) = %q %v, want a [Quest heal]", pkg, segs)
	}
}

func join(segs []string) string {
	out := segs[0]
	for _, s := range segs[1:] {
		out += "." + s
	}
	return out
}

package i18n_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
)

// TestTranslationKeysAgreeWithCheck: i18n never calls a key valid that check leaves unresolved.
func TestTranslationKeysAgreeWithCheck(t *testing.T) {
	_, files := loadEveryExample(t)
	bags := check.Bags{}
	proj := exampleProject()
	prog := check.Check(context.Background(), proj, files, bags, eval.NewFolder(bags, eval.Options{}))
	res := i18n.Check(prog, proj, bags, emitsView(prog))

	for _, pkg := range prog.Packages {
		cat := res[pkg.Path].Catalogue
		for _, f := range pkg.Files {
			if f.FileKind != syntax.FileTranslation {
				continue
			}
			checkFileKeys(t, prog, f, cat)
		}
	}
}

// checkFileKeys checks every entry of f against cat and prog's NameUses.
func checkFileKeys(t *testing.T, prog *check.Program, f *syntax.File, cat *i18n.Catalogue) {
	t.Helper()
	for _, e := range f.Entries {
		if e.Key == nil || len(e.Key.Parts) == 0 {
			continue
		}
		if !cat.Resolve(syntax.Qualified(e.Key)).Found {
			continue
		}
		for _, name := range nameSegments(e.Key.Parts) {
			if prog.Info.ObjectOf(name) == nil {
				t.Errorf("%s: i18n resolves %s but check.Info has no object for %s",
					f.Src.Path, syntax.Qualified(e.Key), name.Name)
			}
		}
	}
}

// TestNameSegments: a kind word or `check` is stepped over to the name it introduces before
// IsReservedSegment ends the key, so a name after one is still returned (I18N.md K5).
func TestNameSegments(t *testing.T) {
	tests := []struct{ key, want string }{
		{"T.field.title", "T title"},
		{"T.check.n", "T n"},
		{"T.case.c.f", "T c f"},
		{"Menu.member.check.help", "Menu check"},
		{"T.c.f", "T c f"},
		{"T.show._0.text", "T"},
	}
	for _, tt := range tests {
		got := nameSegments(identParts(tt.key))
		if strings.Join(names(got), " ") != tt.want {
			t.Errorf("nameSegments(%s) = %v, want %s", tt.key, names(got), tt.want)
		}
	}
}

// identParts is key's dot-separated segments as bare Idents (name only; nameSegments reads no
// other field).
func identParts(key string) []*syntax.Ident {
	segs := strings.Split(key, ".")
	out := make([]*syntax.Ident, len(segs))
	for i, s := range segs {
		out[i] = &syntax.Ident{Name: s}
	}
	return out
}

// names is idents' own Name, in order.
func names(idents []*syntax.Ident) []string {
	out := make([]string, len(idents))
	for i, id := range idents {
		out[i] = id.Name
	}
	return out
}

// nameSegments is every identifier of a key's parts that names something (I18N.md K5).
func nameSegments(parts []*syntax.Ident) []*syntax.Ident {
	if parts[0].Name == syntax.WordCheck {
		if len(parts) > 1 {
			return parts[1:2]
		}
		return nil
	}
	names := []*syntax.Ident{parts[0]}
	for i := 1; i < len(parts); {
		switch parts[i].Name {
		case syntax.WordField, syntax.WordMethod, syntax.ArgCase, syntax.WordMember:
			if i+1 < len(parts) {
				names = append(names, parts[i+1])
			}
			i += 2
			continue
		case syntax.WordShow, syntax.WordGroup:
			i += 2 // the id after show/group names nothing
			continue
		case syntax.WordCheck:
			if i+1 < len(parts) {
				names = append(names, parts[i+1])
			}
			return names // a named check ends the key
		}
		if syntax.IsReservedSegment(parts[i].Name) {
			break // a text part (title, help, …) ends the key
		}
		names = append(names, parts[i]) // a bare field, case or member name
		i++
	}
	return names
}

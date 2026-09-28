package i18n

import (
	"strconv"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// viewEntry is a target's first view (VIEWMODEL.md §3.2).
type viewEntry struct {
	file *syntax.File
	decl *syntax.ViewDecl
}

// firstViews maps each view target of pkg's source files to its first view (I18N.md K1).
func firstViews(pkg *check.Package, info *check.Info) map[check.Object]viewEntry {
	out := map[check.Object]viewEntry{}
	for _, f := range pkg.Files {
		if f.FileKind == syntax.FileSource {
			collectViews(f, info, out)
		}
	}
	return out
}

// collectViews adds f's view decls to out, in declaration order.
func collectViews(f *syntax.File, info *check.Info, out map[check.Object]viewEntry) {
	for _, d := range f.Decls {
		if v, ok := d.(*syntax.ViewDecl); ok {
			addFirstView(out, viewTarget(v, info), f, v)
		}
	}
}

// addFirstView records f, v as o's first view; a later one for the same o is left alone.
func addFirstView(out map[check.Object]viewEntry, o check.Object, f *syntax.File, v *syntax.ViewDecl) {
	if o == nil {
		return
	}
	if _, dup := out[o]; !dup {
		out[o] = viewEntry{file: f, decl: v}
	}
}

// viewTarget is what a view decl names: a type, a case or a define-table let (check has already
// resolved it into NameUses).
func viewTarget(v *syntax.ViewDecl, info *check.Info) check.Object {
	if v.Case != nil {
		return info.ObjectOf(v.Case)
	}
	return info.ObjectOf(v.Type)
}

// viewText is the text of a top-level view item of kind k ("title", "subtitle", "singular",
// "plural"), when v has one.
func viewText(v *syntax.ViewDecl, k string) syntax.StrLit {
	for _, it := range v.Items {
		switch x := it.(type) {
		case *syntax.ViewTitle:
			if k == syntax.WordTitle {
				return x.Text
			}
		case *syntax.ViewSubtitle:
			if k == syntax.WordSubtitle {
				return x.Text
			}
		case *syntax.ViewSingular:
			if k == syntax.WordSingular {
				return x.Text
			}
		case *syntax.ViewPlural:
			if k == syntax.WordPlural {
				return x.Text
			}
		}
	}
	return nil
}

// scanned is what walkItems collects from a view: its groups, its show lines in order (unnamed
// ones counted across the whole view, VIEWMODEL.md G17), and its member items by name.
type scanned struct {
	groups []*syntax.ViewGroup
	shows  []*syntax.ViewShow
	fields map[string]*syntax.ViewField
}

// walkItems collects v's groups, show lines and member items (I18N.md §3.3).
func walkItems(v *syntax.ViewDecl) *scanned {
	s := &scanned{fields: map[string]*syntax.ViewField{}}
	for _, it := range v.Items {
		switch x := it.(type) {
		case *syntax.ViewGroup:
			s.groups = append(s.groups, x)
			for _, m := range x.Members {
				s.member(m)
			}
		default:
			if m, ok := it.(syntax.GroupMember); ok {
				s.member(m)
			}
		}
	}
	return s
}

func (s *scanned) member(m syntax.GroupMember) {
	switch x := m.(type) {
	case *syntax.ViewShow:
		s.shows = append(s.shows, x)
	case *syntax.ViewField:
		if x.Name != nil {
			if _, dup := s.fields[x.Name.Name]; !dup {
				s.fields[x.Name.Name] = x
			}
		}
	}
}

// showID is a show line's key segment: its written id, or "_<n>", n its 0-based position among
// the view's unnamed show lines so far (VIEWMODEL.md G17, I18N.md K "T.show.s").
func showID(s *syntax.ViewShow, unnamed int) string {
	if s.ID != nil {
		return s.ID.Name
	}
	return underscorePrefix + strconv.Itoa(unnamed)
}

// fieldProp is the text value of prop named on f's props, nil when absent or not a string.
func fieldProp(f *syntax.ViewField, prop string) syntax.StrLit {
	if f.Props == nil {
		return nil
	}
	for _, it := range f.Props.Items {
		fi, ok := it.(*syntax.FieldItem)
		if !ok || fi.Name == nil || fi.Name.Name != prop {
			continue
		}
		if s, ok := fi.Value.(syntax.StrLit); ok {
			return s
		}
	}
	return nil
}

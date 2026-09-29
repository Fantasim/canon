package i18n

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// labelInfo is a field, method, case or enum member's label and help source (I18N.md K "T.f",
// "T.c", "T.e", "T.m", each with ".help"). A broken view (view) supplies neither: a field, case
// or enum member falls back to def/doc (L8), but a method (methodLike) has no fallback (K2).
type labelInfo struct {
	key, alt, def, doc string
	vf                 *syntax.ViewField
	file               *syntax.File
	view               *syntax.ViewDecl
	methodLike         bool
}

// memberLabel adds m.key (and its .help): the view's label when it has one, else m.def; help
// is the view's help prop, else m.doc.
func (b *builder) memberLabel(m labelInfo) {
	broken := m.view != nil && check.ViewBroken(b.info, m.view)
	if broken && m.methodLike {
		b.tagView(m.key, m.view)
		b.tagView(join(m.key, syntax.PropHelp), m.view)
		return
	}
	vf := m.vf
	if broken {
		vf = nil
	}
	if vf != nil && vf.Label != nil {
		b.addText(m.key, m.alt, m.file, vf.Label, Plain)
	} else {
		b.addPlain(m.key, m.alt, m.def)
	}
	helpKey := join(m.key, syntax.PropHelp)
	if vf != nil {
		if s := fieldProp(vf, syntax.PropHelp); s != nil {
			b.addText(helpKey, "", m.file, s, Plain)
			return
		}
	}
	b.addPlain(helpKey, "", m.doc)
}

// viewTopEntries adds prefix.title, .subtitle, .singular and .plural, when view has them
// (I18N.md K "T.title", "T.subtitle", "T.singular", "T.plural"): view-only, VIEWMODEL.md J4.
func (b *builder) viewTopEntries(prefix string, view viewEntry) {
	b.addViewText(join(prefix, syntax.WordTitle), view.file, viewText(view.decl, syntax.WordTitle), Template, view.decl)
	b.addViewText(join(prefix, syntax.WordSubtitle), view.file, viewText(view.decl, syntax.WordSubtitle), Template, view.decl)
	b.addViewText(join(prefix, syntax.WordSingular), view.file, viewText(view.decl, syntax.WordSingular), Plain, view.decl)
	b.addViewText(join(prefix, syntax.WordPlural), view.file, viewText(view.decl, syntax.WordPlural), Plain, view.decl)
}

// groupsAndShows adds prefix.group.<id>[.intro] and prefix.show.<id>[.text] for view's groups
// and show lines, unnamed show ids counted across the whole view (I18N.md K "T.group.g",
// "T.show.s", VIEWMODEL.md G17): view-only, VIEWMODEL.md J4.
func (b *builder) groupsAndShows(prefix string, view viewEntry, sc *scanned) {
	if sc == nil {
		return
	}
	for _, g := range sc.groups {
		b.groupEntries(prefix, g, view)
	}
	unnamed := 0
	for _, s := range sc.shows {
		id := showID(s, unnamed)
		if s.ID == nil {
			unnamed++
		}
		b.showEntries(prefix, id, s, view)
	}
}

func (b *builder) groupEntries(prefix string, g *syntax.ViewGroup, view viewEntry) {
	if g.ID == nil {
		return
	}
	key := join(prefix, syntax.WordGroup, g.ID.Name)
	b.addViewText(key, view.file, g.Label, Plain, view.decl)
	if g.Help != nil {
		b.addViewText(join(key, syntax.WordIntro), view.file, g.Help, Plain, view.decl)
	}
}

func (b *builder) showEntries(prefix, id string, s *syntax.ViewShow, view viewEntry) {
	key := join(prefix, syntax.WordShow, id)
	b.addViewText(key, view.file, s.Label, Plain, view.decl)
	b.addViewText(join(key, syntax.WordText), view.file, s.Template, Template, view.decl)
}

package i18n

import "github.com/fantasim/canonlang/internal/syntax"

// labelInfo is a field, method, case or enum member's label and help source (I18N.md K "T.f",
// "T.f.help", "T.c", "T.c.help", "T.e", "T.e.help").
type labelInfo struct {
	key, alt, def, doc string
	vf                 *syntax.ViewField
	file               *syntax.File
}

// memberLabel adds m.key (and its .help): the view's label when it has one, else m.def; help
// is the view's help prop, else m.doc.
func (b *builder) memberLabel(m labelInfo) {
	if m.vf != nil && m.vf.Label != nil {
		b.addText(m.key, m.alt, m.file, m.vf.Label, Plain)
	} else {
		b.addPlain(m.key, m.alt, m.def)
	}
	helpKey := join(m.key, syntax.PropHelp)
	if m.vf != nil {
		if s := fieldProp(m.vf, syntax.PropHelp); s != nil {
			b.addText(helpKey, "", m.file, s, Plain)
			return
		}
	}
	b.addPlain(helpKey, "", m.doc)
}

// viewTopEntries adds prefix.title, .subtitle, .singular and .plural, when view has them
// (I18N.md K "T.title", "T.subtitle", "T.singular", "T.plural").
func (b *builder) viewTopEntries(prefix string, view viewEntry) {
	b.addText(join(prefix, syntax.WordTitle), "", view.file, viewText(view.decl, syntax.WordTitle), Template)
	b.addText(join(prefix, syntax.WordSubtitle), "", view.file, viewText(view.decl, syntax.WordSubtitle), Template)
	b.addText(join(prefix, syntax.WordSingular), "", view.file, viewText(view.decl, syntax.WordSingular), Plain)
	b.addText(join(prefix, syntax.WordPlural), "", view.file, viewText(view.decl, syntax.WordPlural), Plain)
}

// groupsAndShows adds prefix.group.<id>[.intro] and prefix.show.<id>[.text] for view's groups
// and show lines, unnamed show ids counted across the whole view (I18N.md K "T.group.g",
// "T.show.s", VIEWMODEL.md G17).
func (b *builder) groupsAndShows(prefix string, view viewEntry, sc *scanned) {
	if sc == nil {
		return
	}
	for _, g := range sc.groups {
		b.groupEntries(prefix, g, view.file)
	}
	unnamed := 0
	for _, s := range sc.shows {
		id := showID(s, unnamed)
		if s.ID == nil {
			unnamed++
		}
		b.showEntries(prefix, id, s, view.file)
	}
}

func (b *builder) groupEntries(prefix string, g *syntax.ViewGroup, file *syntax.File) {
	if g.ID == nil {
		return
	}
	key := join(prefix, syntax.WordGroup, g.ID.Name)
	b.addText(key, "", file, g.Label, Plain)
	if g.Help != nil {
		b.addText(join(key, syntax.WordIntro), "", file, g.Help, Plain)
	}
}

func (b *builder) showEntries(prefix, id string, s *syntax.ViewShow, file *syntax.File) {
	key := join(prefix, syntax.WordShow, id)
	b.addText(key, "", file, s.Label, Plain)
	b.addText(join(key, syntax.WordText), "", file, s.Template, Template)
}

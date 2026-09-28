package rules

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// allowed lists the items each target takes; every other one is E1627 (VIEWMODEL.md §3.2, G6).
var allowed map[targetKind][]syntax.NodeKind

// targetKinds are the words E1627 names each target by.
var targetKinds map[targetKind]diag.Kind

// repeatable are the items a view may hold several of; any other is E1605 given twice.
var repeatable []syntax.NodeKind

// itemTable checks an item by its node kind (VIEWMODEL.md §3.1, §3.6).
var itemTable map[syntax.NodeKind]func(*view, syntax.ViewItem)

func init() {
	repeatable = []syntax.NodeKind{syntax.KindViewGroup, syntax.KindViewShow, syntax.KindViewField}
	heads := []syntax.NodeKind{syntax.KindViewTitle, syntax.KindViewSubtitle, syntax.KindViewSingular, syntax.KindViewPlural}
	layout := []syntax.NodeKind{syntax.KindViewGroup, syntax.KindViewShow, syntax.KindViewField}
	lists := []syntax.NodeKind{syntax.KindViewSearch, syntax.KindViewColumns, syntax.KindViewFilters, syntax.KindViewMenu}
	allowed = map[targetKind][]syntax.NodeKind{
		targetRecord:  slices.Concat(heads, layout, lists, []syntax.NodeKind{syntax.KindViewPreview}),
		targetVariant: slices.Concat(heads, lists, []syntax.NodeKind{syntax.KindViewPreview, syntax.KindViewField}),
		targetCase:    slices.Concat(heads, layout, []syntax.NodeKind{syntax.KindViewPreview}),
		targetEnum:    {syntax.KindViewField},
		targetDefine:  slices.Concat(heads, []syntax.NodeKind{syntax.KindViewSearch}),
	}
	targetKinds = map[targetKind]diag.Kind{
		targetRecord: diag.KindRecord, targetVariant: diag.KindVariant, targetCase: diag.KindCase,
		targetEnum: diag.KindEnum, targetDefine: diag.KindDefineTable,
	}
	itemTable = map[syntax.NodeKind]func(*view, syntax.ViewItem){
		syntax.KindViewTitle:    func(v *view, it syntax.ViewItem) { v.template(it.(*syntax.ViewTitle).Text) },
		syntax.KindViewSubtitle: func(v *view, it syntax.ViewItem) { v.template(it.(*syntax.ViewSubtitle).Text) },
		syntax.KindViewSingular: func(v *view, it syntax.ViewItem) { v.plain(it.(*syntax.ViewSingular).Text) },
		syntax.KindViewPlural:   func(v *view, it syntax.ViewItem) { v.plain(it.(*syntax.ViewPlural).Text) },
		syntax.KindViewMenu:     (*view).menu,
		syntax.KindViewColumns:  (*view).columns,
		syntax.KindViewSearch:   (*view).search,
		syntax.KindViewFilters:  (*view).filters,
		syntax.KindViewPreview:  (*view).preview,
		syntax.KindViewShow:     func(v *view, it syntax.ViewItem) { v.show(it.(*syntax.ViewShow)) },
		syntax.KindViewGroup:    (*view).group,
		syntax.KindViewField:    func(v *view, it syntax.ViewItem) { v.member(it.(*syntax.ViewField), nil) },
	}
}

// item checks one item: E1627 when its target does not take it; a recovery node is skipped.
func (v *view) item(it syntax.ViewItem) {
	fn := itemTable[it.Kind()]
	if fn == nil {
		return
	}
	if !slices.Contains(allowed[v.kind], it.Kind()) {
		v.report(diag.E1627.At(v.head(it), v.itemWord(it), targetKinds[v.kind]))
		return
	}
	if !slices.Contains(repeatable, it.Kind()) && v.again(v.head(it), v.itemWord(it), v.items) {
		return
	}
	fn(v, it)
}

// again reports E1605 when seen already holds name, else records it at at (log-2026-09-28
// views V1 review calls): an item or a column or filter name given twice in one view.
func (v *view) again(at source.Span, name string, seen map[string]source.Span) bool {
	if first, dup := seen[name]; dup {
		v.report(diag.E1605.At(at, name, v.name, first))
		return true
	}
	seen[name] = at
	return false
}

// itemWord is an item's vocabulary word as written, or a field item's name (E1627).
func (v *view) itemWord(it syntax.ViewItem) string {
	if f, ok := it.(*syntax.ViewField); ok && f.Name != nil {
		return f.Name.Name
	}
	at := v.head(it)
	return string(v.file.Src.Content[at.Start:at.End])
}

// show is a show line: a plain label, a template and its id (VIEWMODEL.md G14, G17).
func (v *view) show(s *syntax.ViewShow) {
	v.id(idShow, s.ID)
	v.plain(s.Label)
	v.template(s.Template)
}

// group is a named group: its id, plain label and intro, and its members (VIEWMODEL.md §3.6).
func (v *view) group(it syntax.ViewItem) {
	g := it.(*syntax.ViewGroup)
	v.id(idGroup, g.ID)
	v.plain(g.Label)
	v.plain(g.Help)
	for _, m := range g.Members {
		switch m := m.(type) {
		case *syntax.ViewShow:
			v.show(m)
		case *syntax.ViewField:
			v.member(m, g.ID)
		}
	}
}

// id is a group or show id: reserved when it starts with `_` (E1618), unique in its view (E1617).
func (v *view) id(k idKind, id *syntax.Ident) {
	if id == nil {
		return
	}
	at := v.span(id)
	if strings.HasPrefix(id.Name, reserved) {
		v.report(diag.E1618.At(at, id.Name))
		return
	}
	first, dup := v.ids[k][id.Name]
	switch {
	case !dup:
		v.ids[k][id.Name] = at
	case k == idGroup:
		v.report(diag.E1617.AtGroup(at, id.Name, v.name, first))
	default:
		v.report(diag.E1617.AtShow(at, id.Name, v.name, first))
	}
}

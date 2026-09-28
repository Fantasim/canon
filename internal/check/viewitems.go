package check

import (
	"strconv"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// viewItemTable checks a view item by its node kind (VIEWMODEL.md §3.1, §3.6).
var viewItemTable map[syntax.NodeKind]func(*checker, *viewCtx, syntax.ViewItem)

// propTable checks a property value by its name; another name is E1613, views' (VIEWMODEL.md §3.5).
var propTable map[string]func(*checker, *viewCtx, syntax.Expr)

func init() {
	viewItemTable = map[syntax.NodeKind]func(*checker, *viewCtx, syntax.ViewItem){
		syntax.KindViewTitle:    (*checker).viewTitle,
		syntax.KindViewSubtitle: (*checker).viewSubtitle,
		syntax.KindViewSingular: func(c *checker, _ *viewCtx, it syntax.ViewItem) { c.plainText(it.(*syntax.ViewSingular).Text) },
		syntax.KindViewPlural:   func(c *checker, _ *viewCtx, it syntax.ViewItem) { c.plainText(it.(*syntax.ViewPlural).Text) },
		syntax.KindViewMenu:     (*checker).viewMenu,
		syntax.KindViewColumns:  (*checker).viewColumns,
		syntax.KindViewSearch:   (*checker).viewSearch,
		syntax.KindViewFilters:  (*checker).viewFilters,
		syntax.KindViewPreview:  func(c *checker, vc *viewCtx, it syntax.ViewItem) { c.synth(vc.env, it.(*syntax.ViewPreview).X) },
		syntax.KindViewShow:     (*checker).viewShow,
		syntax.KindViewGroup:    (*checker).viewGroup,
		syntax.KindViewField:    (*checker).viewField,
	}
	propTable = map[string]func(*checker, *viewCtx, syntax.Expr){
		syntax.PropUnit: (*checker).unitProp, syntax.PropWidget: (*checker).widgetProp,
		syntax.PropIcon: studioProp(syntax.StudioIcon), syntax.PropTone: studioProp(syntax.StudioTone),
		syntax.PropControl: (*checker).controlProp, syntax.PropStep: (*checker).stepText,
		syntax.PropWhen: (*checker).boolProp, syntax.PropReadonly: (*checker).boolProp, syntax.PropHidden: (*checker).boolProp,
		syntax.PropHelp: (*checker).plainProp, syntax.PropPlaceholder: (*checker).plainProp, syntax.PropNone: (*checker).plainProp,
	}
}

// viewItem checks one item; a recovery node records nothing (IMPLEMENTATION-PLAN §4.7).
func (c *checker) viewItem(vc *viewCtx, it syntax.ViewItem) {
	if check := viewItemTable[it.Kind()]; check != nil {
		check(c, vc, it)
	}
}

func (c *checker) viewTitle(vc *viewCtx, it syntax.ViewItem) {
	c.template(vc.env, it.(*syntax.ViewTitle).Text)
	vc.texts[titleWord] = true
}

func (c *checker) viewSubtitle(vc *viewCtx, it syntax.ViewItem) {
	c.template(vc.env, it.(*syntax.ViewSubtitle).Text)
	vc.texts[subtitleWord] = true
}

// viewMenu resolves `menu m icon i` in the studio package (VIEWMODEL.md G16).
func (c *checker) viewMenu(_ *viewCtx, it syntax.ViewItem) {
	m := it.(*syntax.ViewMenu)
	for _, n := range []struct {
		id   *syntax.Ident
		enum string
	}{{m.Menu, syntax.StudioMenu}, {m.Icon, syntax.StudioIcon}} {
		if n.id == nil {
			continue
		}
		if o := c.studioMember(n.enum, n.id.Name); o != nil {
			c.info.NameUses[n.id] = o
		}
	}
}

func (c *checker) viewColumns(vc *viewCtx, it syntax.ViewItem) {
	for _, col := range it.(*syntax.ViewColumns).Items {
		c.itemName(col.Name, func(n string) *object { return c.columnName(vc, n) })
		if col.Width != nil {
			c.info.Types[col.Width] = types.IntType
		}
	}
}

func (c *checker) viewFilters(vc *viewCtx, it syntax.ViewItem) {
	for _, f := range it.(*syntax.ViewFilters).Items {
		c.itemName(f.Name, func(n string) *object { return c.columnName(vc, n) })
	}
}

// viewSearch types each search term (VIEWMODEL.md G13; which types a term may have is E1622, views').
func (c *checker) viewSearch(vc *viewCtx, it syntax.ViewItem) {
	for _, e := range it.(*syntax.ViewSearch).Items {
		c.synth(vc.env, e)
	}
}

// viewShow is a show line: a plain label and a template; unnamed ones are `_0`, `_1`… (VIEWMODEL.md G17).
func (c *checker) viewShow(vc *viewCtx, it syntax.ViewItem) {
	s := it.(*syntax.ViewShow)
	c.plainText(s.Label)
	c.template(vc.env, s.Template)
	id := underscore + strconv.Itoa(vc.unnamed)
	if s.ID != nil {
		id = s.ID.Name
	} else {
		vc.unnamed++
	}
	vc.texts[showWord+dot+id] = true
}

// viewGroup is a group: plain label and intro, a Bool `when`, its members (VIEWMODEL.md §3.6).
func (c *checker) viewGroup(vc *viewCtx, it syntax.ViewItem) {
	g := it.(*syntax.ViewGroup)
	c.plainText(g.Label)
	c.plainText(g.Help)
	if g.When != nil {
		c.expr(vc.env, g.When, types.BoolType)
	}
	for _, m := range g.Members {
		c.viewItem(vc, m)
	}
}

// viewField is a member item: its name, plain label and properties (VIEWMODEL.md §3.3, §3.5).
func (c *checker) viewField(vc *viewCtx, it syntax.ViewItem) {
	f := it.(*syntax.ViewField)
	c.itemName(f.Name, func(n string) *object { return c.memberName(vc, n) })
	c.plainText(f.Label)
	if f.Props == nil {
		return
	}
	vc.field, _ = c.info.NameUses[f.Name].(*object)
	for _, p := range f.Props.Items {
		fi, ok := p.(*syntax.FieldItem)
		if !ok || fi.Name == nil || fi.Value == nil {
			continue
		}
		if check := propTable[fi.Name.Name]; check != nil {
			check(c, vc, fi.Value)
		}
	}
}

// itemName records what an item's name names; a name naming nothing is left to views (E1602).
func (c *checker) itemName(id *syntax.Ident, find func(string) *object) {
	if id == nil {
		return
	}
	if o := find(id.Name); o != nil {
		c.info.NameUses[id] = o
	}
}

// template types a view template in its scope (VIEWMODEL.md G14, §3.4).
func (c *checker) template(env *env, s syntax.StrLit) {
	if s != nil {
		c.synth(env, s)
	}
}

// plainText is a plain text: a String whose braces are E1615's, views' (VIEWMODEL.md G14).
func (c *checker) plainText(s syntax.StrLit) {
	if s == nil {
		return
	}
	c.info.Types[s] = types.StringType
	if c.lexError(s) {
		c.info.Types[s] = types.ErrorType
	}
}

func (c *checker) plainProp(_ *viewCtx, e syntax.Expr) {
	if s, ok := e.(syntax.StrLit); ok {
		c.plainText(s)
	}
}

// boolProp is `when`, `readonly` or `hidden`: a Bool (VIEWMODEL.md G13, §3.5).
func (c *checker) boolProp(vc *viewCtx, e syntax.Expr) {
	c.expr(vc.env, e, types.BoolType)
}

// controlProp is a built-in control's name: a symbol (VIEWMODEL.md §4.5; an unknown one is E1609, views').
func (c *checker) controlProp(_ *viewCtx, e syntax.Expr) {
	if id, ok := e.(*syntax.IdentExpr); ok {
		c.info.Symbols[id] = true
		c.info.Types[id] = types.StringType
	}
}

// stepText is a `step` template: each `{index}` is an Int; another interpolation is views' (VIEWMODEL.md T24).
func (c *checker) stepText(vc *viewCtx, e syntax.Expr) {
	s, ok := e.(syntax.StrLit)
	if !ok {
		return
	}
	c.plainText(s)
	if vc.field != nil {
		c.steps[vc.field] = true
	}
	sl, ok := s.(*syntax.StringLit)
	if !ok {
		return
	}
	var index *object
	for _, p := range sl.Parts {
		if id := bareIndex(p.Interp); id != nil {
			if index == nil {
				index = c.magicName(vc.env.pkg, vc.env.file, s, indexMember, types.IntType)
			}
			c.info.Uses[id], c.info.Types[id] = index, types.IntType
		}
	}
}

// bareIndex is the `index` of a `{index}` with no format spec, the one interpolation a step text
// allows (VIEWMODEL.md T24); nil for any other.
func bareIndex(in *syntax.Interp) *syntax.IdentExpr {
	if in == nil || in.Spec != nil {
		return nil
	}
	if id, ok := in.X.(*syntax.IdentExpr); ok && id.Name == indexMember {
		return id
	}
	return nil
}

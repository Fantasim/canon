package layout

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
)

// sections are the named groups in view order, then `_other`, "More" and "Unused fields"
// (VIEWMODEL.md L1, L3).
func (l *lay) sections() []vm.Section {
	var out []vm.Section
	placed := map[string]bool{}
	other := vm.Section{Kind: sectionOther, Fields: []string{}}
	unused := vm.Section{Kind: sectionUnused}
	var items []syntax.ViewItem
	if l.view.Decl != nil {
		items = l.view.Decl.Items
	}
	for _, it := range items {
		if g, ok := it.(*syntax.ViewGroup); ok {
			out = append(out, l.group(g, placed))
		}
	}
	for _, it := range items {
		if e, ok := l.entry(it); ok && !placed[e.Method] {
			other.Entries = append(other.Entries, e) // L22: a method a group names stays there
		}
	}
	for _, k := range l.keys {
		if k.Field.Deprecated != nil {
			unused.Fields = append(unused.Fields, k.Key) // L12
		}
	}
	other.Fields = append(other.Fields, l.other(placed)...)
	return append(out, other, vm.Section{Kind: sectionMore}, unused)
}

// Other are the `_other` field keys of a record or case in declaration order (VIEWMODEL.md L1,
// L8): the fields no group of its view places, deprecated ones aside.
func Other(in Input, t types.Type) []string {
	l := in.lay(t)
	placed := map[string]bool{}
	if l.view.Decl != nil {
		for _, it := range l.view.Decl.Items {
			if g, ok := it.(*syntax.ViewGroup); ok {
				l.group(g, placed)
			}
		}
	}
	return l.other(placed)
}

// other are the keys of the fields not placed, deprecated ones aside.
func (l *lay) other(placed map[string]bool) []string {
	var out []string
	for _, k := range l.keys {
		if k.Field.Deprecated == nil && !placed[k.Key] {
			out = append(out, k.Key)
		}
	}
	return out
}

// group is a named group (L4, L5): its entries in view order, a name matching case fields
// expanded to each of them (L18), a deprecated field left to "Unused fields" (L6).
func (l *lay) group(g *syntax.ViewGroup, placed map[string]bool) vm.Section {
	id := ""
	if g.ID != nil {
		id = g.ID.Name
	}
	segs := l.key(syntax.WordGroup, id)
	label, _ := encode.PlainText(g.Label)
	intro, _ := encode.PlainText(g.Help)
	sec := vm.Section{
		Kind: sectionGroup, ID: id, Label: l.in.Texts.Label(l.pkg, label, label, segs...),
		Intro: l.in.Texts.Text(l.pkg, intro, append(segs, syntax.WordIntro)...), Advanced: g.Advanced.Valid(),
		Entries: []vm.Entry{},
	}
	if g.When != nil {
		w := encode.SourceText(l.view.File, g.When)
		sec.When = &w
	}
	for _, m := range g.Members {
		for _, e := range l.entries(m) {
			sec.Entries = append(sec.Entries, e)
			if e.Show == "" {
				placed[e.Field+e.Method] = true // a field key or a method name
			}
		}
	}
	sec.Flatten = l.flatten(sec)
	return sec
}

// entries are a group member's entries: a field's every field key, but deprecated ones.
func (l *lay) entries(m syntax.Node) []vm.Entry {
	f, isField := m.(*syntax.ViewField)
	if o := l.namedObj(f, isField); o != nil && o.Kind() == check.ObjField {
		var out []vm.Entry
		for _, k := range encode.Named(l.keys, f.Name.Name) {
			if k.Field.Deprecated == nil {
				out = append(out, vm.Entry{Field: k.Key})
			}
		}
		return out
	}
	if e, ok := l.entry(m); ok {
		return []vm.Entry{e}
	}
	return nil
}

// entry is a method or `show` line entry (L21, L22); false for another item.
func (l *lay) entry(n syntax.Node) (vm.Entry, bool) {
	if s, ok := n.(*syntax.ViewShow); ok {
		return vm.Entry{Show: l.showIDs[s]}, true
	}
	f, isField := n.(*syntax.ViewField)
	if o := l.namedObj(f, isField); o != nil && o.Kind() == check.ObjMethod {
		return vm.Entry{Method: o.Name()}, true
	}
	return vm.Entry{}, false
}

// flatten reports a group whose one entry is a non-optional field shown as a section or a card,
// without `when` (L16).
func (l *lay) flatten(sec vm.Section) bool {
	if len(sec.Entries) != 1 || sec.Entries[0].Field == "" {
		return false
	}
	for _, k := range l.keys {
		if k.Key != sec.Entries[0].Field {
			continue
		}
		ctl := l.in.Res.Field(k.Decl, k.Field)
		_, when := l.in.Index.Field(k.Field).Source(syntax.PropWhen)
		optional := k.Field.Type.Base().Kind() == types.Optional
		return !optional && !when && (ctl.Kind == control.CtlSection || ctl.Kind == control.CtlCard)
	}
	return false
}

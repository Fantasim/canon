package live

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"github.com/fantasim/canonlang/internal/views/encode"
)

// item is an item of a view, or a member of one of its groups.
type item struct {
	n       syntax.Node
	grouped bool
}

// layoutOf is the view laying out rec's fields (VIEWMODEL.md D5: a case's own view) and its
// items and group members in view order, each group before its members; nil for none.
func (s *session) layoutOf(rec *value.Record) (string, []string, []item) {
	t := viewed(rec.T)[0]
	v, ok := s.index.ViewOf(t)
	if !ok || v.Decl == nil {
		return "", nil, nil
	}
	var out []item
	for _, it := range v.Decl.Items {
		out = append(out, item{n: it})
		if g, isGroup := it.(*syntax.ViewGroup); isGroup {
			for _, m := range g.Members {
				out = append(out, item{n: m, grouped: true})
			}
		}
	}
	pkg, prefix := i18n.TypeKey(t)
	return pkg, prefix, out
}

// when evaluates the `when` of every field, method and group of the view of fr's record
// (API.md V9, V12): false only when the condition evaluates to false.
func (s *session) when(fr *frame, at *verify.Path) {
	_, _, items := s.layoutOf(fr.rec)
	for _, it := range items {
		switch x := it.n.(type) {
		case *syntax.ViewGroup:
			if x.When != nil && x.ID != nil {
				s.cond(rel(at)+groupSep+x.ID.Name, x.When, fr)
			}
		case *syntax.ViewField:
			s.fieldWhen(fr, at, x)
		}
	}
}

// fieldWhen evaluates the `when` of a field or method item, keyed by the value path of each
// field it names in the current shape (VIEWMODEL.md L18, Q2) or of the method.
func (s *session) fieldWhen(fr *frame, at *verify.Path, f *syntax.ViewField) {
	x := whenOf(f)
	if x == nil || f.Name == nil {
		return
	}
	o := s.in.Program.Info.NameUses[f.Name]
	switch {
	case o == nil:
	case o.Kind() == check.ObjMethod:
		s.cond(rel(at.Field(f.Name.Name)), x, fr)
	case o.Kind() == check.ObjField:
		for _, p := range fieldPaths(fr.rec, at, f.Name.Name) {
			s.cond(rel(p), x, fr)
		}
	}
}

// cond records the condition x of fr's record at key: shown unless it evaluates to false, a
// failure counting as true (V12); the first condition given for a key stands.
func (s *session) cond(key string, x syntax.Expr, fr *frame) {
	if _, seen := s.out.When[key]; seen {
		return
	}
	v, ok := s.eval(x, fr.rec, fr.magic)
	b, isBool := v.(*value.Bool)
	s.out.When[key] = !ok || !isBool || b.V
}

// whenOf is the `when` property of a member item, nil for none (VIEWMODEL.md 3.5).
func whenOf(f *syntax.ViewField) syntax.Expr {
	if f.Props == nil {
		return nil
	}
	for _, it := range f.Props.Items {
		if fi, ok := it.(*syntax.FieldItem); ok && fi.Name != nil && fi.Name.Name == syntax.PropWhen {
			return fi.Value
		}
	}
	return nil
}

// fieldPaths are the paths of rec's field name, else of that field of the current case of each
// of its inline variant fields, nested ones depth first (VIEWMODEL.md L17, L18).
func fieldPaths(rec *value.Record, at *verify.Path, name string) []*verify.Path {
	fields := encode.FieldsOf(rec.T)
	for _, f := range fields {
		if f.Name == name {
			return []*verify.Path{at.Field(name)}
		}
	}
	var out []*verify.Path
	for i, f := range fields {
		if i >= len(rec.Fields) || !f.Inline {
			continue
		}
		if c, ok := rec.Fields[i].(*value.Record); ok {
			out = append(out, fieldPaths(c, at.Field(f.Name), name)...)
		}
	}
	return out
}

// shows evaluates the `show` lines and view-named methods of the view of fr's record, in view
// order, a method once: in its group when a group names it (API.md V10, VIEWMODEL.md Q1, L22).
func (s *session) shows(fr *frame, at *verify.Path) {
	pkg, prefix, items := s.layoutOf(fr.rec)
	unnamed := 0
	grouped, done := s.groupedMethods(items), map[string]bool{}
	for _, it := range items {
		switch x := it.n.(type) {
		case *syntax.ViewShow:
			id := unnamedShow + strconv.Itoa(unnamed)
			if x.ID != nil {
				id = x.ID.Name
			} else {
				unnamed++
			}
			s.showLine(fr, rel(at), pkg, slices.Concat(prefix, []string{syntax.WordShow, id}), x)
		case *syntax.ViewField:
			name := s.methodOf(x)
			if name != "" && !done[name] && grouped[name] == it.grouped {
				done[name] = true
				s.methodLine(fr, rel(at), pkg, slices.Concat(prefix, []string{i18n.MethodSeg(name)}), name)
			}
		}
	}
}

// groupedMethods are the methods a group of the view names (VIEWMODEL.md L22), each true.
func (s *session) groupedMethods(items []item) map[string]bool {
	out := map[string]bool{}
	for _, it := range items {
		if f, ok := it.n.(*syntax.ViewField); ok && it.grouped && s.methodOf(f) != "" {
			out[s.methodOf(f)] = true
		}
	}
	return out
}

// methodOf is the method a member item names, "" for none.
func (s *session) methodOf(f *syntax.ViewField) string {
	if o := s.nameOf(f); o != nil && o.Kind() == check.ObjMethod {
		return o.Name()
	}
	return ""
}

// nameOf is what a member item names, nil for nothing.
func (s *session) nameOf(f *syntax.ViewField) check.Object {
	if f.Name == nil {
		return nil
	}
	return s.in.Program.Info.NameUses[f.Name]
}

// showLine is a `show` line of the view keyed segs: its label and its template rendered in the
// session's language (V8, V11; I18N.md 3.3 `T.show.s`, `T.show.s.text`).
func (s *session) showLine(fr *frame, owner, pkg string, segs []string, x *syntax.ViewShow) {
	src, _ := encode.PlainText(x.Label)
	line := ShowLine{Owner: owner, Key: strings.Join(segs, dot), Label: s.label(pkg, segs, src)}
	textKey := slices.Concat(segs, []string{syntax.WordText})
	if s.in.Lines != nil {
		v, ok := s.in.Lines.Show(Line{Pkg: pkg, Key: textKey, Template: x.Template, Magic: fr.magic, Eval: s.memo}, fr.rec, s.lang)
		if ok {
			defer s.place(fr.rec, fr.magic)()
			line.Text = Text{Value: v, OK: true, Fallback: s.fallbackIn(pkg, textKey, x.Template, fr.rec, true)}
		}
	}
	s.out.Show = append(s.out.Show, line)
}

// methodLine is a view-named method keyed segs: its label and value (V10, VIEWMODEL.md L23).
func (s *session) methodLine(fr *frame, owner, pkg string, segs []string, name string) {
	src, ok := s.index.ItemLabel(viewed(fr.rec.T)[0], name)
	if !ok {
		src = i18n.Humanize(name)
	}
	line := ShowLine{Owner: owner, Key: strings.Join(segs, dot), Label: s.label(pkg, segs, src)}
	if s.in.Lines != nil {
		if v, ok := s.in.Lines.Method(fr.rec, name, s.lang); ok {
			line.Text = Text{Value: v, OK: true}
		}
	}
	s.out.Show = append(s.out.Show, line)
}

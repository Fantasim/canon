package live

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/render"
	"github.com/fantasim/canonlang/internal/views/shape"
	"github.com/fantasim/canonlang/internal/views/table"
	"github.com/fantasim/canonlang/internal/views/typedef"
)

// session is one Evaluate: what it reads the program with and the result it fills.
type session struct {
	ctx    context.Context
	in     Input
	lang   string
	index  *control.Index
	colls  *encode.Colls
	res    *control.Resolver
	defs   *typedef.Types
	memo   *memo            // every view expression evaluated, once
	at     *placed          // the value the shown renderer renders, with its magic names
	shown  *render.Renderer // renders heads in the value's place
	target *render.Renderer // renders a ref's target title, refs in it as keys (S8)
	keys   map[*types.Collection]map[value.Key]*value.Record
	out    *Result
	err    error // the context's, or a resolved type that did not encode
}

// newSession reads prog's views, collections and texts as the view model does (VIEWMODEL.md 12).
func newSession(ctx context.Context, in Input, lang string) *session {
	s := &session{ctx: ctx, in: in, lang: lang, keys: map[*types.Collection]map[value.Key]*value.Record{}}
	s.memo = &memo{eval: in.Eval, seen: map[memoKey]memoVal{}}
	s.at = &placed{memo: s.memo}
	s.index = control.NewIndex(in.Program, in.Studio)
	s.colls = encode.NewColls(in.Force)
	texts := encode.NewTexts(catalogues(in.I18N))
	assets := encode.NewAssets(in.Program, in.Layout)
	tables := table.New(s.index, texts)
	s.res = control.NewResolver(s.index, control.Env{
		Counts: s.colls.Counts, Table: tables.Complete, Singular: tables.Singular, Assets: assets,
	})
	tables.Bind(s.res)
	s.defs = typedef.New(ctx, typedef.Input{Program: in.Program, Index: s.index, Colls: s.colls, Texts: texts, Assets: assets}, "")
	base := render.Input{Program: in.Program, Index: s.index, Texts: texts, Colls: s.colls}
	shown, target := base, base
	shown.Eval, target.Eval = s.at, keyed{memo: s.memo}
	s.shown, s.target = render.New(ctx, shown), render.New(ctx, target)
	s.out = &Result{When: map[string]bool{}, Show: []ShowLine{}, Headings: map[string]Heading{}, Types: map[string]json.RawMessage{}}
	return s
}

// catalogues are the key catalogues of results, by package.
func catalogues(results map[string]*i18n.Result) map[string]*i18n.Catalogue {
	out := make(map[string]*i18n.Catalogue, len(results))
	//canon:unordered a map copied into a map
	for pkg, r := range results {
		if r != nil {
			out[pkg] = r.Catalogue
		}
	}
	return out
}

// frame is a record of the form and the record whose field holds it (TYPES.md 11.1).
type frame struct {
	rec     *value.Record
	magic   render.Magic                 // its magic names: the value at the path's only (VIEWMODEL.md 3.4)
	applied *types.AppliedRecord         // the type its field gives it, binding its parameters; nil for none
	params  map[*types.Param]value.Value // a type function's parameters, bound (TYPES.md 11.2)
	up      *frame
}

// form evaluates the view items of fr's record, then of each record or case its fields hold (V5),
// and heads the elements of every collection its fields hold (V6).
func (s *session) form(fr *frame, at *verify.Path) {
	if s.stopped() {
		return
	}
	s.when(fr, at)
	s.shows(fr, at)
	rec := fr.rec
	for i, f := range encode.FieldsOf(rec.T) {
		if i >= len(rec.Fields) || rec.Fields[i] == nil {
			continue
		}
		p := at.Field(f.Name)
		s.depend(fr, f, p)
		if inner, ok := rec.Fields[i].(*value.Record); ok {
			applied, _ := shape.StripOptional(f.Type).Base().(*types.AppliedRecord)
			s.form(&frame{rec: inner, applied: applied, up: fr}, p)
			continue
		}
		if isCollection(rec.Fields[i]) {
			s.collection(rec.Fields[i], p, s.res.Field(rec.T, f))
		}
	}
}

// stopped reports a cancelled context, kept as the session's error.
func (s *session) stopped() bool {
	if s.err == nil {
		s.err = s.ctx.Err()
	}
	return s.err != nil
}

// eval is x's value for self with m (VIEWMODEL.md 3.4); false when it fails (X7).
func (s *session) eval(x syntax.Expr, self value.Value, m render.Magic) (value.Value, bool) {
	return s.memo.Eval(s.ctx, x, self, m)
}

// rel is a path relative to the value at the request's path, without its leading `.` (V9).
func rel(p *verify.Path) string { return strings.TrimPrefix(p.String(), dot) }

// fieldOf is the field name of the record rec, nil for none (an input, a field it lacks).
func fieldOf(rec *value.Record, name string) value.Value {
	for i, f := range encode.FieldsOf(rec.T) {
		if f.Name == name && i < len(rec.Fields) {
			return rec.Fields[i]
		}
	}
	return nil
}

// idOf is the magic name `id` of a table or define-table entry: its key (VIEWMODEL.md 3.4); nil
// for another value.
func idOf(e *value.Record) value.Value {
	switch {
	case e.Ident == nil || e.Ident.Coll == nil || e.Ident.Coll.KeyedBy != nil:
		return nil
	case e.Ident.Key.IsInt:
		return &value.Int{V: e.Ident.Key.I, T: types.IntType}
	}
	return &value.Str{V: e.Ident.Key.S, T: types.StringType}
}

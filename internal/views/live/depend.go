package live

import (
	"encoding/json"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// depend records the type the dependent field f of fr's record resolves to, in the view model's
// encoding (API.md 11 `Types`, VIEWMODEL.md D6, J14, 13); left out when a driver is `none` or
// cannot be read (log-2026-09-29 M4 U9).
func (s *session) depend(fr *frame, f *types.Field, at *verify.Path) {
	t := shape.StripOptional(f.Type)
	if _, ok := t.Base().(*types.TypeAppType); !ok {
		return
	}
	t, ok := s.substitute(t, fr)
	if !ok {
		return
	}
	g := *f
	g.Type = t
	b, err := json.Marshal(s.defs.FieldExpr(fr.rec.T, &g))
	if err != nil {
		s.err = err
		return
	}
	s.out.Types[rel(at)] = b
}

// substitute is t with each application it holds replaced by the type it computes in fr, nested
// ones computed with their function's parameters bound, refinements kept (TYPES.md 11.2, 11.6).
func (s *session) substitute(t types.Type, fr *frame) (types.Type, bool) {
	switch x := t.(type) {
	case *types.TypeAppType:
		bt, inner, ok := s.branch(fr, x)
		if !ok {
			return nil, false
		}
		return s.substitute(bt, inner)
	case *types.Refined:
		of, ok := s.substitute(x.Of, fr)
		return &types.Refined{Of: of, Range: x.Range, Pattern: x.Pattern, Where: x.Where, Asset: x.Asset}, ok
	case *types.Alias:
		return s.substitute(x.Def, fr)
	case *types.OptionalType:
		elem, ok := s.substitute(x.Elem, fr)
		return &types.OptionalType{Elem: elem}, ok
	case *types.LitUnionType:
		of, ok := s.substitute(x.Of, fr)
		return &types.LitUnionType{Of: of, Literals: x.Literals}, ok
	case *types.ListType:
		elem, ok := s.substitute(x.Elem, fr)
		return &types.ListType{Elem: elem, KeyedBy: x.KeyedBy}, ok
	case *types.MapType:
		k, ok := s.substitute(x.Key, fr)
		v, vok := s.substitute(x.Value, fr)
		return &types.MapType{Key: k, Value: v}, ok && vok
	}
	return t, true
}

// branch is the type app computes in fr: the arm its scrutinee selects, or its function's body,
// and the frame binding the function's parameters (TYPES.md 11.2).
func (s *session) branch(fr *frame, app *types.TypeAppType) (types.Type, *frame, bool) {
	fn := app.Fn
	if len(fn.Params) != len(app.Args) {
		return nil, nil, false
	}
	inner := &frame{params: map[*types.Param]value.Value{}}
	for i, p := range fn.Params {
		if v, ok := s.arg(fr, app.Args[i]); ok {
			inner.params[p] = v
		}
	}
	if fn.Scrutinee == nil {
		return fn.Body, inner, fn.Body != nil
	}
	v, read := s.follow(inner.params[fn.Scrutinee.Param], fn.Scrutinee.Path)
	i, ok := value.ArmIndex(v)
	if !read || !ok || fn.Arm(i) == nil {
		return nil, nil, false
	}
	return fn.Arm(i).Result, inner, true
}

// arg is a's value in fr, followed down a's path (TYPES.md 11.1): an earlier field of fr's
// record, or a parameter bound in fr.
func (s *session) arg(fr *frame, a *types.Arg) (value.Value, bool) {
	switch a.Source {
	case types.ArgField:
		if len(a.Path) == 0 || fr.rec == nil {
			return nil, false
		}
		return s.follow(fieldOf(fr.rec, a.Path[0].Name), a.Path[1:])
	case types.ArgParam:
		v, ok := s.param(fr, a.Param)
		if !ok {
			return nil, false
		}
		return s.follow(v, a.Path)
	case types.ArgKey:
	}
	return nil, false
}

// param is p's value in fr: bound by a type function, by Input.Bound for fr's record, by the
// argument the type of fr's field reads in the record holding it, or by fr's map key (11.5).
func (s *session) param(fr *frame, p *types.Param) (value.Value, bool) {
	if v, ok := fr.params[p]; ok {
		return v, v != nil
	}
	if fr.rec != nil && s.in.Bound != nil {
		if v, ok := s.in.Bound(fr.rec)[p]; ok {
			return v, v != nil
		}
	}
	if fr.applied == nil {
		return nil, false
	}
	i := slices.Index(fr.applied.Rec.Params, p)
	if i < 0 || i >= len(fr.applied.Args) {
		return nil, false
	}
	a := fr.applied.Args[i]
	switch {
	case a.Source == types.ArgKey && fr.magic.Key != nil:
		return s.follow(fr.magic.Key, a.Path)
	case a.Source == types.ArgKey || fr.up == nil:
		return nil, false
	}
	return s.arg(fr.up, a)
}

// follow reads path down v, field by field, a ref read as its target entry in this build.
func (s *session) follow(v value.Value, path []*types.Field) (value.Value, bool) {
	for _, f := range path {
		rec := s.record(v)
		if rec == nil {
			return nil, false
		}
		v = fieldOf(rec, f.Name)
	}
	_, none := v.(*value.None)
	return v, v != nil && !none
}

// record is v as a record: itself, or the entry a ref names; nil for another value.
func (s *session) record(v value.Value) *value.Record {
	switch x := v.(type) {
	case *value.Record:
		return x
	case *value.Ref:
		if rt, ok := x.T.Base().(*types.RefType); ok {
			e, _ := s.shown.Target(rt.Target, x.Key)
			return e
		}
	}
	return nil
}

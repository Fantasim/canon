package live

import (
	"context"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/render"
)

// memoKey is one evaluation: an expression, the value shown and its magic names, by text (render
// builds a new `id` value on each call); text is enough since a magic name has one key type, else
// the checker leaves it undeclared (E2102, check.magicNames).
type memoKey struct {
	e     syntax.Expr
	self  value.Value
	magic magicText
}

// magicText are magic names by canonical text, each with whether it has a value.
type magicText struct {
	id, key, index          string
	hasID, hasKey, hasIndex bool
}

// textOf is m by canonical text.
func textOf(m render.Magic) magicText {
	var t magicText
	t.id, t.hasID = canon(m.ID)
	t.key, t.hasKey = canon(m.Key)
	t.index, t.hasIndex = canon(m.Index)
	return t
}

// canon is v's canonical text; false for no value.
func canon(v value.Value) (string, bool) {
	if v == nil {
		return "", false
	}
	return v.CanonText(), true
}

// memoVal is what an evaluation gave.
type memoVal struct {
	v  value.Value
	ok bool
}

// memo evaluates each expression once per value and magic names in a session, so that detecting
// Fallback reads what rendering evaluated and never spends a step (API.md V13; log-2026-09-29 M4 U9).
type memo struct {
	eval render.Evaluator
	seen map[memoKey]memoVal
}

func (c *memo) Eval(ctx context.Context, e syntax.Expr, self value.Value, m render.Magic) (value.Value, bool) {
	k := memoKey{e: e, self: self, magic: textOf(m)}
	if r, ok := c.seen[k]; ok {
		return r.v, r.ok
	}
	if c.eval == nil || e == nil {
		return nil, false
	}
	v, ok := c.eval.Eval(ctx, e, self, m)
	ok = ok && v != nil
	c.seen[k] = memoVal{v: v, ok: ok}
	return v, ok
}

// read is what rendering evaluated for e, self and m; false when it failed or was not evaluated.
func (c *memo) read(e syntax.Expr, self value.Value, m render.Magic) (value.Value, bool) {
	r, ok := c.seen[memoKey{e: e, self: self, magic: textOf(m)}]
	return r.v, ok && r.ok
}

// placed evaluates for a renderer: the magic names of the value it renders are given to that
// value's expressions only (VIEWMODEL.md 3.4); render itself gives `id` alone.
type placed struct {
	memo  *memo
	self  *value.Record
	magic render.Magic
}

func (p *placed) Eval(ctx context.Context, e syntax.Expr, self value.Value, m render.Magic) (value.Value, bool) {
	if r, ok := self.(*value.Record); ok && r == p.self {
		m = withMagic(m, p.magic)
	}
	return p.memo.Eval(ctx, e, self, m)
}

// withMagic is m with each magic name it lacks taken from more.
func withMagic(m, more render.Magic) render.Magic {
	if m.ID == nil {
		m.ID = more.ID
	}
	if m.Key == nil {
		m.Key = more.Key
	}
	if m.Index == nil {
		m.Index = more.Index
	}
	return m
}

// keyed evaluates for a ref's target title: a ref it reads renders as its key (S8, one level).
type keyed struct{ memo *memo }

func (k keyed) Eval(ctx context.Context, e syntax.Expr, self value.Value, m render.Magic) (value.Value, bool) {
	v, ok := k.memo.Eval(ctx, e, self, m)
	if r, isRef := v.(*value.Ref); ok && isRef {
		return &value.Str{V: r.Key.Text(), T: types.StringType}, true
	}
	return v, ok
}

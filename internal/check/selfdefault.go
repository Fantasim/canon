package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// selfMembers are E3010's variants for a built-in member read on a default's `self`: its case is known, its entry key unread (DECISIONS 465).
var selfMembers = map[string]func(source.Span) *diag.Builder{
	kindMember:    diag.E3010.AtOwnCase,
	idMember:      diag.E3010.AtEntryKey,
	retiredMember: diag.E3010.AtEntryKey,
}

// receiving is env for typing x, a selection's or method call's receiver: a default's `self` there is judged by what it selects (TYPES.md §15).
func (env *env) receiving(x syntax.Expr) *env {
	return env.selfIn(x, modeSelfRecv)
}

// caseTested is env for typing x, the operand of `is`: a case default's `self` tests its own case; a record's is typed, so `is` reports E3605 (TYPES.md §8.3, §15).
func (env *env) caseTested(x syntax.Expr) *env {
	if env.rec != nil {
		if _, isCase := env.rec.self.(*types.CaseType); isCase {
			return env.selfIn(x, modeSelfCase)
		}
	}
	return env.selfIn(x, modeSelfRecv)
}

// selfIn is env with flag m when x is a field default's `self`, parenthesized or not.
func (env *env) selfIn(x syntax.Expr, m mode) *env {
	if _, isSelf := syntax.Unparen(x).(*syntax.SelfExpr); !isSelf || env.fields < 0 {
		return env
	}
	e := env.with()
	e.mode |= m
	return e
}

// bareSelf is E3010 at a default's bare `self`: it reads the field itself and later ones, or, under `is`, its own case (DECISIONS 465).
func bareSelf(env *env, span source.Span) *diag.Builder {
	if env.mode&modeSelfCase != 0 {
		return diag.E3010.AtOwnCase(span)
	}
	return diag.E3010.AtNotEarlier(span)
}

// selfMemberAllowed is E3010 at `self.name` in a field default unless name is an earlier field, as its bare name (TYPES.md §3.3, §15).
func (c *checker) selfMemberAllowed(env *env, s *syntax.SelectorExpr) bool {
	if _, isSelf := syntax.Unparen(s.X).(*syntax.SelfExpr); !isSelf || env.fields < 0 {
		return true
	}
	sel := c.info.Selections[s]
	o, isObj := sel.Obj.(*object)
	if isObj && o != nil && sel.Kind == SelField && defaultName(env, o) {
		return true
	}
	at := diag.E3010.AtNotEarlier
	if f, builtin := selfMembers[s.Name.Name]; builtin && sel.Kind == SelBuiltinMember {
		at = f
	}
	c.report(env, at(env.span(s)))
	return false
}

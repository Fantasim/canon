package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// recordLit is a record or case literal (TYPES.md §5.2).
func (c *checker) recordLit(env *env, e *syntax.BraceLit, t types.Type) types.Type {
	c.info.Literals[e] = LitRecord
	fields := recordFields(c, t)
	given := map[string]bool{}
	spread := false
	for i, it := range e.Items {
		switch it := it.(type) {
		case *syntax.SpreadItem:
			c.spreadItem(env, it, t, i)
			spread = true
		case *syntax.FieldItem:
			c.fieldItem(env, it, t, fields, given)
		default:
			c.report(env, diag.E3320.At(env.span(it), itemKind(it), diag.KindRecord))
			c.itemsAlone(env, []syntax.BraceItem{it})
		}
	}
	if !spread {
		c.requiredFields(env, e, t, fields, given)
	}
	return t
}

// recordFields are the fields of a record or case type.
func recordFields(c *checker, t types.Type) []*types.Field {
	switch x := t.(type) {
	case *types.RecordType:
		c.completeRecord(x)
		return x.Fields
	case *types.AppliedRecord:
		c.completeRecord(x.Rec)
		return x.Rec.Fields
	case *types.CaseType:
		c.completeVariant(x.Variant)
		return x.Fields
	}
	return nil
}

// spreadItem is `...e`: first and once (E3323), of the literal's own type, a case of the same
// case (E3303); a spread anywhere gives the fields, so none is reported missing.
func (c *checker) spreadItem(env *env, it *syntax.SpreadItem, t types.Type, i int) {
	st := c.synth(env, it.X)
	if i != 0 {
		c.report(env, diag.E3323.At(env.span(it)))
		return
	}
	if st.Kind() != types.Error && !types.Identical(st, t) {
		c.report(env, diag.E3303.At(env.span(it.X), st, t))
	}
}

// fieldItem is `name: value` in a record literal (TYPES.md §5.2, §11.4).
func (c *checker) fieldItem(env *env, it *syntax.FieldItem, t types.Type, fields []*types.Field, given map[string]bool) {
	f := fieldNamed(fields, it.Name.Name)
	if f == nil {
		c.report(env, diag.E3301.At(env.span(it.Name), t, it.Name.Name))
		c.synth(env, it.Value)
		return
	}
	fo := c.fieldObjects[f]
	c.info.NameUses[it.Name] = fo
	if given[f.Name] {
		c.report(env, diag.E3321.At(env.span(it.Name), f.Name))
	}
	given[f.Name] = true
	if f.Input != nil {
		c.report(env, diag.E3312.At(env.span(it.Name), f.Name))
		c.synth(env, it.Value)
		return
	}
	c.deprecatedUse(env, it.Name, fo)
	c.expr(env.storing(fo), it.Value, staticView(f.Type))
}

// requiredFields is E3302 for each field with no default, not optional and not an input,
// that the literal leaves out; a field its record contains itself through is E3022's.
func (c *checker) requiredFields(env *env, e *syntax.BraceLit, t types.Type, fields []*types.Field, given map[string]bool) {
	owner := requiredRecord(t)
	for _, f := range fields {
		if given[f.Name] || f.Default != nil || f.Input != nil || f.Type.Base().Kind() == types.Optional {
			continue
		}
		if selfContaining(owner, f) {
			continue
		}
		c.report(env, diag.E3302.At(env.tokSpan(e.First()), t, f.Name))
	}
}

// tokSpan is the span of one token of env's file.
func (env *env) tokSpan(t syntax.Tok) source.Span {
	tok := env.file.Tokens[t]
	return source.Span{File: env.file.Src.ID, Start: tok.Start, End: tok.End}
}

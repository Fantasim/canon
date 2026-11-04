package edit

import (
	"context"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// literalRoot is the part of an edit-layer line naming n steps that an edit upto steps down res's
// path rewrites: the shallowest node on the path, at most upto steps down, whose subtree holds no
// load, expression or spread; a sibling of one stays as written (DECISIONS 462). False for none.
func (s *Snapshot) literalRoot(line *syntax.Amendment, n int, res resolution, upto int) (syntax.Node, int, bool) {
	var node syntax.Node = syntax.Unparen(line.Value)
	for d := n; ; d++ {
		if s.opaqueIn(node) == 0 {
			return node, d, true
		}
		next, ok := s.childOn(node, res, d)
		if d >= upto || !ok {
			return nil, 0, false
		}
		node = next
	}
}

// nodeAt is the node of line, naming n steps, at depth d of res's path, false where it has none.
func (s *Snapshot) nodeAt(line *syntax.Amendment, n int, res resolution, d int) (syntax.Node, bool) {
	var node syntax.Node = syntax.Unparen(line.Value)
	for i := n; i < d; i++ {
		next, ok := s.childOn(node, res, i)
		if !ok {
			return nil, false
		}
		node = next
	}
	return node, true
}

// absentField is the record literal of the line, at depth d of res's path, leaving out the field step
// d names, which an edit upto steps down inserts as a literal beside its loads or expressions (API.md
// W7, W8; DECISIONS 462); false for no such field, or one whose default no literal states.
func (s *Snapshot) absentField(line *syntax.Amendment, n int, res resolution, upto int) (*syntax.BraceLit, int, bool) {
	var node syntax.Node = syntax.Unparen(line.Value)
	for d := n; d < upto; d++ {
		if next, ok := s.childOn(node, res, d); ok {
			if s.nodeForm(next) != shape.FormLiteral {
				return nil, 0, false
			}
			node = next
			continue
		}
		lit, isLit := node.(*syntax.BraceLit)
		st := res.Steps[d]
		if !isLit || hasSpread(lit) || st.Seg.Kind != SegField || fieldItem(lit, st.Seg.Name) != nil {
			return nil, 0, false
		}
		return lit, d, s.literalDefault(typeAtDepth(res, d), st.Seg.Name)
	}
	return nil, 0, false
}

// literalDefault reports field name of record type rt left out to a default naming no field of the
// record, no `self`, no record parameter and no range (TYPES.md 15): one an edit computes and prints
// without the record's other values or the arguments it is applied to.
func (s *Snapshot) literalDefault(rt types.Type, name string) bool {
	fields := fieldsOf(present(rt))
	i := fieldIndex(fields, name)
	if i < 0 || !hasDefault(fields[i]) {
		return false
	}
	if fields[i].Default == nil {
		return true
	}
	params := recordParams(present(rt))
	free := true
	syntax.Inspect(fields[i].Default, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.SelfExpr, *syntax.RangeExpr: // a range no literal prints (M7)
			free = false
		case *syntax.IdentExpr:
			free = free && fieldIndex(fields, x.Name) < 0 && !slices.ContainsFunc(params, func(p *types.Param) bool { return p.Name == x.Name })
		}
		return free
	})
	return free
}

// recordParams are the parameters of record type t, applied or not.
func recordParams(t types.Type) []*types.Param {
	switch b := t.Base().(type) {
	case *types.RecordType:
		return b.Params
	case *types.AppliedRecord:
		return b.Rec.Params
	}
	return nil
}

// partSource reports the part of at's line an edit upto steps down res's path rewrites (literalRoot)
// a Source of its declared type, the line itself as lineSource reads it, or a field it leaves out
// (absentField) (DECISIONS 462).
func (s *Snapshot) partSource(ctx context.Context, at staticSite, res resolution, upto int) bool {
	node, d, ok := s.literalRoot(at.line, at.n, res, max(upto, at.n))
	switch {
	case !ok:
		_, _, inserts := s.absentField(at.line, at.n, res, max(upto, at.n))
		return inserts
	case d == at.n:
		return s.lineSource(ctx, at.f, res.root.at(), at.line)
	}
	p := res.root.at()
	for _, st := range res.Steps[:d] {
		p.Segs = append(p.Segs, st.Seg)
	}
	sp := at.f.Span(node)
	return s.sourceLine(ctx, p.String(), string(at.f.Src.Content[sp.Start:sp.End]))
}

// printable reports v a value a literal states: no range nor pair in it, which canonPrinter cannot write.
func printable(v value.Value) bool {
	switch x := v.(type) {
	case *value.Range, *value.Pair:
		return false
	case *value.Record:
		return allPrintable(x.Fields)
	case *value.List:
		return allPrintable(x.Elems)
	case *value.Map:
		return allPrintable(x.Keys) && allPrintable(x.Vals)
	case *value.Table:
		for _, e := range x.Entries {
			if !printable(e) {
				return false
			}
		}
	}
	return true
}

// allPrintable reports every value of vs printable.
func allPrintable(vs []value.Value) bool {
	for _, v := range vs {
		if !printable(v) {
			return false
		}
	}
	return true
}

// typeAtDepth is the declared type of the value res's first d steps reach.
func typeAtDepth(res resolution, d int) types.Type {
	if d == 0 {
		return res.root.obj.Type()
	}
	st := res.Steps[d-1]
	if rec, ok := res.parent(d - 1).(*value.Record); ok {
		fields := fieldsOf(rec.T)
		if i := fieldIndex(fields, st.Seg.Name); i >= 0 {
			return fields[i].Type
		}
	}
	if _, vt, ok := mapTypes(st.Container); ok {
		return vt
	}
	return elemType(st.Container)
}

// holdsDependent reports t, or a type inside it but behind no ref, one verification computes from a
// value: a type function's application or static union, a map keyed by a collection, a record
// applied to arguments (TYPES.md 11).
func holdsDependent(t types.Type) bool { return dependentIn(t, map[types.Type]bool{}) }

// dependentIn is holdsDependent, seen the records and variants already walked.
func dependentIn(t types.Type, seen map[types.Type]bool) bool {
	if t == nil {
		return false
	}
	switch b := t.Base().(type) {
	case *types.TypeAppType, *types.DepUnionType, *types.DepMapType:
		return true
	case *types.OptionalType:
		return dependentIn(b.Elem, seen)
	case *types.ListType:
		return dependentIn(b.Elem, seen)
	case *types.TableType:
		return dependentIn(b.Elem, seen)
	case *types.MapType:
		return dependentIn(b.Key, seen) || dependentIn(b.Value, seen)
	case *types.VariantType:
		return !seen[b] && casesDependent(b, seen)
	case *types.AppliedRecord:
		if len(b.Args) > 0 {
			return true
		}
		return recordDependent(b, seen)
	case *types.RecordType, *types.CaseType:
		return recordDependent(b, seen)
	}
	return false
}

// recordDependent reports a field of record or case t holding a dependent type, t walked once.
func recordDependent(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	return fieldsDependent(fieldsOf(t), seen)
}

// casesDependent reports a field of one of v's cases holding a dependent type.
func casesDependent(v *types.VariantType, seen map[types.Type]bool) bool {
	seen[v] = true
	for _, c := range v.Cases {
		if dependentIn(c, seen) {
			return true
		}
	}
	return false
}

// fieldsDependent reports one of fields holding a dependent type.
func fieldsDependent(fields []*types.Field, seen map[types.Type]bool) bool {
	for _, f := range fields {
		if dependentIn(f.Type, seen) {
			return true
		}
	}
	return false
}

// lineKey is a line's text as verdicts read it: its tokens with no comment, each gap between two
// a space or a newline, then flat where it can be; trivia never changes a verdict (DECISIONS 462).
func lineKey(text string) string {
	e, f := parseLiteral(text)
	if e == nil {
		return text
	}
	src := f.Src.Content
	var b strings.Builder
	prev := f.Tokens[e.First()].Start
	for _, tok := range f.Tokens[e.First() : e.Last()+1] {
		switch gap := string(src[prev:tok.Start]); {
		case strings.Contains(gap, newline):
			b.WriteString(newline)
		case gap != "":
			b.WriteString(space)
		}
		b.Write(src[tok.Start:tok.End])
		prev = tok.End
	}
	return flatText(b.String())
}

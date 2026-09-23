package syntax_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
)

// The groups of IMPLEMENTATION-PLAN §4.1 and the item interfaces, checked by the compiler.
var (
	_ syntax.Decl         = (*syntax.ConstDecl)(nil)
	_ syntax.Decl         = (*syntax.LetDecl)(nil)
	_ syntax.Decl         = (*syntax.TypeDecl)(nil)
	_ syntax.Decl         = (*syntax.FnDecl)(nil)
	_ syntax.Decl         = (*syntax.EntryDecl)(nil)
	_ syntax.Decl         = (*syntax.CheckDecl)(nil)
	_ syntax.Decl         = (*syntax.ViewDecl)(nil)
	_ syntax.Decl         = (*syntax.WidgetDecl)(nil)
	_ syntax.Decl         = (*syntax.TestDecl)(nil)
	_ syntax.Decl         = (*syntax.EmitDecl)(nil)
	_ syntax.Decl         = (*syntax.RecordDecl)(nil)
	_ syntax.Decl         = (*syntax.EnumDecl)(nil)
	_ syntax.Decl         = (*syntax.VariantDecl)(nil)
	_ syntax.RecordItem   = (*syntax.FieldDecl)(nil)
	_ syntax.RecordItem   = (*syntax.FnDecl)(nil)
	_ syntax.RecordItem   = (*syntax.CheckDecl)(nil)
	_ syntax.VariantItem  = (*syntax.VariantCase)(nil)
	_ syntax.VariantItem  = (*syntax.FnDecl)(nil)
	_ syntax.VariantItem  = (*syntax.CheckDecl)(nil)
	_ syntax.GroupMember  = (*syntax.ViewShow)(nil)
	_ syntax.GroupMember  = (*syntax.ViewField)(nil)
	_ syntax.StrLit       = (*syntax.StringLit)(nil)
	_ syntax.StrLit       = (*syntax.RawStringLit)(nil)
	_ syntax.NameLit      = (*syntax.Ident)(nil)
	_ syntax.NameLit      = (*syntax.StringLit)(nil)
	_ syntax.NameLit      = (*syntax.RawStringLit)(nil)
	_ syntax.EntryKey     = (*syntax.Ident)(nil)
	_ syntax.EntryKey     = (*syntax.IntLit)(nil)
	_ syntax.AnnValue     = (*syntax.StringLit)(nil)
	_ syntax.AnnValue     = (*syntax.RawStringLit)(nil)
	_ syntax.AnnValue     = (*syntax.IntLit)(nil)
	_ syntax.AnnValue     = (*syntax.FloatLit)(nil)
	_ syntax.AnnValue     = (*syntax.DurationLit)(nil)
	_ syntax.AnnValue     = (*syntax.BoolLit)(nil)
	_ syntax.AnnValue     = (*syntax.QualifiedName)(nil)
	_ syntax.AnnValue     = (*syntax.AnnotationList)(nil)
	_ syntax.AnnValue     = (*syntax.BraceLit)(nil)
	_ syntax.ProjectValue = (*syntax.StringLit)(nil)
	_ syntax.ProjectValue = (*syntax.RawStringLit)(nil)
	_ syntax.ProjectValue = (*syntax.IntLit)(nil)
	_ syntax.ProjectValue = (*syntax.QualifiedName)(nil)
	_ syntax.ProjectValue = (*syntax.ProjectList)(nil)
	_ syntax.ProjectValue = (*syntax.ProjectMap)(nil)
)

// Every node type has its own kind, named after the type, and every kind has a node type.
func TestNodeKinds(t *testing.T) {
	seen := map[syntax.NodeKind]string{}
	for _, s := range samples() {
		k := s.node.Kind()
		name := reflect.TypeOf(s.node).Elem().Name()
		if prev, dup := seen[k]; dup {
			t.Errorf("%s and %s share kind %d", prev, name, k)
		}
		seen[k] = name
		if k.String() != name {
			t.Errorf("%s.Kind().String() = %q", name, k.String())
		}
	}
	for k := syntax.KindInvalid + 1; k < syntax.NodeKindCount; k++ {
		if _, ok := seen[k]; !ok {
			t.Errorf("kind %s has no sample node", k)
		}
	}
	if got := syntax.NodeKindCount.String(); got != syntax.KindInvalid.String() {
		t.Errorf("out-of-range kind prints %q", got)
	}
}

// The walker yields every child that is set, in source order (GRAMMAR.md §10: one lossless tree).
func TestChildrenInSourceOrder(t *testing.T) {
	for _, s := range samples() {
		var got []syntax.Node
		for c := range syntax.Children(s.node) {
			got = append(got, c)
		}
		if len(got) != s.kids {
			t.Errorf("%s: %d children, want %d", s.node.Kind(), len(got), s.kids)
		}
		if !slices.IsSortedFunc(got, func(a, b syntax.Node) int { return int(a.First() - b.First()) }) {
			t.Errorf("%s: children out of source order", s.node.Kind())
		}
	}
}

func TestChildrenStopsEarly(t *testing.T) {
	g := &gen{}
	call := &syntax.CallExpr{Fun: g.x(), Args: []*syntax.Arg{g.arg(), g.arg()}}
	n := 0
	for range syntax.Children(call) {
		n++
		break
	}
	if n != 1 {
		t.Errorf("iterated %d children after break", n)
	}
}

// recorder logs the kinds Walk visits, "end" for the nil after a node's children.
type recorder struct {
	log  *[]string
	skip syntax.NodeKind
}

func (r recorder) Visit(n syntax.Node) syntax.Visitor {
	if n == nil {
		*r.log = append(*r.log, "end")
		return nil
	}
	*r.log = append(*r.log, n.Kind().String())
	if n.Kind() == r.skip {
		return nil
	}
	return r
}

func TestWalk(t *testing.T) {
	g := &gen{}
	sum := &syntax.BinaryExpr{X: g.x(), Op: syntax.TokPlus, Y: &syntax.ParenExpr{X: g.int()}}
	var log []string
	syntax.Walk(recorder{log: &log}, sum)
	want := []string{"BinaryExpr", "IdentExpr", "end", "ParenExpr", "IntLit", "end", "end", "end"}
	if !slices.Equal(log, want) {
		t.Errorf("Walk = %v, want %v", log, want)
	}
	log = nil
	syntax.Walk(recorder{log: &log, skip: syntax.KindParenExpr}, sum)
	want = []string{"BinaryExpr", "IdentExpr", "end", "ParenExpr", "end"}
	if !slices.Equal(log, want) {
		t.Errorf("Walk skipping ParenExpr = %v, want %v", log, want)
	}
}

// Inspect skips the children of a node for which f returns false, and calls f(nil) after the
// children of the others.
func TestInspect(t *testing.T) {
	g := &gen{}
	spread := &syntax.SpreadItem{X: g.x()}
	lit := &syntax.TypedLit{Type: g.qn(), Lit: &syntax.BraceLit{Items: []syntax.BraceItem{spread}}}
	var got []string
	syntax.Inspect(lit, func(n syntax.Node) bool {
		if n == nil {
			got = append(got, "end")
			return false
		}
		got = append(got, n.Kind().String())
		return n.Kind() != syntax.KindSpreadItem
	})
	want := []string{"TypedLit", "QualifiedName", "end", "BraceLit", "SpreadItem", "end", "end"}
	if !slices.Equal(got, want) {
		t.Errorf("Inspect = %v, want %v", got, want)
	}
}

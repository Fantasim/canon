package load_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// dependentFixture is Task.filterParam's shape (resource/heistia/heistia.canon): a dependent
// type (TypeAppType) nested inside a list field, needing Host.Deref to select its branch.
func dependentFixture() *types.RecordType {
	dep := &types.TypeAppType{Fn: &types.TypeFunc{Body: types.IntType}}
	filterParam := &types.Field{Name: "filterParam", Type: dep, Wire: "filterParam", WirePath: []string{"filterParam"}}
	task := &types.RecordType{Pkg: "p", Name: "Task", Fields: []*types.Field{filterParam}}
	tasks := &types.Field{Name: "tasks", Type: &types.ListType{Elem: task}, Wire: "tasks", WirePath: []string{"tasks"}}
	return &types.RecordType{Pkg: "p", Name: "Config", Fields: []*types.Field{tasks}}
}

func bareExpr(path string) *syntax.LoadExpr {
	return &syntax.LoadExpr{Args: []*syntax.Arg{{Value: &syntax.StringLit{Parts: []syntax.StringPart{{Text: path}}}}}}
}

func csvExpr(path string) *syntax.LoadExpr {
	return &syntax.LoadExpr{Method: &syntax.Ident{Name: "csv"}, Args: []*syntax.Arg{{Value: &syntax.StringLit{Parts: []syntax.StringPart{{Text: path}}}}}}
}

// DECISIONS 173: bare load and load.csv refuse a dependent type loudly (ErrUnsupported) instead
// of silently poisoning the value (the resource.heistia gap), unlike a plain ref or variant.
func TestLoadDependentRefused(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"data/a.json": `{"tasks": []}`})
	_, _, err := l.Load(context.Background(), req, bareExpr("data/a.json"), dependentFixture())
	var ue *load.UnsupportedError
	if !errors.Is(err, load.ErrUnsupported) || !errors.As(err, &ue) || ue.Cause == "" {
		t.Errorf("bare load: err = %v, want ErrUnsupported with a cause", err)
	}

	l2, req2 := loaderFor(t, map[string]string{"data/a.csv": "filterParam\n1\n"})
	csvType := &types.ListType{Elem: dependentFixture()}
	if _, _, err := l2.Load(context.Background(), req2, csvExpr("data/a.csv"), csvType); !errors.Is(err, load.ErrUnsupported) {
		t.Errorf("load.csv: err = %v, want ErrUnsupported", err)
	}
}

// A ref, a variant or a map field does not need a host to decode, so they stay ungated
// (DECISIONS 173: only dependent-type branch selection needs Host.Deref).
func TestLoadRefStaysUngated(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"data/a.json": `{"id": "x"}`})
	ref := &types.RecordType{Pkg: "p", Name: "R", Fields: []*types.Field{
		{Name: "id", Type: &types.RefType{}, Wire: "id", WirePath: []string{"id"}},
	}}
	_, _, err := l.Load(context.Background(), req, bareExpr("data/a.json"), ref)
	if errors.Is(err, load.ErrUnsupported) {
		t.Errorf("a plain ref field: err = %v, want no refusal", err)
	}
}

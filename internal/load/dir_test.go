package load_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// itemType is a small record keyed by "id", used by the list and table cases below.
func itemType() *types.RecordType {
	id := &types.Field{Name: "id", Type: types.StringType, Wire: "id", WirePath: []string{"id"}}
	name := &types.Field{Name: "name", Type: types.StringType, Wire: "name", WirePath: []string{"name"}, Index: 1}
	id.Index = 0
	return &types.RecordType{Pkg: "p", Name: "Item", Fields: []*types.Field{id, name}}
}

// loaderFor builds a Loader and Request over an archive's files, none of them named "pattern".
func loaderFor(t *testing.T, files map[string]string) (*load.Loader, load.Request) {
	t.Helper()
	var a txtar.Archive
	for name, data := range files {
		a.Files = append(a.Files, txtar.File{Name: name, Data: []byte(data)})
	}
	set := &source.FileSet{}
	bag := diag.NewBag(set, "p")
	layout, ok := project.NewLayout(&project.Project{}, projectDir, nil, bag)
	if !ok {
		t.Fatal("layout")
	}
	l := &load.Loader{FS: newMemFS(&a), Layout: layout, Set: set}
	return l, load.Request{Pkg: "p", Bag: bag}
}

// WIRE.md §6.5: `load.dir` decodes each matched file into a table keyed by its stem, in path order.
func TestLoadDirTable(t *testing.T) {
	l, req := loaderFor(t, map[string]string{
		"data/a.json": `{"id": "a", "name": "A"}`,
		"data/b.json": `{"id": "b", "name": "B"}`,
	})
	v, ok, err := l.Load(context.Background(), req, dirExpr("data/*.json"), &types.TableType{Elem: itemType()})
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	tbl := v.(*value.Table)
	if len(tbl.Entries) != 2 || tbl.Entries[0].Ident.Key.S != "a" || tbl.Entries[1].Ident.Key.S != "b" {
		t.Errorf("table = %s", v.CanonText())
	}
}

// WIRE.md §6.5: `[T] keyed by f`'s elements keep the file order; their identity comes from f.
func TestLoadDirKeyedList(t *testing.T) {
	l, req := loaderFor(t, map[string]string{
		"data/b.json": `{"id": "b", "name": "B"}`,
		"data/a.json": `{"id": "a", "name": "A"}`,
	})
	item := itemType()
	keyed := &types.ListType{Elem: item, KeyedBy: item.Fields[0]}
	v, ok, err := l.Load(context.Background(), req, dirExpr("data/*.json"), keyed)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	list := v.(*value.List)
	if len(list.Elems) != 2 || list.Elems[0].(*value.Record).Ident.Key.S != "a" || list.Elems[1].(*value.Record).Ident.Key.S != "b" {
		t.Errorf("list = %s", v.CanonText())
	}
}

// M2: every load form but a plain `load.dir` is ErrUnsupported, its cause set (DECISIONS 196).
func TestLoadUnsupportedForms(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"data/a.json": `{"id": "a", "name": "A"}`})
	t.Run("plain load", func(t *testing.T) {
		e := &syntax.LoadExpr{Args: []*syntax.Arg{{Value: &syntax.StringLit{Parts: []syntax.StringPart{{Text: "data/a.json"}}}}}}
		_, _, err := l.Load(context.Background(), req, e, &types.TableType{Elem: itemType()})
		var ue *load.UnsupportedError
		if !errors.Is(err, load.ErrUnsupported) || !errors.As(err, &ue) || ue.Cause == "" {
			t.Errorf("err = %v, want ErrUnsupported with a cause", err)
		}
	})
	t.Run("a named option", func(t *testing.T) {
		e := dirExpr("data/*.json")
		e.Args[0].Name = &syntax.Ident{Name: "at"}
		if _, _, err := l.Load(context.Background(), req, e, &types.TableType{Elem: itemType()}); !errors.Is(err, load.ErrUnsupported) {
			t.Errorf("err = %v, want ErrUnsupported", err)
		}
	})
	t.Run("a recognized but unsupported format", func(t *testing.T) {
		for _, name := range []string{"data/a.txt", "data/a.csv"} {
			l, req := loaderFor(t, map[string]string{name: "A"})
			if _, _, err := l.Load(context.Background(), req, dirExpr("data/*"), &types.TableType{Elem: itemType()}); !errors.Is(err, load.ErrUnsupported) {
				t.Errorf("%s: err = %v, want ErrUnsupported", name, err)
			}
		}
	})
	t.Run("a refused format beside an unknown one reports nothing", func(t *testing.T) {
		// Review round 2 R2-5: the refusal fails the build (ErrLoad), so no E7007 may precede it.
		l, req := loaderFor(t, map[string]string{"data/a.h": "A", "data/b.csv": "B"})
		if _, _, err := l.Load(context.Background(), req, dirExpr("data/*"), &types.TableType{Elem: itemType()}); !errors.Is(err, load.ErrUnsupported) {
			t.Errorf("err = %v, want ErrUnsupported", err)
		}
		if f := req.Bag.Findings(); len(f) != 0 {
			t.Errorf("findings before the refusal: %+v", f)
		}
	})
	t.Run("an unsupported element type", func(t *testing.T) {
		ref := &types.RecordType{Pkg: "p", Name: "Item", Fields: []*types.Field{{Name: "r", Type: &types.RefType{}}}}
		if _, _, err := l.Load(context.Background(), req, dirExpr("data/*.json"), &types.TableType{Elem: ref}); !errors.Is(err, load.ErrUnsupported) {
			t.Errorf("err = %v, want ErrUnsupported", err)
		}
	})
}
